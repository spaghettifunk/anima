package systems

import (
	"fmt"
	"sort"

	mt "math"

	"github.com/google/uuid"
	"github.com/spaghettifunk/anima/engine/core"
	"github.com/spaghettifunk/anima/engine/math"
	"github.com/spaghettifunk/anima/engine/renderer/metadata"
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
		err := fmt.Errorf("func NewRenderViewSystem - config.MaxViewCount must be > 0")
		return nil, err
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

			switch view.RenderViewType {
			case metadata.RENDERER_VIEW_KNOWN_TYPE_PICK:
				data := view.InternalData.(*metadata.RenderViewPick)
				if err := rvs.pickOnDestroy(data); err != nil {
					core.LogError("failed to destroy pick renderview")
					return err
				}
			case metadata.RENDERER_VIEW_KNOWN_TYPE_SKYBOX:
				data := view.InternalData.(*metadata.RenderViewSkybox)
				if err := rvs.skyboxOnDestroy(data); err != nil {
					core.LogError("failed to destroy skybox renderview")
					return err
				}
			case metadata.RENDERER_VIEW_KNOWN_TYPE_UI:
				data := view.InternalData.(*metadata.RenderViewUI)
				if err := rvs.uiOnDestroy(data); err != nil {
					core.LogError("failed to destroy ui renderview")
					return err
				}
			case metadata.RENDERER_VIEW_KNOWN_TYPE_WORLD:
				data := view.InternalData.(*metadata.RenderViewWorld)
				if err := rvs.worldOnDestroy(data); err != nil {
					core.LogError("failed to destroy world renderview")
					return err
				}
			}

			// Destroy its renderpasses.
			for p := 0; p < int(view.RenderpassCount); p++ {
				if err := rvs.renderer.RenderPassDestroy(view.Passes[p], true); err != nil {
					core.LogError("failed to render pass destroy")
					return err
				}
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
		p, err := rvs.renderer.RenderPassCreate(config.PassConfigs[i])
		if err != nil {
			core.LogError("failed to create renderpass")
			return err
		}
		view.Passes[i] = p
	}

	switch config.RenderViewType {
	case metadata.RENDERER_VIEW_KNOWN_TYPE_WORLD:
		if err := rvs.worldOnRenderViewCreate(view); err != nil {
			return err
		}
	case metadata.RENDERER_VIEW_KNOWN_TYPE_UI:
		if err := rvs.uiOnRenderViewCreate(view); err != nil {
			return err
		}
	case metadata.RENDERER_VIEW_KNOWN_TYPE_SKYBOX:
		if err := rvs.skyboxOnRenderViewCreate(view); err != nil {
			return err
		}
	case metadata.RENDERER_VIEW_KNOWN_TYPE_PICK:
		if err := rvs.pickOnRenderViewCreate(view); err != nil {
			return err
		}
	default:
		err := fmt.Errorf("not a valid render view type")
		return err
	}

	// register event for each view
	core.EventRegister(core.EVENT_CODE_DEFAULT_RENDERTARGET_REFRESH_REQUIRED, rvs.renderViewOnEvent)

	if err := rvs.RegenerateRenderTargets(view); err != nil {
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

				switch rvs.RegisteredViews[i].RenderViewType {
				case metadata.RENDERER_VIEW_KNOWN_TYPE_PICK:
					vp := rvs.RegisteredViews[i].InternalData.(*metadata.RenderViewPick)
					vp.UIShaderInfo.Projection = math.NewMat4Orthographic(0.0, float32(width), float32(height), 0.0, vp.UIShaderInfo.NearClip, vp.UIShaderInfo.FarClip)
					aspect := float32(width / height)
					vp.WorldShaderInfo.Projection = math.NewMat4Perspective(vp.WorldShaderInfo.FOV, aspect, vp.WorldShaderInfo.NearClip, vp.WorldShaderInfo.FarClip)
				case metadata.RENDERER_VIEW_KNOWN_TYPE_SKYBOX:
					aspect := width / height
					vs := rvs.RegisteredViews[i].InternalData.(*metadata.RenderViewSkybox)
					vs.ProjectionMatrix = math.NewMat4Perspective(vs.FOV, float32(aspect), vs.NearClip, vs.FarClip)
				case metadata.RENDERER_VIEW_KNOWN_TYPE_UI:
					vu := rvs.RegisteredViews[i].InternalData.(*metadata.RenderViewUI)
					vu.ProjectionMatrix = math.NewMat4Orthographic(0.0, float32(width), float32(height), 0.0, vu.NearClip, vu.FarClip)
				case metadata.RENDERER_VIEW_KNOWN_TYPE_WORLD:
					aspect := float32(width / height)
					vw := rvs.RegisteredViews[i].InternalData.(*metadata.RenderViewWorld)
					vw.ProjectionMatrix = math.NewMat4Perspective(vw.FOV, aspect, vw.NearClip, vw.FarClip)
				default:
					core.LogError("renderview type with value %d does not exist. Skip", rvs.RegisteredViews[i].RenderViewType)
					continue
				}

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
func (rvs *RenderViewSystem) OnRender(packet *metadata.RenderViewPacket, frameNumber, renderTargetIndex uint64) error {
	if packet != nil && packet.View != nil {
		switch packet.View.RenderViewType {
		case metadata.RENDERER_VIEW_KNOWN_TYPE_WORLD:
			return rvs.worldOnRenderView(packet, frameNumber, renderTargetIndex)
		case metadata.RENDERER_VIEW_KNOWN_TYPE_UI:
			return rvs.uiOnRenderView(packet, frameNumber, renderTargetIndex)
		case metadata.RENDERER_VIEW_KNOWN_TYPE_SKYBOX:
			return rvs.skyboxOnRenderView(packet, frameNumber, renderTargetIndex)
		case metadata.RENDERER_VIEW_KNOWN_TYPE_PICK:
			return rvs.pickOnRenderView(packet, frameNumber, renderTargetIndex)
		default:
			err := fmt.Errorf("not a valid render view type")
			return err
		}
	}
	return nil
}

func (rvs *RenderViewSystem) OnDestroyPacket(packet *metadata.RenderViewPacket) error {
	packet.Geometries = nil
	packet = nil
	return nil
}

func (rvs *RenderViewSystem) RegenerateRenderTargets(view *metadata.RenderView) error {
	// Create render targets for each. TODO: Should be configurable.
	for r := uint32(0); r < uint32(view.RenderpassCount); r++ {
		pass := view.Passes[r]

		for i := uint8(0); i < pass.RenderTargetCount; i++ {
			target := pass.Targets[i]

			// Destroy the old first if it exists.
			// TODO: check if a resize is actually needed for this target.
			if err := rvs.renderer.RenderTargetDestroy(target, false); err != nil {
				core.LogError("failed to render target destroy")
				return err
			}

			for a := 0; a < int(target.AttachmentCount); a++ {
				attachment := target.Attachments[a]
				if attachment.Source == metadata.RENDER_TARGET_ATTACHMENT_SOURCE_DEFAULT {
					switch attachment.RenderTargetAttachmentType {
					case metadata.RENDER_TARGET_ATTACHMENT_TYPE_COLOUR:
						attachment.Texture = rvs.renderer.backend.WindowAttachmentGet(i)
					case metadata.RENDER_TARGET_ATTACHMENT_TYPE_DEPTH:
						attachment.Texture = rvs.renderer.backend.DepthAttachmentGet(i)
					default:
						err := fmt.Errorf("unsupported attachment type: 0x%d", attachment.RenderTargetAttachmentType)
						return err
					}
				} else if attachment.Source == metadata.RENDER_TARGET_ATTACHMENT_SOURCE_VIEW {
					if view.RenderViewType == metadata.RENDERER_VIEW_KNOWN_TYPE_PICK {
						if err := rvs.pickRegenerateAttachmentTarget(view.InternalData.(*metadata.RenderViewPick), pass, attachment); err != nil {
							core.LogError("failed to regenerate attachment target for pick with error %s", err.Error())
							return err
						}
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
				return err
			}
			pass.Targets[i] = tc
		}
	}
	return nil
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
func (rvs *RenderViewSystem) skyboxOnRenderViewCreate(view *metadata.RenderView) error {
	res, err := rvs.renderer.assetManager.LoadAsset("Shader.Builtin.Skybox", metadata.ResourceTypeShader, nil)
	if err != nil {
		return err
	}
	shaderCfg := res.Data.(*metadata.ShaderConfig)
	shader, err := rvs.shaderSystem.CreateShader(view.Passes[0], shaderCfg, true)
	if err != nil {
		return err
	}
	rvskb := &metadata.RenderViewSkybox{
		ProjectionLocation: rvs.shaderSystem.GetUniformIndex(shader, "projection"),
		ViewLocation:       rvs.shaderSystem.GetUniformIndex(shader, "view"),
		CubeMapLocation:    rvs.shaderSystem.GetUniformIndex(shader, "cube_texture"),
		NearClip:           0.1,
		FarClip:            1000.0,
		FOV:                math.DegToRad(45.0),
		WorldCamera:        rvs.cameraSystem.GetDefault(),
	}
	// Default
	rvskb.ProjectionMatrix = math.NewMat4Perspective(rvskb.FOV, 1280/720.0, rvskb.NearClip, rvskb.FarClip)

	view.InternalData = rvskb

	return nil
}

func (rvs *RenderViewSystem) skyboxOnRenderView(packet *metadata.RenderViewPacket, frameNumber, renderTargetIndex uint64) error {
	vs := packet.View.InternalData.(*metadata.RenderViewSkybox)

	skybox_data := packet.ExtendedData.(*metadata.SkyboxPacketData)

	for p := 0; p < int(packet.View.RenderpassCount); p++ {
		pass := packet.View.Passes[p]

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
	if data == nil {
		err := fmt.Errorf("render_view_skybox_on_build_packet requires valid pointer to view, packet, and data")
		return nil, err
	}

	skybox_data := data.(*metadata.SkyboxPacketData)
	vs := view.InternalData.(*metadata.RenderViewSkybox)

	// Set matrices, etc.
	out_packet := &metadata.RenderViewPacket{
		ProjectionMatrix: vs.ProjectionMatrix,
		ViewMatrix:       vs.WorldCamera.GetView(),
		ViewPosition:     vs.WorldCamera.GetPosition(),
		ExtendedData:     skybox_data,
	}

	return out_packet, nil
}

func (rvs *RenderViewSystem) skyboxOnDestroy(view *metadata.RenderViewSkybox) error {
	// nothing to do here for now
	return nil
}

/* UI */
func (rvs *RenderViewSystem) uiOnRenderViewCreate(view *metadata.RenderView) error {
	res, err := rvs.renderer.assetManager.LoadAsset("Shader.Builtin.UI", metadata.ResourceTypeShader, nil)
	if err != nil {
		return err
	}
	shaderCfg := res.Data.(*metadata.ShaderConfig)
	shader, err := rvs.shaderSystem.CreateShader(view.Passes[0], shaderCfg, true)
	if err != nil {
		return err
	}

	rvui := &metadata.RenderViewUI{
		ShaderID:              shader.ID,
		Shader:                shader,
		DiffuseMapLocation:    rvs.shaderSystem.GetUniformIndex(shader, "diffuse_texture"),
		DiffuseColourLocation: rvs.shaderSystem.GetUniformIndex(shader, "diffuse_colour"),
		ModelLocation:         rvs.shaderSystem.GetUniformIndex(shader, "model"),
		// TODO: Set from configuration.
		NearClip:   -100.0,
		FarClip:    100.0,
		ViewMatrix: math.NewMat4Identity(),
	}

	// Default
	rvui.ProjectionMatrix = math.NewMat4Orthographic(0.0, 1280.0, 720.0, 0.0, rvui.NearClip, rvui.FarClip)

	view.InternalData = rvui

	return nil
}

func (rvs *RenderViewSystem) uiOnRenderView(packet *metadata.RenderViewPacket, frameNumber, renderTargetIndex uint64) error {
	data := packet.View.InternalData.(*metadata.RenderViewUI)

	for p := uint32(0); p < uint32(packet.View.RenderpassCount); p++ {
		pass := packet.View.Passes[p]

		if err := rvs.renderer.RenderPassBegin(pass, pass.Targets[renderTargetIndex]); err != nil {
			core.LogError("render_view_ui_on_render pass index %d failed to start", p)
			return err
		}

		if err := rvs.shaderSystem.UseShaderByID(data.ShaderID); err != nil {
			core.LogError("failed to use material shader. Render frame failed")
			return err
		}

		// Apply globals
		if !rvs.materialSystem.ApplyGlobal(data.ShaderID, frameNumber, packet.ProjectionMatrix, packet.ViewMatrix, math.NewVec3Zero(), math.NewVec3Zero(), 0) {
			err := fmt.Errorf("failed to use apply globals for material shader. Render frame failed")
			return err
		}

		// Draw geometries.
		count := packet.GeometryCount
		for i := uint32(0); i < count; i++ {
			var m *metadata.Material
			if packet.Geometries[i].Geometry.Material != nil {
				m = packet.Geometries[i].Geometry.Material
			} else {
				m = rvs.materialSystem.GetDefault()
			}

			// Update the material if it hasn't already been this frame. This keeps the
			// same material from being updated multiple times. It still needs to be bound
			// either way, so this check result gets passed to the backend which either
			// updates the internal shader bindings and binds them, or only binds them.
			needs_update := m.RenderFrameNumber != uint32(frameNumber)
			if !rvs.materialSystem.ApplyInstance(m, needs_update) {
				core.LogWarn("failed to apply material '%s'. Skipping draw", m.Name)
				continue
			} else {
				// Sync the frame number.
				m.RenderFrameNumber = uint32(frameNumber)
			}

			// Apply the locals
			if err := rvs.materialSystem.ApplyLocal(m, packet.Geometries[i].Model); err != nil {
				return err
			}

			// Draw it.
			rvs.renderer.DrawGeometry(packet.Geometries[i])
		}

		// // Draw bitmap text
		// packet_data := packet.ExtendedData.(*metadata.UIPacketData)  // array of texts
		// for i := 0; i < packet_data->text_count; ++i) {
		//     ui_text* text = packet_data->texts[i];
		//     shader_system_bind_instance(text->instance_id);

		//     if (!shader_system_uniform_set_by_index(data->diffuse_map_location, &text->data->atlas)) {
		//         KERROR("Failed to apply bitmap font diffuse map uniform.");
		//         return false;
		//     }

		//     // TODO: font colour.
		//     static vec4 white_colour = (vec4){1.0f, 1.0f, 1.0f, 1.0f};  // white
		//     if (!shader_system_uniform_set_by_index(data->diffuse_colour_location, &white_colour)) {
		//         KERROR("Failed to apply bitmap font diffuse colour uniform.");
		//         return false;
		//     }
		//     b8 needs_update = text->render_frame_number != frame_number;
		//     shader_system_apply_instance(needs_update);

		//     // Sync the frame number.
		//     text->render_frame_number = frame_number;

		//     // Apply the locals
		//     mat4 model = transform_get_world(&text->transform);
		//     if(!shader_system_uniform_set_by_index(data->model_location, &model)) {
		//         KERROR("Failed to apply model matrix for text");
		//     }

		//     ui_text_draw(text);
		// }

		if err := rvs.renderer.RenderPassEnd(pass); err != nil {
			core.LogError("render_view_ui_on_render pass index %d failed to end", p)
			return err
		}
	}

	return nil
}

func (rvs *RenderViewSystem) uiOnBuildPacket(view *metadata.RenderView, data interface{}) (*metadata.RenderViewPacket, error) {
	packet_data := data.(*metadata.UIPacketData)
	rvu := view.InternalData.(*metadata.RenderViewUI)

	out_packet := &metadata.RenderViewPacket{
		Geometries: []*metadata.GeometryRenderData{},
		// Set matrices, etc.
		ProjectionMatrix: rvu.ProjectionMatrix,
		ViewMatrix:       rvu.ViewMatrix,
		// TODO: temp set extended data to the test text objects for now.
		ExtendedData: packet_data,
	}

	// Obtain all geometries from the current scene.
	// Iterate all meshes and add them to the packet's geometries collection
	for i := 0; i < int(packet_data.MeshData.MeshCount); i++ {
		m := packet_data.MeshData.Meshes[i]
		for j := 0; j < int(m.GeometryCount); j++ {
			render_data := &metadata.GeometryRenderData{
				Geometry: m.Geometries[j],
				Model:    m.Transform.GetWorld(),
			}
			out_packet.Geometries = append(out_packet.Geometries, render_data)
			out_packet.GeometryCount++
		}
	}

	return out_packet, nil
}

func (rvs *RenderViewSystem) uiOnDestroy(view *metadata.RenderViewUI) error {
	// nothing to do here for now
	view = nil
	return nil
}

/* WORLD */
func (rvs *RenderViewSystem) worldOnRenderViewCreate(view *metadata.RenderView) error {
	res, err := rvs.renderer.assetManager.LoadAsset("Shader.Builtin.Material", metadata.ResourceTypeShader, nil)
	if err != nil {
		return err
	}
	shaderCfg := res.Data.(*metadata.ShaderConfig)
	shader, err := rvs.shaderSystem.CreateShader(view.Passes[0], shaderCfg, true)
	if err != nil {
		return err
	}

	rvw := &metadata.RenderViewWorld{
		ShaderID:    shader.ID,
		Shader:      shader,
		WorldCamera: rvs.cameraSystem.GetDefault(),
		// Get either the custom shader override or the defined default.
		// TODO: Set from configuration.
		NearClip:      0.1,
		FarClip:       1000.0,
		FOV:           math.DegToRad(45.0),
		AmbientColour: math.NewVec4(0.25, 0.25, 0.25, 1.0),
	}

	// Default
	rvw.ProjectionMatrix = math.NewMat4Perspective(rvw.FOV, 1280/720.0, rvw.NearClip, rvw.FarClip)

	// Listen for mode changes.
	core.EventRegister(core.EVENT_CODE_SET_RENDER_MODE, rvw.OnSetRenderMode)

	view.InternalData = rvw

	return nil
}

func (rvs *RenderViewSystem) worldOnRenderView(packet *metadata.RenderViewPacket, frameNumber, renderTargetIndex uint64) error {
	rvw := packet.View.InternalData.(*metadata.RenderViewWorld)

	for p := uint32(0); p < uint32(packet.View.RenderpassCount); p++ {
		pass := packet.View.Passes[p]
		if err := rvs.renderer.RenderPassBegin(pass, pass.Targets[renderTargetIndex]); err != nil {
			core.LogError("render_view_world_on_render pass index %d failed to start", p)
			return err
		}

		if err := rvs.shaderSystem.UseShaderByID(rvw.ShaderID); err != nil {
			core.LogError("failed to use material shader. Render frame failed")
			return err
		}

		// Apply globals
		// TODO: Find a generic way to request data such as ambient colour (which should be from a scene),
		// and mode (from the renderer)
		if !rvs.materialSystem.ApplyGlobal(rvw.ShaderID, frameNumber, packet.ProjectionMatrix, packet.ViewMatrix, packet.AmbientColour.ToVec3(), packet.ViewPosition, uint32(rvw.RenderMode)) {
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
	mesh_data := data.(*metadata.MeshPacketData)

	rvw := view.InternalData.(*metadata.RenderViewWorld)

	out_packet := &metadata.RenderViewPacket{
		Geometries:       []*metadata.GeometryRenderData{},
		ProjectionMatrix: rvw.ProjectionMatrix,
		ViewMatrix:       rvw.WorldCamera.GetView(),
		ViewPosition:     rvw.WorldCamera.GetPosition(),
		AmbientColour:    rvw.AmbientColour,
	}

	// Obtain all geometries from the current scene.
	geometry_distances := []*metadata.GeometryDistance{}

	for i := uint32(0); i < mesh_data.MeshCount; i++ {
		m := mesh_data.Meshes[i]
		model := m.Transform.GetWorld()

		for j := uint32(0); j < uint32(m.GeometryCount); j++ {
			render_data := &metadata.GeometryRenderData{
				Geometry: m.Geometries[j],
				Model:    model,
			}

			// TODO: Add something to material to check for transparency.
			if (m.Geometries[j].Material.DiffuseMap.Texture.Flags & metadata.TextureFlagBits(metadata.TextureFlagHasTransparency)) == 0 {
				// Only add meshes with _no_ transparency.
				out_packet.Geometries = append(out_packet.Geometries, render_data)
				out_packet.GeometryCount++
			} else {
				// For meshes _with_ transparency, add them to a separate list to be sorted by distance later.
				// Get the center, extract the global position from the model matrix and add it to the center,
				// then calculate the distance between it and the camera, and finally save it to a list to be sorted.
				// NOTE: This isn't perfect for translucent meshes that intersect, but is enough for our purposes now.
				center := render_data.Geometry.Center.Transform(model)
				distance := center.Distance(rvw.WorldCamera.Position)

				gdist := &metadata.GeometryDistance{
					Distance:           float32(mt.Abs(float64(distance))),
					GeometryRenderData: render_data,
				}

				geometry_distances = append(geometry_distances, gdist)
			}
		}
	}

	// Sort the distances
	// FIXME: validate if it is the correct ordering
	sort.Slice(geometry_distances, func(i, j int) bool {
		return geometry_distances[i].Distance < geometry_distances[j].Distance
	})

	// Add them to the packet geometry.
	for i := 0; i < len(geometry_distances); i++ {
		out_packet.Geometries = append(out_packet.Geometries, geometry_distances[i].GeometryRenderData)
		out_packet.GeometryCount++
	}

	return out_packet, nil
}

func (rvs *RenderViewSystem) worldOnDestroy(view *metadata.RenderViewWorld) error {
	// nothing to do for now
	view = nil
	return nil
}

/* PICK */
func (rvs *RenderViewSystem) pickOnRenderViewCreate(view *metadata.RenderView) error {
	res, err := rvs.renderer.assetManager.LoadAsset("Shader.Builtin.UIPick", metadata.ResourceTypeShader, nil)
	if err != nil {
		return err
	}
	shaderPickCfg := res.Data.(*metadata.ShaderConfig)
	shaderUIrPick, err := rvs.shaderSystem.CreateShader(view.Passes[0], shaderPickCfg, true)
	if err != nil {
		return err
	}

	res, err = rvs.renderer.assetManager.LoadAsset("Shader.Builtin.WorldPick", metadata.ResourceTypeShader, nil)
	if err != nil {
		return err
	}
	shaderWorldPickCfg := res.Data.(*metadata.ShaderConfig)
	shaderWorldPick, err := rvs.shaderSystem.CreateShader(view.Passes[1], shaderWorldPickCfg, true)
	if err != nil {
		return err
	}

	rvp := &metadata.RenderViewPick{
		UIShaderInfo: &metadata.PickShaderInfo{
			Shader:             shaderUIrPick,
			Renderpass:         view.Passes[0],
			IDColorLocation:    rvs.shaderSystem.GetUniformIndex(shaderUIrPick, "id_colour"),
			ModelLocation:      rvs.shaderSystem.GetUniformIndex(shaderUIrPick, "model"),
			ProjectionLocation: rvs.shaderSystem.GetUniformIndex(shaderUIrPick, "projection"),
			ViewLocation:       rvs.shaderSystem.GetUniformIndex(shaderUIrPick, "view"),
			NearClip:           -100.0,
			FarClip:            100.0,
			FOV:                0,
			View:               math.NewMat4Identity(),
		},
		WorldShaderInfo: &metadata.PickShaderInfo{
			Shader:             shaderWorldPick,
			Renderpass:         view.Passes[1],
			IDColorLocation:    rvs.shaderSystem.GetUniformIndex(shaderWorldPick, "id_colour"),
			ModelLocation:      rvs.shaderSystem.GetUniformIndex(shaderWorldPick, "model"),
			ProjectionLocation: rvs.shaderSystem.GetUniformIndex(shaderWorldPick, "projection"),
			ViewLocation:       rvs.shaderSystem.GetUniformIndex(shaderWorldPick, "view"),

			// Default World properties
			NearClip: 0.1,
			FarClip:  1000.0,
			FOV:      math.DegToRad(45.0),
			View:     math.NewMat4Identity(),
		},
		InstanceUpdated:               make([]bool, 1),
		InstanceCount:                 0,
		ColourTargetAttachmentTexture: &metadata.Texture{},
		DepthTargetAttachmentTexture:  &metadata.Texture{},
		WorldCamera:                   rvs.cameraSystem.GetDefault(),
	}

	rvp.UIShaderInfo.Projection = math.NewMat4Orthographic(0.0, 1280.0, 720.0, 0.0, rvp.UIShaderInfo.NearClip, rvp.UIShaderInfo.FarClip)
	rvp.WorldShaderInfo.Projection = math.NewMat4Perspective(rvp.WorldShaderInfo.FOV, 1280/720.0, rvp.WorldShaderInfo.NearClip, rvp.WorldShaderInfo.FarClip)

	core.EventRegister(core.EVENT_CODE_MOUSE_MOVED, rvp.OnMouseMoved)

	view.InternalData = rvp

	return nil
}

func (rvs *RenderViewSystem) pickOnRenderView(packet *metadata.RenderViewPacket, frameNumber, renderTargetIndex uint64) error {
	data := packet.View.InternalData.(*metadata.RenderViewPick)

	p := uint32(0)
	pass := packet.View.Passes[p] // First pass

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
		pass = packet.View.Passes[p] // Second pass

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
	x_coord := math.Clamp(uint32(data.MouseX), 0, uint32(packet.View.Width-1))
	y_coord := math.Clamp(uint32(data.MouseY), 0, uint32(packet.View.Height-1))

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
	if data == nil {
		err := fmt.Errorf("render_view_pick_on_build_packet requires valid pointer to view, packet, and data")
		return nil, err
	}

	rvp := view.InternalData.(*metadata.RenderViewPick)
	packet_data := data.(*metadata.PickPacketData)

	out_packet := &metadata.RenderViewPacket{
		Geometries:   make([]*metadata.GeometryRenderData, 1),
		ExtendedData: packet_data,
	}

	// TODO: Get active camera.
	rvp.WorldShaderInfo.View = rvp.WorldCamera.GetView()

	// Set the pick packet data to extended data.
	packet_data.WorldGeometryCount = 0
	packet_data.UIGeometryCount = 0
	out_packet.ExtendedData = data

	highest_instance_id := uint32(0)
	// Iterate all meshes in world data.
	for i := 0; i < int(packet_data.WorldMeshData.MeshCount); i++ {
		m := packet_data.WorldMeshData.Meshes[i]
		for j := 0; j < int(m.GeometryCount); j++ {
			render_data := &metadata.GeometryRenderData{
				Geometry: m.Geometries[j],
				Model:    m.Transform.GetWorld(),
				UniqueID: m.UniqueID,
			}
			out_packet.Geometries = append(out_packet.Geometries, render_data)
			out_packet.GeometryCount++
			packet_data.WorldGeometryCount++
		}
		// Count all geometries as a single id.
		if m.UniqueID > highest_instance_id {
			highest_instance_id = m.UniqueID
		}
	}

	// Iterate all meshes in UI data.
	for i := 0; i < int(packet_data.UIMeshData.MeshCount); i++ {
		m := packet_data.UIMeshData.Meshes[i]
		for j := 0; j < int(m.GeometryCount); j++ {
			render_data := &metadata.GeometryRenderData{
				Geometry: m.Geometries[j],
				Model:    m.Transform.GetWorld(),
				UniqueID: m.UniqueID,
			}
			out_packet.Geometries = append(out_packet.Geometries, render_data)
			out_packet.GeometryCount++
			packet_data.UIGeometryCount++
		}
		// Count all geometries as a single id.
		if m.UniqueID > highest_instance_id {
			highest_instance_id = m.UniqueID
		}
	}

	// Count texts as well.
	// for i := 0; i < int(packet_data.TextCount); i++ {
	// 	if packet_data.Texts[i].UniqueID > highest_instance_id {
	// 		highest_instance_id = packet_data.Texts[i].UniqueID
	// 	}
	// }

	packet_data.RequiredInstanceCount = highest_instance_id + 1

	// TODO: this needs to take into account the highest id, not the count, because they can and do skip ids.
	// Verify instance resources exist.
	if packet_data.RequiredInstanceCount > uint32(rvp.InstanceCount) {
		diff := packet_data.RequiredInstanceCount - uint32(rvp.InstanceCount)
		for i := uint32(0); i < diff; i++ {
			// Not saving the instance id because it doesn't matter.
			// UI shader
			_, err := rvs.renderer.ShaderAcquireInstanceResources(rvp.UIShaderInfo.Shader, nil)
			if err != nil {
				core.LogError("render_view_pick failed to acquire shader resources.")
				return nil, err
			}
			// World shader
			_, err = rvs.renderer.ShaderAcquireInstanceResources(rvp.WorldShaderInfo.Shader, nil)
			if err != nil {
				core.LogError("render_view_pick failed to acquire shader resources.")
				return nil, err
			}
			rvp.InstanceCount++
			rvp.InstanceUpdated = append(rvp.InstanceUpdated, false)
		}
	}

	return out_packet, nil
}

func (rvs *RenderViewSystem) pickOnDestroy(view *metadata.RenderViewPick) error {
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

func (rvs *RenderViewSystem) pickRegenerateAttachmentTarget(view *metadata.RenderViewPick, pass *metadata.RenderPass, attachment *metadata.RenderTargetAttachment) error {
	if attachment.RenderTargetAttachmentType == metadata.RENDER_TARGET_ATTACHMENT_TYPE_COLOUR {
		attachment.Texture = view.ColourTargetAttachmentTexture
	} else if attachment.RenderTargetAttachmentType == metadata.RENDER_TARGET_ATTACHMENT_TYPE_DEPTH {
		attachment.Texture = view.DepthTargetAttachmentTexture
	} else {
		err := fmt.Errorf("unsupported attachment type 0x%d", attachment.RenderTargetAttachmentType)
		return err
	}

	// Destroy current attachment if it exists.
	if attachment.Texture.InternalData != nil {
		if err := rvs.renderer.TextureDestroy(attachment.Texture); err != nil {
			return err
		}
	}

	// Setup a new texture.
	// Generate a UUID to act as the texture name.
	texture_name_uuid := uuid.New()

	width := pass.RenderArea.Z
	height := pass.RenderArea.W
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
