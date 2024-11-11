package systems

import (
	"fmt"

	"github.com/spaghettifunk/anima/engine/core"
	"github.com/spaghettifunk/anima/engine/renderer/metadata"
	"github.com/spaghettifunk/anima/engine/renderer/views"
)

/** @brief The configuration for the render view system. */
type RenderViewSystemConfig struct {
	/** @brief The maximum number of views that can be registered with the system. */
	MaxViewCount uint16
}

type RenderViewSystem struct {
	Lookup          map[string]uint16
	MaxViewCount    uint32
	RegisteredViews []*metadata.RenderView
	// subsystems
	renderer     *RendererSystem
	shaderSystem *ShaderSystem
	cameraSystem *CameraSystem
}

func NewRenderViewSystem(config RenderViewSystemConfig, r *RendererSystem, shaderSystem *ShaderSystem, cs *CameraSystem) (*RenderViewSystem, error) {
	if config.MaxViewCount == 0 {
		core.LogError("render_view_system_initialize - config.MaxViewCount must be > 0.")
		return nil, nil
	}

	rvs := &RenderViewSystem{
		MaxViewCount:    uint32(config.MaxViewCount),
		Lookup:          make(map[string]uint16, config.MaxViewCount),
		RegisteredViews: make([]*metadata.RenderView, config.MaxViewCount),
		renderer:        r,
		cameraSystem:    cs,
		shaderSystem:    shaderSystem,
	}
	// Fill the array with invalid entries.
	for i := uint32(0); i < rvs.MaxViewCount; i++ {
		rvs.RegisteredViews[i] = &metadata.RenderView{
			ID: metadata.InvalidIDUint16,
		}
	}
	return rvs, nil
}

func (rvs *RenderViewSystem) Shutdown() error {
	rvs.Lookup = nil
	rvs.RegisteredViews = nil
	return nil
}

func (rvs *RenderViewSystem) Create(config *metadata.RenderViewConfig) error {
	if config == nil {
		err := fmt.Errorf("render_view_system_create requires a pointer to a valid config")
		return err
	}

	if config.Name == "" {
		err := fmt.Errorf("render_view_system_create: name is required")
		return err
	}

	if config.PassCount < 1 {
		err := fmt.Errorf("render_view_system_create - Config must have at least one renderpass")
		return err
	}

	// Make sure there is not already an entry with this name already registered.
	id, ok := rvs.Lookup[config.Name]
	if ok && id != metadata.InvalidIDUint16 {
		err := fmt.Errorf("render_view_system_create - A view named '%s' already exists. A new one will not be created", config.Name)
		return err
	}

	// Find a new id.
	for i := uint32(0); i < rvs.MaxViewCount; i++ {
		if rvs.RegisteredViews[i].ID == metadata.InvalidIDUint16 {
			id = uint16(i)
			break
		}
	}

	// Make sure a valid entry was found.
	if id == metadata.InvalidIDUint16 {
		err := fmt.Errorf("render_view_system_create - No available space for a new view. Change system config to account for more")
		return err
	}

	view := rvs.RegisteredViews[id]
	view.ID = id
	view.RenderViewType = config.RenderViewType

	// TODO: Leaking the name, create a destroy method and kill this.
	view.Name = config.Name
	view.CustomShaderName = config.CustomShaderName
	view.RenderpassCount = config.PassCount
	view.Passes = make([]*metadata.RenderPass, view.RenderpassCount)

	for i := uint8(0); i < view.RenderpassCount; i++ {
		view.Passes[i] = rvs.renderer.RenderPassGet(config.Passes[i].Name)
		if view.Passes[i] == nil {
			err := fmt.Errorf("render_view_system_create - renderpass not found: '%s'", config.Passes[i].Name)
			return err
		}
	}

	// TODO: Assign these function pointers to known functions based on the view type.
	// TODO: Factory pattern (with register, etc. for each type)?

	uniforms := map[string]uint16{}
	if config.RenderViewType == metadata.RENDERER_VIEW_KNOWN_TYPE_WORLD {
		view.View = &views.RenderViewWorld{}
	} else if config.RenderViewType == metadata.RENDERER_VIEW_KNOWN_TYPE_UI {
		view.View = &views.RenderViewUI{}
	} else if config.RenderViewType == metadata.RENDERER_VIEW_KNOWN_TYPE_SKYBOX {
		shader, err := rvs.shaderSystem.GetShader("Shader.Builtin.Skybox")
		if err != nil {
			return err
		}
		c := rvs.cameraSystem.GetDefault()
		view.View = views.NewRenderViewSkybox(shader, c)

		uniforms["projection"] = rvs.shaderSystem.GetUniformIndex(shader, "projection")
		uniforms["view"] = rvs.shaderSystem.GetUniformIndex(shader, "view")
		uniforms["cube_texture"] = rvs.shaderSystem.GetUniformIndex(shader, "cube_texture")
	}
	// save current configuration
	view.ViewConfig = config

	// Call the on create
	if !view.View.OnCreateRenderView(uniforms) {
		err := fmt.Errorf("failed to create view")
		rvs.RegisteredViews[id] = nil
		return err
	}

	// Update the hashtable entry.
	rvs.Lookup[config.Name] = id

	return nil
}

/**
 * @brief Called when the owner of this view (i.e. the window) is resized.
 *
 * @param width The new width in pixels.
 * @param width The new height in pixels.
 */
func (rvs *RenderViewSystem) OnWindowResize(width, height uint32) {
	// Send to all views
	for i := uint32(0); i < rvs.MaxViewCount; i++ {
		if rvs.RegisteredViews[i].ID != metadata.InvalidIDUint16 {
			if width != uint32(rvs.RegisteredViews[i].Width) || height != uint32(rvs.RegisteredViews[i].Height) {
				rvs.RegisteredViews[i].Width = uint16(width)
				rvs.RegisteredViews[i].Height = uint16(height)

				rvs.RegisteredViews[i].View.OnResizeRenderView(width, height)

				for i := 0; i < int(rvs.RegisteredViews[i].RenderpassCount); i++ {
					rvs.RegisteredViews[i].Passes[i].RenderArea.X = 0
					rvs.RegisteredViews[i].Passes[i].RenderArea.Y = 0
					rvs.RegisteredViews[i].Passes[i].RenderArea.Z = float32(width)
					rvs.RegisteredViews[i].Passes[i].RenderArea.W = float32(height)
				}
			}
		}
	}
}

/**
 * @brief Obtains a pointer to a view with the given name.
 *
 * @param name The name of the view.
 * @return A pointer to a view if found; otherwise 0.
 */
func (rvs *RenderViewSystem) Get(name string) *metadata.RenderView {
	if id, ok := rvs.Lookup[name]; ok && id != metadata.InvalidIDUint16 {
		return rvs.RegisteredViews[id]
	}
	return nil
}

/**
 * @brief Builds a render view packet using the provided view and meshes.
 *
 * @param view A pointer to the view to use.
 * @param data Freeform data used to build the packet.
 * @param out_packet A pointer to hold the generated packet.
 * @return True on success; otherwise false.
 */
func (rvs *RenderViewSystem) BuildPacket(view *metadata.RenderView, data interface{}) *metadata.RenderViewPacket {
	if view != nil {
		op, err := view.View.OnBuildPacketRenderView(data)
		if err != nil {
			core.LogError(err.Error())
			return nil
		}
		op.View = view
		return op
	}
	core.LogError("render_view_system_build_packet requires valid pointers to a view and a packet.")
	return nil
}

/**
 * @brief Uses the given view and packet to render the contents therein.
 *
 * @param view A pointer to the view to use.
 * @param packet A pointer to the packet whose data is to be rendered.
 * @param frame_number The current renderer frame number, typically used for data synchronization.
 * @param render_target_index The current render target index for renderers that use multiple render targets at once (i.e. Vulkan).
 * @return True on success; otherwise false.
 */
func (rvs *RenderViewSystem) OnRender(view *metadata.RenderView, packet *metadata.RenderViewPacket, frameNumber, renderTargetIndex uint64) bool {
	if view != nil && packet != nil {

		return view.View.OnRenderRenderView(view, packet, frameNumber, renderTargetIndex)
	}
	core.LogError("render_view_system_on_render requires a valid pointer to a data.")
	return false
}

func (rvs *RenderViewSystem) skyboxOnRenderView(view *metadata.RenderView, packet *metadata.RenderViewPacket, frameNumber, renderTargetIndex uint64) error {
	vs := view.View.(*views.RenderViewSkybox)

	skybox_data := packet.ExtendedData.(*metadata.SkyboxPacketData)

	for p := 0; p < int(view.RenderpassCount); p++ {

		pass := view.Passes[p]

		if !rvs.renderer.RenderPassBegin(pass, pass.Targets[renderTargetIndex]) {
			err := fmt.Errorf("render_view_skybox_on_render pass index %u failed to start.", p)
			return err
		}

		if !rvs.shaderSystem.useByID(vs.ShaderID) {
			err := fmt.Errorf("Failed to use skybox shader. Render frame failed.")
			return err
		}

		// Get the view matrix, but zero out the position so the skybox stays put on screen.
		view_matrix := vs.WorldCamera.GetView()
		view_matrix.Data[12] = 0.0
		view_matrix.Data[13] = 0.0
		view_matrix.Data[14] = 0.0

		// Apply globals
		// TODO: This is terrible. Need to bind by id.
		if !rvs.renderer.ShaderBindGlobals(vs.Shader) {
			err := fmt.Errorf("failed to bind shader globals")
			return err
		}
		if !rvs.shaderSystem.SetUniformByIndex(vs.ProjectionLocation, packet.ProjectionMatrix) {
			err := fmt.Errorf("failed to apply skybox projection uniform")
			return err
		}
		if !rvs.shaderSystem.SetUniformByIndex(vs.ViewLocation, view_matrix) {
			err := fmt.Errorf("failed to apply skybox view uniform")
			return err
		}
		if !rvs.shaderSystem.ApplyGlobal() {
			err := fmt.Errorf("failed to apply shader globals")
			return err
		}

		// Instance
		if !rvs.shaderSystem.BindInstance(skybox_data.Skybox.InstanceID) {
			err := fmt.Errorf("failed to to bind shader instance for skybox")
			return err
		}

		if !rvs.shaderSystem.SetUniformByIndex(vs.CubeMapLocation, skybox_data.Skybox.Cubemap) {
			err := fmt.Errorf("failed to apply skybox cube map uniform")
			return err
		}

		needs_update := skybox_data.Skybox.RenderFrameNumber != frameNumber
		if !rvs.shaderSystem.ApplyInstance(needs_update) {
			err := fmt.Errorf("failed to apply instance for skybox")
			return err
		}

		// Sync the frame number.
		skybox_data.Skybox.RenderFrameNumber = frameNumber

		// Draw it.
		render_data := &metadata.GeometryRenderData{
			Geometry: skybox_data.Skybox.Geometry,
		}

		rvs.renderer.DrawGeometry(render_data)

		if !rvs.renderer.RenderPassEnd(pass) {
			err := fmt.Errorf("render_view_skybox_on_render pass index %u failed to end.", p)
			return err
		}
	}
	return nil
}
