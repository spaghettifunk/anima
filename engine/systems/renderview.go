package systems

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/spaghettifunk/anima/engine/core"
	"github.com/spaghettifunk/anima/engine/math"
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
	fontsystem     *FontSystem
}

func NewRenderViewSystem(config RenderViewSystemConfig, r *RendererSystem, shaderSystem *ShaderSystem, cs *CameraSystem, ms *MaterialSystem, fs *FontSystem) (*RenderViewSystem, error) {
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
		fontsystem:      fs,
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
	// Destroy all views in the system.
	for i := 0; i < int(rvs.MaxViewCount); i++ {
		view := rvs.RegisteredViews[i]
		if view.ID != metadata.InvalidIDUint16 {

			switch view.ViewConfig.RenderViewType {
			case metadata.RENDERER_VIEW_KNOWN_TYPE_PICK:
				data := view.InternalData.(*views.RenderViewPick)
				if err := rvs.pickOnDestroy(data); err != nil {
					core.LogError("failed to destroy pick renderview")
					return err
				}
			case metadata.RENDERER_VIEW_KNOWN_TYPE_SKYBOX:
				data := view.InternalData.(*views.RenderViewSkybox)
				if err := rvs.skyboxOnDestroy(data); err != nil {
					core.LogError("failed to destroy skybox renderview")
					return err
				}
			case metadata.RENDERER_VIEW_KNOWN_TYPE_UI:
				data := view.InternalData.(*views.RenderViewUI)
				if err := rvs.uiOnDestroy(data); err != nil {
					core.LogError("failed to destroy ui renderview")
					return err
				}
			case metadata.RENDERER_VIEW_KNOWN_TYPE_WORLD:
				data := view.InternalData.(*views.RenderViewWorld)
				if err := rvs.worldOnDestroy(data); err != nil {
					core.LogError("failed to destroy world renderview")
					return err
				}
			}

			// Destroy its renderpasses.
			for p := 0; p < int(view.RenderpassCount); p++ {
				rvs.renderer.RenderPassDestroy(view.Passes[p])
			}
			view.ID = metadata.InvalidIDUint16
		}
	}
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
	view.Name = config.Name
	view.CustomShaderName = config.CustomShaderName
	view.RenderpassCount = config.PassCount
	view.Passes = make([]*metadata.RenderPass, view.RenderpassCount)

	for i := uint8(0); i < view.RenderpassCount; i++ {
		p, err := rvs.renderer.RenderPassCreate(config.Passes[i], view.Passes[i])
		if err != nil {
			core.LogDebug("failed to create renderpass")
			return err
		}
		view.Passes[i] = p
	}

	uniforms := map[string]uint16{}
	switch config.RenderViewType {
	case metadata.RENDERER_VIEW_KNOWN_TYPE_WORLD:
		v, err := rvs.worldOnRenderViewCreate(view, uniforms)
		if err != nil {
			return err
		}
		view.View = v
	case metadata.RENDERER_VIEW_KNOWN_TYPE_UI:
		v, err := rvs.uiOnRenderViewCreate(view, uniforms)
		if err != nil {
			return err
		}
		view.View = v
	case metadata.RENDERER_VIEW_KNOWN_TYPE_SKYBOX:
		v, err := rvs.skyboxOnRenderViewCreate(view, uniforms)
		if err != nil {
			return err
		}
		view.View = v
	case metadata.RENDERER_VIEW_KNOWN_TYPE_PICK:
		v, err := rvs.pickOnRenderViewCreate(view, uniforms)
		if err != nil {
			return err
		}
		view.View = v
	default:
		err := fmt.Errorf("not a valid render view type")
		return err
	}

	// save current configuration
	view.ViewConfig = config

	// register event for each view
	core.EventRegister(core.EVENT_CODE_DEFAULT_RENDERTARGET_REFRESH_REQUIRED, rvs.renderViewOnEvent)

	// Call the on create
	if !view.View.OnCreate(uniforms) {
		err := fmt.Errorf("failed to create view")
		rvs.RegisteredViews[id] = nil
		return err
	}

	rvs.RegenerateRenderTargets(view)

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

				rvs.RegisteredViews[i].View.OnResize(width, height)

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
func (rvs *RenderViewSystem) BuildPacket(view *metadata.RenderView, data interface{}) (*metadata.RenderViewPacket, error) {
	if view != nil {
		switch view.RenderViewType {
		case metadata.RENDERER_VIEW_KNOWN_TYPE_PICK:
			return rvs.pickOnBuildPacket(view, data)
		case metadata.RENDERER_VIEW_KNOWN_TYPE_SKYBOX:
			return rvs.skyboxOnBuildPacket(view, data)
		case metadata.RENDERER_VIEW_KNOWN_TYPE_UI:
			return rvs.uiOnBuildPacket(view, data)
		case metadata.RENDERER_VIEW_KNOWN_TYPE_WORLD:
			return rvs.worldOnBuildPacket(view, data)
		default:
			err := fmt.Errorf("invalid renderview type")
			return nil, err
		}
	}
	err := fmt.Errorf("render_view_system_build_packet requires valid pointers to a view and a packet")
	return nil, err
}

/**
 * @brief Uses the given view and packet to render the contents therein.
 *
 * @param view A pointer to the view to use.
 * @param packet A pointer to the packet whose data is to be rendered.
 * @param frame_number The current renderer frame number, typically used for data synchronization.
 * @param renderTargetIndex The current render target index for renderers that use multiple render targets at once (i.e. Vulkan).
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
		case metadata.RENDERER_VIEW_KNOWN_TYPE_PICK:
			return rvs.pickOnRenderView(view, packet, frameNumber, renderTargetIndex)
		default:
			err := fmt.Errorf("not a valid render view type")
			return err
		}
	}
	err := fmt.Errorf("render_view_system_on_render requires a valid pointer to a data")
	return err
}

func (rvs *RenderViewSystem) RegenerateRenderTargets(view *metadata.RenderView) {
	// Create render targets for each. TODO: Should be configurable.

	for r := uint32(0); r < uint32(view.RenderpassCount); r++ {
		pass := view.Passes[r]

		for i := uint8(0); i < pass.RenderTargetCount; i++ {
			target := pass.Targets[i]

			// Destroy the old first if it exists.
			// TODO: check if a resize is actually needed for this target.
			rvs.renderer.RenderTargetDestroy(target, false)

			for a := 0; a < int(target.AttachmentCount); a++ {
				attachment := target.Attachments[a]
				if attachment.Source == metadata.RENDER_TARGET_ATTACHMENT_SOURCE_DEFAULT {
					switch attachment.RenderTargetAttachmentType {
					case metadata.RENDER_TARGET_ATTACHMENT_TYPE_COLOUR:
						attachment.Texture = rvs.renderer.backend.WindowAttachmentGet(i)
					case metadata.RENDER_TARGET_ATTACHMENT_TYPE_DEPTH:
						attachment.Texture = rvs.renderer.backend.DepthAttachmentGet(i)
					default:
						core.LogFatal("unsupported attachment type: 0x%d", attachment.RenderTargetAttachmentType)
						continue
					}
				} else if attachment.Source == metadata.RENDER_TARGET_ATTACHMENT_SOURCE_VIEW {
					if view.RenderViewType == metadata.RENDERER_VIEW_KNOWN_TYPE_PICK {
						if err := rvs.pickRegenerateAttachmentTarget(view.InternalData.(*views.RenderViewPick), r, attachment); err != nil {
							core.LogFatal("failed to regenerate attachment target for pick with error %s", err.Error())
							return
						}
					}

					if !view.View.RegenerateAttachmentTarget(r, attachment) {
						core.LogError("view failed to regenerate attachment target for attachment type: 0x%d", attachment.RenderTargetAttachmentType)
					}
				}
			}

			// Create the render target.
			tc, err := rvs.renderer.RenderTargetCreate(
				target.AttachmentCount,
				target.Attachments,
				pass,
				// NOTE: just going off the first attachment size here, but should be enough for most cases.
				target.Attachments[0].Texture.Width,
				target.Attachments[0].Texture.Height,
			)
			if err != nil {
				core.LogError("failed to render target create with error %s", err.Error())
				return
			}
			pass.Targets[i] = tc
		}
	}
}

func (rvs *RenderViewSystem) renderViewOnEvent(context core.EventContext) {
	view, ok := context.Data.(*metadata.RenderView)
	if !ok {
		return
	}

	switch context.Type {
	case core.EVENT_CODE_DEFAULT_RENDERTARGET_REFRESH_REQUIRED:
		rvs.RegenerateRenderTargets(view)
	}
}

// Dedicated functions for each renderview
// FIXME: this is not the proper way of coding, however, I realised that the code is a bit too spaghetti
// for now this solution work

/* SKYBOX */
func (rvs *RenderViewSystem) skyboxOnRenderViewCreate(view *metadata.RenderView, uniforms map[string]uint16) (*views.RenderViewSkybox, error) {
	res, err := rvs.renderer.assetManager.LoadAsset("Shader.Builtin.Skybox", metadata.ResourceTypeShader, nil)
	if err != nil {
		return nil, err
	}
	shaderCfg := res.Data.(*metadata.ShaderConfig)
	shader, err := rvs.shaderSystem.CreateShader(view.Passes[0], shaderCfg, true)
	if err != nil {
		return nil, err
	}
	c := rvs.cameraSystem.GetDefault()
	v := views.NewRenderViewSkybox(view, shader, c)

	uniforms["projection"] = rvs.shaderSystem.GetUniformIndex(shader, "projection")
	uniforms["view"] = rvs.shaderSystem.GetUniformIndex(shader, "view")
	uniforms["cube_texture"] = rvs.shaderSystem.GetUniformIndex(shader, "cube_texture")

	return v, nil
}

func (rvs *RenderViewSystem) skyboxOnRenderView(view *metadata.RenderView, packet *metadata.RenderViewPacket, frameNumber, renderTargetIndex uint64) error {
	vs := view.View.(*views.RenderViewSkybox)

	skybox_data := packet.ExtendedData.(*metadata.SkyboxPacketData)

	for p := 0; p < int(view.RenderpassCount); p++ {
		pass := view.Passes[p]

		if err := rvs.renderer.RenderPassBegin(pass, pass.Targets[renderTargetIndex]); err != nil {
			core.LogError("render_view_skybox_on_render pass index %d failed to start", p)
			return err
		}

		if vs.ShaderID != vs.Shader.ID {
			return fmt.Errorf("shader ID not correct")
		}

		if err := rvs.shaderSystem.UseShaderByID(vs.ShaderID); err != nil {
			core.LogError("failed to use skybox shader. Render frame failed")
			return err
		}

		// Get the view matrix, but zero out the position so the skybox stays put on screen.
		view_matrix := vs.WorldCamera.GetView()
		view_matrix.Data[12] = 0.0
		view_matrix.Data[13] = 0.0
		view_matrix.Data[14] = 0.0

		// Apply globals
		if err := rvs.renderer.ShaderBindGlobals(vs.Shader); err != nil {
			core.LogError("failed to bind shader globals")
			return err
		}
		if err := rvs.shaderSystem.SetUniformByIndex(vs.ProjectionLocation, packet.ProjectionMatrix); err != nil {
			core.LogError("failed to apply skybox projection uniform")
			return err
		}
		if err := rvs.shaderSystem.SetUniformByIndex(vs.ViewLocation, view_matrix); err != nil {
			core.LogError("failed to apply skybox view uniform")
			return err
		}
		if err := rvs.shaderSystem.ApplyGlobal(); err != nil {
			core.LogError("failed to apply shader globals")
			return err
		}

		// Instance
		if err := rvs.shaderSystem.BindInstance(skybox_data.Skybox.InstanceID); err != nil {
			core.LogError("failed to to bind shader instance for skybox")
			return err
		}

		if err := rvs.shaderSystem.SetUniformByIndex(vs.CubeMapLocation, skybox_data.Skybox.Cubemap); err != nil {
			core.LogError("failed to apply skybox cube map uniform")
			return err
		}

		needs_update := skybox_data.Skybox.RenderFrameNumber != frameNumber
		if err := rvs.shaderSystem.ApplyInstance(needs_update); err != nil {
			core.LogError("failed to apply instance for skybox")
			return err
		}

		// Sync the frame number.
		skybox_data.Skybox.RenderFrameNumber = frameNumber

		// Draw it.
		render_data := &metadata.GeometryRenderData{
			Geometry: skybox_data.Skybox.Geometry,
		}

		rvs.renderer.DrawGeometry(render_data)

		if err := rvs.renderer.RenderPassEnd(pass); err != nil {
			core.LogError("render_view_skybox_on_render pass index %d failed to end", p)
			return err
		}
	}
	return nil
}

func (rvs *RenderViewSystem) skyboxOnBuildPacket(view *metadata.RenderView, data interface{}) (*metadata.RenderViewPacket, error) {
	op, err := view.View.OnBuildPacket(data)
	if err != nil {
		return nil, err
	}
	return op, nil
}

func (rvs *RenderViewSystem) skyboxOnDestroy(view *views.RenderViewSkybox) error {
	if err := view.OnDestroy(); err != nil {
		return err
	}
	return nil
}

/* UI */
func (rvs *RenderViewSystem) uiOnRenderViewCreate(view *metadata.RenderView, uniforms map[string]uint16) (*views.RenderViewUI, error) {
	res, err := rvs.renderer.assetManager.LoadAsset("Shader.Builtin.UI", metadata.ResourceTypeShader, nil)
	if err != nil {
		return nil, err
	}
	shaderCfg := res.Data.(*metadata.ShaderConfig)
	shader, err := rvs.shaderSystem.CreateShader(view.Passes[0], shaderCfg, true)
	if err != nil {
		return nil, err
	}
	v := views.NewRenderViewUI(view, shader)

	uniforms["diffuse_texture"] = rvs.shaderSystem.GetUniformIndex(shader, "diffuse_texture")
	uniforms["diffuse_colour"] = rvs.shaderSystem.GetUniformIndex(shader, "diffuse_colour")
	uniforms["model"] = rvs.shaderSystem.GetUniformIndex(shader, "model")

	return v, nil
}

func (rvs *RenderViewSystem) uiOnRenderView(view *metadata.RenderView, packet *metadata.RenderViewPacket, frameNumber, renderTargetIndex uint64) error {
	return nil
}

func (rvs *RenderViewSystem) uiOnBuildPacket(view *metadata.RenderView, data interface{}) (*metadata.RenderViewPacket, error) {
	op, err := view.View.OnBuildPacket(data)
	if err != nil {
		return nil, err
	}
	return op, nil
}

func (rvs *RenderViewSystem) uiOnDestroy(view *views.RenderViewUI) error {
	if err := view.OnDestroy(); err != nil {
		return err
	}
	return nil
}

/* WORLD */
func (rvs *RenderViewSystem) worldOnRenderViewCreate(view *metadata.RenderView, uniforms map[string]uint16) (*views.RenderViewWorld, error) {
	res, err := rvs.renderer.assetManager.LoadAsset("Shader.Builtin.Material", metadata.ResourceTypeShader, nil)
	if err != nil {
		return nil, err
	}
	shaderCfg := res.Data.(*metadata.ShaderConfig)
	shader, err := rvs.shaderSystem.CreateShader(view.Passes[0], shaderCfg, true)
	if err != nil {
		return nil, err
	}
	c := rvs.cameraSystem.GetDefault()
	v := views.NewRenderViewWorld(view, shader, c)

	return v, nil
}

func (rvs *RenderViewSystem) worldOnRenderView(view *metadata.RenderView, packet *metadata.RenderViewPacket, frameNumber, renderTargetIndex uint64) error {
	data := view.View.(*views.RenderViewWorld)

	for p := uint32(0); p < uint32(view.RenderpassCount); p++ {
		pass := view.Passes[p]
		if err := rvs.renderer.RenderPassBegin(pass, pass.Targets[renderTargetIndex]); err != nil {
			core.LogError("render_view_world_on_render pass index %d failed to start", p)
			return err
		}

		if err := rvs.shaderSystem.UseShaderByID(data.ShaderID); err != nil {
			core.LogError("failed to use material shader. Render frame failed")
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
			if err := rvs.materialSystem.ApplyLocal(material, packet.Geometries[i].Model); err != nil {
				core.LogError("failed to apply local for material system")
				return err
			}

			// Draw it.
			rvs.renderer.DrawGeometry(packet.Geometries[i])
		}

		if err := rvs.renderer.RenderPassEnd(pass); err != nil {
			core.LogError("render_view_world_on_render pass index %d failed to end", p)
			return err
		}
	}

	return nil
}

func (rvs *RenderViewSystem) worldOnBuildPacket(view *metadata.RenderView, data interface{}) (*metadata.RenderViewPacket, error) {
	op, err := view.View.OnBuildPacket(data)
	if err != nil {
		return nil, err
	}
	return op, nil
}

func (rvs *RenderViewSystem) worldOnDestroy(view *views.RenderViewWorld) error {
	if err := view.OnDestroy(); err != nil {
		return err
	}
	return nil
}

/* PICK */
func (rvs *RenderViewSystem) pickOnRenderViewCreate(view *metadata.RenderView, uniforms map[string]uint16) (*views.RenderViewPick, error) {
	res, err := rvs.renderer.assetManager.LoadAsset("Shader.Builtin.UIPick", metadata.ResourceTypeShader, nil)
	if err != nil {
		return nil, err
	}
	shaderPickCfg := res.Data.(*metadata.ShaderConfig)
	shaderPick, err := rvs.shaderSystem.CreateShader(view.Passes[0], shaderPickCfg, true)
	if err != nil {
		return nil, err
	}

	res, err = rvs.renderer.assetManager.LoadAsset("Shader.Builtin.WorldPick", metadata.ResourceTypeShader, nil)
	if err != nil {
		return nil, err
	}
	shaderWorldPickCfg := res.Data.(*metadata.ShaderConfig)
	shaderWorldPick, err := rvs.shaderSystem.CreateShader(view.Passes[1], shaderWorldPickCfg, true)
	if err != nil {
		return nil, err
	}
	c := rvs.cameraSystem.GetDefault()
	v := views.NewRenderViewPick(view, shaderPick, shaderWorldPick, c)

	// Extract uniform locations
	uniforms["ui_id_colour"] = rvs.shaderSystem.GetUniformIndex(v.UIShaderInfo.Shader, "id_colour")
	uniforms["ui_model"] = rvs.shaderSystem.GetUniformIndex(v.UIShaderInfo.Shader, "model")
	uniforms["ui_projection"] = rvs.shaderSystem.GetUniformIndex(v.UIShaderInfo.Shader, "projection")
	uniforms["ui_view"] = rvs.shaderSystem.GetUniformIndex(v.UIShaderInfo.Shader, "view")

	uniforms["world_id_colour"] = rvs.shaderSystem.GetUniformIndex(v.WorldShaderInfo.Shader, "id_colour")
	uniforms["world_model"] = rvs.shaderSystem.GetUniformIndex(v.WorldShaderInfo.Shader, "model")
	uniforms["world_projection"] = rvs.shaderSystem.GetUniformIndex(v.WorldShaderInfo.Shader, "projection")
	uniforms["world_view"] = rvs.shaderSystem.GetUniformIndex(v.WorldShaderInfo.Shader, "view")

	return v, nil
}

func (rvs *RenderViewSystem) pickOnRenderView(view *metadata.RenderView, packet *metadata.RenderViewPacket, frameNumber, renderTargetIndex uint64) error {
	data := view.InternalData.(*views.RenderViewPick)

	p := uint32(0)
	pass := view.Passes[p] // First pass

	if renderTargetIndex == 0 {
		// Reset.
		for i := 0; i < len(data.InstanceUpdated); i++ {
			data.InstanceUpdated[i] = false
		}

		if err := rvs.renderer.RenderPassBegin(pass, pass.Targets[renderTargetIndex]); err != nil {
			core.LogError("render_view_ui_on_render pass index %d failed to start", p)
			return err
		}

		packet_data := packet.ExtendedData.(*metadata.PickPacketData)

		// World
		if err := rvs.shaderSystem.UseShaderByID(data.WorldShaderInfo.Shader.ID); err != nil {
			core.LogError("failed to use world pick shader. Render frame failed")
			return err
		}

		// Apply globals
		if err := rvs.shaderSystem.SetUniformByIndex(data.WorldShaderInfo.ProjectionLocation, data.WorldShaderInfo.Projection); err != nil {
			core.LogError("failed to apply projection matrix")
			return err
		}

		if err := rvs.shaderSystem.SetUniformByIndex(data.WorldShaderInfo.ViewLocation, data.WorldShaderInfo.View); err != nil {
			core.LogError("failed to apply view matrix")
			return err
		}

		if err := rvs.shaderSystem.ApplyGlobal(); err != nil {
			core.LogError("failed to apply globals shader")
			return err
		}

		// Draw geometries. Start from 0 since world geometries are added first, and stop at the world geometry count.
		for i := uint32(0); i < packet_data.WorldGeometryCount; i++ {
			geo := packet.Geometries[i]
			if geo == nil {
				continue
			}
			current_instance_id := geo.UniqueID

			if err := rvs.shaderSystem.BindInstance(current_instance_id); err != nil {
				core.LogError("failed to bind instance for shader with id %d", current_instance_id)
				return err
			}

			// Get colour based on id
			r, g, b := math.UInt32ToRGB(geo.UniqueID)
			id_colour := math.RGBUInt32ToVec3(r, g, b)

			if err := rvs.shaderSystem.SetUniformByIndex(data.WorldShaderInfo.IDColorLocation, id_colour); err != nil {
				core.LogError("failed to apply id colour uniform")
				return err
			}

			needs_update := !data.InstanceUpdated[current_instance_id]
			if err := rvs.shaderSystem.ApplyInstance(needs_update); err != nil {
				core.LogError("failed to apply shader instance to world geometry")
				return err
			}
			data.InstanceUpdated[current_instance_id] = true

			// Apply the locals
			if err := rvs.shaderSystem.SetUniformByIndex(data.WorldShaderInfo.ModelLocation, geo.Model); err != nil {
				core.LogError("failed to apply model matrix for world geometry")
				return err
			}

			// Draw it.
			rvs.renderer.DrawGeometry(packet.Geometries[i])
		}

		if err := rvs.renderer.RenderPassEnd(pass); err != nil {
			core.LogError("render_view_ui_on_render pass index %d failed to end", p)
			return err
		}

		p++
		pass = view.Passes[p] // Second pass

		if err := rvs.renderer.RenderPassBegin(pass, pass.Targets[renderTargetIndex]); err != nil {
			core.LogError("render_view_ui_on_render pass index %d failed to start", p)
			return err
		}

		// UI
		if err := rvs.shaderSystem.UseShaderByID(data.UIShaderInfo.Shader.ID); err != nil {
			core.LogError("failed to use material shader. Render frame failed")
			return err
		}

		// Apply globals
		if err := rvs.shaderSystem.SetUniformByIndex(data.UIShaderInfo.ProjectionLocation, data.UIShaderInfo.Projection); err != nil {
			core.LogError("failed to apply projection matrix")
			return err
		}
		if err := rvs.shaderSystem.SetUniformByIndex(data.UIShaderInfo.ViewLocation, data.UIShaderInfo.View); err != nil {
			core.LogError("failed to apply view matrix")
			return err
		}

		if err := rvs.shaderSystem.ApplyGlobal(); err != nil {
			core.LogError("failed to apply globals shader")
			return err
		}

		// Draw geometries. Start off where world geometries left off.
		for i := packet_data.WorldGeometryCount; i < packet.GeometryCount; i++ {
			geo := packet.Geometries[i]
			current_instance_id := geo.UniqueID

			if err := rvs.shaderSystem.BindInstance(current_instance_id); err != nil {
				core.LogError("failed to bind instance shader")
				return err
			}

			// Get colour based on id
			r, g, b := math.UInt32ToRGB(geo.UniqueID)
			id_colour := math.RGBUInt32ToVec3(r, g, b)
			if err := rvs.shaderSystem.SetUniformByIndex(data.UIShaderInfo.IDColorLocation, id_colour); err != nil {
				core.LogError("failed to apply id colour uniform")
				return err
			}

			needs_update := !data.InstanceUpdated[current_instance_id]
			if err := rvs.shaderSystem.ApplyInstance(needs_update); err != nil {
				core.LogError("failed to apply instance shader for the rest of the geometry")
				return err
			}

			data.InstanceUpdated[current_instance_id] = true

			// Apply the locals
			if err := rvs.shaderSystem.SetUniformByIndex(data.UIShaderInfo.ModelLocation, geo.Model); err != nil {
				core.LogError("failed to apply model matrix for text")
				return err
			}

			// Draw it.
			rvs.renderer.DrawGeometry(packet.Geometries[i])
		}

		if err := rvs.renderer.RenderPassEnd(pass); err != nil {
			core.LogError("render_view_ui_on_render pass index %d failed to end", p)
			return err
		}
	}

	// Clamp to image size
	x_coord := math.Clamp(uint32(data.MouseX), 0, uint32(view.Width-1))
	y_coord := math.Clamp(uint32(data.MouseY), 0, uint32(view.Height-1))

	pixel, err := rvs.renderer.backend.TextureReadPixel(data.ColourTargetAttachmentTexture, x_coord, y_coord)
	if err != nil {
		err := fmt.Errorf("failed to read pixel from texture")
		return err
	}

	// Extract the id from the sampled colour.
	id := metadata.InvalidID
	id = math.RGBUToUInt32(uint32(pixel[0]), uint32(pixel[1]), uint32(pixel[2]))
	if id == 0x00FFFFFF {
		// This is pure white.
		id = metadata.InvalidID
	}

	context := core.EventContext{
		Type: core.EVENT_CODE_OBJECT_HOVER_ID_CHANGED,
		Data: id,
	}
	core.EventFire(context)

	return nil
}

func (rvs *RenderViewSystem) pickOnBuildPacket(view *metadata.RenderView, data interface{}) (*metadata.RenderViewPacket, error) {
	rvp, err := view.View.OnBuildPacket(data)
	if err != nil {
		return nil, err
	}

	packet := data.(*metadata.PickPacketData)
	internalData := view.InternalData.(*views.RenderViewPick)

	// TODO: this needs to take into account the highest id, not the count, because they can and do skip ids.
	// Verify instance resources exist.
	if packet.RequiredInstanceCount > uint32(internalData.InstanceCount) {
		diff := packet.RequiredInstanceCount - uint32(internalData.InstanceCount)
		for i := uint32(0); i < diff; i++ {
			// Not saving the instance id because it doesn't matter.
			// UI shader
			_, err := rvs.renderer.ShaderAcquireInstanceResources(internalData.UIShaderInfo.Shader, nil)
			if err != nil {
				core.LogError("render_view_pick failed to acquire shader resources.")
				return nil, err
			}
			// World shader
			_, err = rvs.renderer.ShaderAcquireInstanceResources(internalData.WorldShaderInfo.Shader, nil)
			if err != nil {
				core.LogError("render_view_pick failed to acquire shader resources.")
				return nil, err
			}
			internalData.InstanceCount++
			internalData.InstanceUpdated = append(internalData.InstanceUpdated, false)
		}
	}
	return rvp, nil
}

func (rvs *RenderViewSystem) pickOnDestroy(view *views.RenderViewPick) error {
	if err := view.OnDestroy(); err != nil {
		return err
	}

	for i := uint32(0); i < uint32(view.InstanceCount); i++ {
		if err := rvs.renderer.ShaderReleaseInstanceResources(view.UIShaderInfo.Shader, i); err != nil {
			core.LogError("failed to release shader resources")
			return err
		}
		if err := rvs.renderer.ShaderReleaseInstanceResources(view.WorldShaderInfo.Shader, i); err != nil {
			core.LogError("failed to release shader resources")
			return err
		}
	}

	rvs.shaderSystem.textureSystem.DestroyTexture(view.ColourTargetAttachmentTexture)
	rvs.shaderSystem.textureSystem.DestroyTexture(view.DepthTargetAttachmentTexture)

	view = nil

	return nil
}

func (rvs *RenderViewSystem) pickRegenerateAttachmentTarget(view *views.RenderViewPick, passIndex uint32, attachment *metadata.RenderTargetAttachment) error {
	if attachment.RenderTargetAttachmentType == metadata.RENDER_TARGET_ATTACHMENT_TYPE_COLOUR {
		attachment.Texture = view.ColourTargetAttachmentTexture
	} else if attachment.RenderTargetAttachmentType == metadata.RENDER_TARGET_ATTACHMENT_TYPE_DEPTH {
		attachment.Texture = view.DepthTargetAttachmentTexture
	} else {
		err := fmt.Errorf("unsupported attachment type 0x%d", attachment.RenderTargetAttachmentType)
		return err
	}

	if passIndex == 1 {
		// Do not need to regenerate for both passes since they both use the same attachment.
		// Just attach it and move on.
		return nil
	}

	// Destroy current attachment if it exists.
	if attachment.Texture.InternalData != nil {
		rvs.renderer.TextureDestroy(attachment.Texture)

	}

	// Setup a new texture.
	// Generate a UUID to act as the texture name.
	texture_name_uuid := uuid.New()

	width := view.View.Passes[passIndex].RenderArea.Z
	height := view.View.Passes[passIndex].RenderArea.W
	has_transparency := false // TODO: configurable

	attachment.Texture.ID = metadata.InvalidID
	attachment.Texture.TextureType = metadata.TextureType2d
	attachment.Texture.Name = texture_name_uuid.String()
	attachment.Texture.Width = uint32(width)
	attachment.Texture.Height = uint32(height)
	attachment.Texture.ChannelCount = 4 // TODO: configurable
	attachment.Texture.Generation = metadata.InvalidID

	attachment.Texture.Flags |= metadata.TextureFlagBits(metadata.TextureFlagHasTransparency)
	if !has_transparency {
		attachment.Texture.Flags |= 0
	}
	attachment.Texture.Flags |= metadata.TextureFlagBits(metadata.TextureFlagIsWriteable)

	if attachment.RenderTargetAttachmentType == metadata.RENDER_TARGET_ATTACHMENT_TYPE_DEPTH {
		attachment.Texture.Flags |= metadata.TextureFlagBits(metadata.TextureFlagDepth)
	}
	attachment.Texture.InternalData = nil

	if err := rvs.renderer.TextureCreateWriteable(attachment.Texture); err != nil {
		return err
	}

	return nil
}
