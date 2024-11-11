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
	renderer       *RendererSystem
	shaderSystem   *ShaderSystem
	cameraSystem   *CameraSystem
	materialSystem *MaterialSystem
}

func NewRenderViewSystem(config RenderViewSystemConfig, r *RendererSystem, shaderSystem *ShaderSystem, cs *CameraSystem, ms *MaterialSystem) (*RenderViewSystem, error) {
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
		materialSystem:  ms,
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
	switch config.RenderViewType {
	case metadata.RENDERER_VIEW_KNOWN_TYPE_WORLD:
		shader, err := rvs.shaderSystem.GetShader("Shader.Builtin.Material")
		if err != nil {
			return err
		}
		c := rvs.cameraSystem.GetDefault()
		view.View = views.NewRenderViewSkybox(shader, c)
	case metadata.RENDERER_VIEW_KNOWN_TYPE_UI:
		shader, err := rvs.shaderSystem.GetShader("Shader.Builtin.UI")
		if err != nil {
			return err
		}
		view.View = views.NewRenderViewUI(shader)

		uniforms["diffuse_texture"] = rvs.shaderSystem.GetUniformIndex(shader, "diffuse_texture")
		uniforms["diffuse_colour"] = rvs.shaderSystem.GetUniformIndex(shader, "diffuse_colour")
		uniforms["model"] = rvs.shaderSystem.GetUniformIndex(shader, "model")
	case metadata.RENDERER_VIEW_KNOWN_TYPE_SKYBOX:
		shader, err := rvs.shaderSystem.GetShader("Shader.Builtin.Skybox")
		if err != nil {
			return err
		}
		c := rvs.cameraSystem.GetDefault()
		view.View = views.NewRenderViewSkybox(shader, c)

		uniforms["projection"] = rvs.shaderSystem.GetUniformIndex(shader, "projection")
		uniforms["view"] = rvs.shaderSystem.GetUniformIndex(shader, "view")
		uniforms["cube_texture"] = rvs.shaderSystem.GetUniformIndex(shader, "cube_texture")
	default:
		err := fmt.Errorf("not a valid render view type")
		return err
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
func (rvs *RenderViewSystem) OnRender(view *metadata.RenderView, packet *metadata.RenderViewPacket, frameNumber, renderTargetIndex uint64) error {
	if view != nil && packet != nil {
		switch view.ViewConfig.RenderViewType {
		case metadata.RENDERER_VIEW_KNOWN_TYPE_WORLD:
			return rvs.worldOnRenderView(view, packet, frameNumber, renderTargetIndex)
		case metadata.RENDERER_VIEW_KNOWN_TYPE_UI:
			return rvs.uiOnRenderView(view, packet, frameNumber, renderTargetIndex)
		case metadata.RENDERER_VIEW_KNOWN_TYPE_SKYBOX:
			return rvs.skyboxOnRenderView(view, packet, frameNumber, renderTargetIndex)
		default:
			err := fmt.Errorf("not a valid render view type")
			return err
		}
	}
	err := fmt.Errorf("render_view_system_on_render requires a valid pointer to a data")
	return err
}

func (rvs *RenderViewSystem) skyboxOnRenderView(view *metadata.RenderView, packet *metadata.RenderViewPacket, frameNumber, renderTargetIndex uint64) error {
	vs := view.View.(*views.RenderViewSkybox)

	skybox_data := packet.ExtendedData.(*metadata.SkyboxPacketData)

	for p := 0; p < int(view.RenderpassCount); p++ {
		pass := view.Passes[p]

		if !rvs.renderer.RenderPassBegin(pass, pass.Targets[renderTargetIndex]) {
			err := fmt.Errorf("render_view_skybox_on_render pass index %d failed to start", p)
			return err
		}

		if !rvs.shaderSystem.useByID(vs.ShaderID) {
			err := fmt.Errorf("failed to use skybox shader. Render frame failed")
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
			err := fmt.Errorf("render_view_skybox_on_render pass index %d failed to end", p)
			return err
		}
	}
	return nil
}

func (rvs *RenderViewSystem) uiOnRenderView(view *metadata.RenderView, packet *metadata.RenderViewPacket, frameNumber, renderTargetIndex uint64) error {
	return nil
}

func (rvs *RenderViewSystem) worldOnRenderView(view *metadata.RenderView, packet *metadata.RenderViewPacket, frameNumber, renderTargetIndex uint64) error {
	data := view.View.(*views.RenderViewWorld)

	for p := uint32(0); p < uint32(view.RenderpassCount); p++ {
		pass := view.Passes[p]
		if !rvs.renderer.RenderPassBegin(pass, pass.Targets[renderTargetIndex]) {
			err := fmt.Errorf("render_view_world_on_render pass index %d failed to start", p)
			return err
		}

		if !rvs.shaderSystem.useByID(data.ShaderID) {
			err := fmt.Errorf("Failed to use material shader. Render frame failed.")
			return err
		}

		// Apply globals
		// TODO: Find a generic way to request data such as ambient colour (which should be from a scene),
		// and mode (from the renderer)
		if !rvs.materialSystem.ApplyGlobal(data.ShaderID, frameNumber, packet.ProjectionMatrix, packet.ViewMatrix, packet.AmbientColour.ToVec3(), packet.ViewPosition, uint32(data.RenderMode)) {
			err := fmt.Errorf("failed to use apply globals for material shader. Render frame failed")
			return err
		}

		// Draw geometries.
		count := packet.GeometryCount
		for i := uint32(0); i < count; i++ {
			material := &metadata.Material{}
			if packet.Geometries[i].Geometry.Material != nil {
				material = packet.Geometries[i].Geometry.Material
			} else {
				material = rvs.materialSystem.DefaultMaterial
			}

			// Update the material if it hasn't already been this frame. This keeps the
			// same material from being updated multiple times. It still needs to be bound
			// either way, so this check result gets passed to the backend which either
			// updates the internal shader bindings and binds them, or only binds them.
			needs_update := material.RenderFrameNumber != uint32(frameNumber)
			if !rvs.materialSystem.ApplyInstance(material, needs_update) {
				core.LogWarn("failed to apply material '%s'. Skipping draw", material.Name)
				continue
			} else {
				// Sync the frame number.
				material.RenderFrameNumber = uint32(frameNumber)
			}

			// Apply the locals
			if !rvs.materialSystem.ApplyLocal(material, packet.Geometries[i].Model) {
				err := fmt.Errorf("failed to apply local for material system")
				return err
			}

			// Draw it.
			rvs.renderer.DrawGeometry(packet.Geometries[i])
		}

		if !rvs.renderer.RenderPassEnd(pass) {
			err := fmt.Errorf("render_view_world_on_render pass index %u failed to end.", p)
			return err
		}
	}

	return nil
}
