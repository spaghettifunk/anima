package systems

import (
	"fmt"

	"github.com/spaghettifunk/anima/engine/assets"
	"github.com/spaghettifunk/anima/engine/core"
	"github.com/spaghettifunk/anima/engine/platform"
	"github.com/spaghettifunk/anima/engine/renderer/metadata"
	"github.com/spaghettifunk/anima/engine/renderer/vulkan"
)

type RendererSystem struct {
	backend      *vulkan.VulkanRenderer
	assetManager *assets.AssetManager

	// application
	AppName   string
	AppWidth  uint32
	AppHeight uint32

	// engine specific
	Platform *platform.Platform

	SkyboxShaderID   uint32
	MaterialShaderID uint32
	UIShaderID       uint32

	// The number of render targets. Typically lines up with the amount of swapchain images.
	WindowRenderTargetCount uint8
	// The current window framebuffer width.
	FramebufferWidth uint32
	// The current window framebuffer height.
	FramebufferHeight uint32

	// A pointer to the skybox renderpass. TODO: Configurable via views.
	SkyboxRenderPass *metadata.RenderPass
	// A pointer to the world renderpass. TODO: Configurable via views.
	WorldRenderPass *metadata.RenderPass
	// A pointer to the UI renderpass. TODO: Configurable via views.
	UIRenderPass *metadata.RenderPass
	// Indicates if the window is currently being resized.
	Resizing bool
	// The current number of frames since the last resize operation.'
	// Only set if resizing = true. Otherwise 0.
	FramesSinceResize uint8
}

func NewRendererSystem(appName string, appWidth, appHeight uint32, platform *platform.Platform, am *assets.AssetManager) (*RendererSystem, error) {
	renderer := &RendererSystem{
		backend:      vulkan.New(platform, am),
		assetManager: am,
		AppName:      appName,
		AppWidth:     appWidth,
		AppHeight:    appHeight,
	}
	return renderer, nil
}

func (r *RendererSystem) Initialize(shaderSystem *ShaderSystem) error {
	// Default framebuffer size. Overridden when window is created.
	r.FramebufferWidth = 1280
	r.FramebufferHeight = 720
	r.Resizing = false
	r.FramesSinceResize = 0
	r.backend.FrameNumber = 0

	SkyboxRenderPassName := "Renderpass.Builtin.Skybox"
	WorldRenderPassName := "Renderpass.Builtin.World"
	UIRenderPassName := "Renderpass.Builtin.UI"

	rbc := &metadata.RendererBackendConfig{
		ApplicationName: r.AppName,
	}

	if err := r.backend.Initialize(rbc, &r.WindowRenderTargetCount); err != nil {
		return err
	}

	/*
// Load render views

    // Skybox view
    render_view_config skybox_config = {};
    skybox_config.type = RENDERER_VIEW_KNOWN_TYPE_SKYBOX;
    skybox_config.width = 0;
    skybox_config.height = 0;
    skybox_config.name = "skybox";
    skybox_config.view_matrix_source = RENDER_VIEW_VIEW_MATRIX_SOURCE_SCENE_CAMERA;

    // Renderpass config.
    skybox_config.pass_count = 1;
    renderpass_config skybox_passes[1];
    skybox_passes[0].name = "Renderpass.Builtin.Skybox";
    skybox_passes[0].render_area = (vec4){0, 0, 1280, 720};  // Default render area resolution.
    skybox_passes[0].clear_colour = (vec4){0.0f, 0.0f, 0.2f, 1.0f};
    skybox_passes[0].clear_flags = RENDERPASS_CLEAR_COLOUR_BUFFER_FLAG;
    skybox_passes[0].depth = 1.0f;
    skybox_passes[0].stencil = 0;

    render_target_attachment_config skybox_target_attachment = {};
    // Color attachment.
    skybox_target_attachment.type = RENDER_TARGET_ATTACHMENT_TYPE_COLOUR;
    skybox_target_attachment.source = RENDER_TARGET_ATTACHMENT_SOURCE_DEFAULT;
    skybox_target_attachment.load_operation = RENDER_TARGET_ATTACHMENT_LOAD_OPERATION_DONT_CARE;
    skybox_target_attachment.store_operation = RENDER_TARGET_ATTACHMENT_STORE_OPERATION_STORE;
    skybox_target_attachment.present_after = false;

    skybox_passes[0].target.attachment_count = 1;
    skybox_passes[0].target.attachments = &skybox_target_attachment;
    skybox_passes[0].render_target_count = renderer_window_attachment_count_get();

    skybox_config.passes = skybox_passes;

    if (!render_view_system_create(&skybox_config)) {
        KFATAL("Failed to create skybox view. Aborting application.");
        return false;
    }

    // World view.
    render_view_config world_view_config = {};
    world_view_config.type = RENDERER_VIEW_KNOWN_TYPE_WORLD;
    world_view_config.width = 0;
    world_view_config.height = 0;
    world_view_config.name = "world";
    world_view_config.view_matrix_source = RENDER_VIEW_VIEW_MATRIX_SOURCE_SCENE_CAMERA;

    // Renderpass config.
    world_view_config.pass_count = 1;
    renderpass_config world_passes[1] = {0};
    world_passes[0].name = "Renderpass.Builtin.World";
    world_passes[0].render_area = (vec4){0, 0, 1280, 720};  // Default render area resolution.
    world_passes[0].clear_colour = (vec4){0.0f, 0.0f, 0.2f, 1.0f};
    world_passes[0].clear_flags = RENDERPASS_CLEAR_DEPTH_BUFFER_FLAG | RENDERPASS_CLEAR_STENCIL_BUFFER_FLAG;
    world_passes[0].depth = 1.0f;
    world_passes[0].stencil = 0;

    render_target_attachment_config world_target_attachments[2] = {0};
    // Colour attachment
    world_target_attachments[0].type = RENDER_TARGET_ATTACHMENT_TYPE_COLOUR;
    world_target_attachments[0].source = RENDER_TARGET_ATTACHMENT_SOURCE_DEFAULT;
    world_target_attachments[0].load_operation = RENDER_TARGET_ATTACHMENT_LOAD_OPERATION_LOAD;
    world_target_attachments[0].store_operation = RENDER_TARGET_ATTACHMENT_STORE_OPERATION_STORE;
    world_target_attachments[0].present_after = false;
    // Depth attachment
    world_target_attachments[1].type = RENDER_TARGET_ATTACHMENT_TYPE_DEPTH;
    world_target_attachments[1].source = RENDER_TARGET_ATTACHMENT_SOURCE_DEFAULT;
    world_target_attachments[1].load_operation = RENDER_TARGET_ATTACHMENT_LOAD_OPERATION_DONT_CARE;
    world_target_attachments[1].store_operation = RENDER_TARGET_ATTACHMENT_STORE_OPERATION_STORE;
    world_target_attachments[1].present_after = false;

    world_passes[0].target.attachment_count = 2;
    world_passes[0].target.attachments = world_target_attachments;
    world_passes[0].render_target_count = renderer_window_attachment_count_get();

    world_view_config.passes = world_passes;

    if (!render_view_system_create(&world_view_config)) {
        KFATAL("Failed to create world view. Aborting application.");
        return false;
    }

    // UI view
    render_view_config ui_view_config = {};
    ui_view_config.type = RENDERER_VIEW_KNOWN_TYPE_UI;
    ui_view_config.width = 0;
    ui_view_config.height = 0;
    ui_view_config.name = "ui";
    ui_view_config.view_matrix_source = RENDER_VIEW_VIEW_MATRIX_SOURCE_SCENE_CAMERA;

    // Renderpass config
    ui_view_config.pass_count = 1;
    renderpass_config ui_passes[1];
    ui_passes[0].name = "Renderpass.Builtin.UI";
    ui_passes[0].render_area = (vec4){0, 0, 1280, 720};
    ui_passes[0].clear_colour = (vec4){0.0f, 0.0f, 0.2f, 1.0f};
    ui_passes[0].clear_flags = RENDERPASS_CLEAR_NONE_FLAG;
    ui_passes[0].depth = 1.0f;
    ui_passes[0].stencil = 0;

    render_target_attachment_config ui_target_attachment = {};
    // Colour attachment.
    ui_target_attachment.type = RENDER_TARGET_ATTACHMENT_TYPE_COLOUR;
    ui_target_attachment.source = RENDER_TARGET_ATTACHMENT_SOURCE_DEFAULT;
    ui_target_attachment.load_operation = RENDER_TARGET_ATTACHMENT_LOAD_OPERATION_LOAD;
    ui_target_attachment.store_operation = RENDER_TARGET_ATTACHMENT_STORE_OPERATION_STORE;
    ui_target_attachment.present_after = true;

    ui_passes[0].target.attachment_count = 1;
    ui_passes[0].target.attachments = &ui_target_attachment;
    ui_passes[0].render_target_count = renderer_window_attachment_count_get();

    ui_view_config.passes = ui_passes;

    if (!render_view_system_create(&ui_view_config)) {
        KFATAL("Failed to create ui view. Aborting application.");
        return false;
    }

    // Pick pass.
    render_view_config pick_view_config = {};
    pick_view_config.type = RENDERER_VIEW_KNOWN_TYPE_PICK;
    pick_view_config.width = 0;
    pick_view_config.height = 0;
    pick_view_config.name = "pick";
    pick_view_config.view_matrix_source = RENDER_VIEW_VIEW_MATRIX_SOURCE_SCENE_CAMERA;

    pick_view_config.pass_count = 2;
    renderpass_config pick_passes[2] = {0};

    // World pass
    pick_passes[0].name = "Renderpass.Builtin.WorldPick";
    pick_passes[0].render_area = (vec4){0, 0, 1280, 720};
    pick_passes[0].clear_colour = (vec4){1.0f, 1.0f, 1.0f, 1.0f};  // HACK: clearing to white for better visibility// TODO: Clear to black, as 0 is invalid id.
    pick_passes[0].clear_flags = RENDERPASS_CLEAR_COLOUR_BUFFER_FLAG | RENDERPASS_CLEAR_DEPTH_BUFFER_FLAG;
    pick_passes[0].depth = 1.0f;
    pick_passes[0].stencil = 0;

    render_target_attachment_config world_pick_target_attachments[2];
    world_pick_target_attachments[0].type = RENDER_TARGET_ATTACHMENT_TYPE_COLOUR;
    world_pick_target_attachments[0].source = RENDER_TARGET_ATTACHMENT_SOURCE_VIEW;  // Obtain the attachment from the view.
    world_pick_target_attachments[0].load_operation = RENDER_TARGET_ATTACHMENT_LOAD_OPERATION_DONT_CARE;
    world_pick_target_attachments[0].store_operation = RENDER_TARGET_ATTACHMENT_STORE_OPERATION_STORE;
    world_pick_target_attachments[0].present_after = false;

    world_pick_target_attachments[1].type = RENDER_TARGET_ATTACHMENT_TYPE_DEPTH;
    world_pick_target_attachments[1].source = RENDER_TARGET_ATTACHMENT_SOURCE_VIEW;  // Obtain the attachment from the view.
    world_pick_target_attachments[1].load_operation = RENDER_TARGET_ATTACHMENT_LOAD_OPERATION_DONT_CARE;
    world_pick_target_attachments[1].store_operation = RENDER_TARGET_ATTACHMENT_STORE_OPERATION_STORE;
    world_pick_target_attachments[1].present_after = false;

    pick_passes[0].target.attachment_count = 2;
    pick_passes[0].target.attachments = world_pick_target_attachments;
    pick_passes[0].render_target_count = 1;

    pick_passes[1].name = "Renderpass.Builtin.UIPick";
    pick_passes[1].render_area = (vec4){0, 0, 1280, 720};
    pick_passes[1].clear_colour = (vec4){1.0f, 1.0f, 1.0f, 1.0f};
    pick_passes[1].clear_flags = RENDERPASS_CLEAR_NONE_FLAG;
    pick_passes[1].depth = 1.0f;
    pick_passes[1].stencil = 0;

    render_target_attachment_config ui_pick_target_attachments[1];
    ui_pick_target_attachments[0].type = RENDER_TARGET_ATTACHMENT_TYPE_COLOUR;
    // Obtain the attachment from the view.
    ui_pick_target_attachments[0].source = RENDER_TARGET_ATTACHMENT_SOURCE_VIEW;
    ui_pick_target_attachments[0].load_operation = RENDER_TARGET_ATTACHMENT_LOAD_OPERATION_LOAD;
    // Need to store it so it can be sampled afterward.
    ui_pick_target_attachments[0].store_operation = RENDER_TARGET_ATTACHMENT_STORE_OPERATION_STORE;
    ui_pick_target_attachments[0].present_after = false;

    pick_passes[1].target.attachment_count = 1;
    pick_passes[1].target.attachments = ui_pick_target_attachments;
    pick_passes[1].render_target_count = 1;

    pick_view_config.passes = pick_passes;
    if (!render_view_system_create(&pick_view_config)) {
        KFATAL("Failed to create pick view. Aborting application.");
        return false;
    }	
	
	*/

	// Shaders

	// Builtin skybox shader.
	config_resource, err := r.assetManager.LoadAsset("Shader.Builtin.Skybox", metadata.ResourceTypeShader, nil)
	if err != nil {
		core.LogError("Failed to load builtin skybox shader.")
		return err
	}

	config := config_resource.Data.(*metadata.ShaderConfig)
	skyboxShader, err := shaderSystem.CreateShader(config)
	if err != nil {
		core.LogError("Failed to load builtin skybox shader.")
		return err
	}
	r.assetManager.UnloadAsset(config_resource)
	r.SkyboxShaderID = shaderSystem.GetShaderID("Shader.Builtin.Skybox")

	if r.SkyboxShaderID != skyboxShader.ID {
		err := fmt.Errorf("why are the IDs different?")
		core.LogError(err.Error())
		return err
	}

	// Builtin material shader.
	config_resource, err = r.assetManager.LoadAsset("Shader.Builtin.Material", metadata.ResourceTypeShader, nil)
	if err != nil {
		core.LogError("Failed to load builtin material shader.")
		return err
	}

	config = config_resource.Data.(*metadata.ShaderConfig)
	materialShader, err := shaderSystem.CreateShader(config)
	if err != nil {
		core.LogError("Failed to load builtin material shader.")
		return err
	}
	r.assetManager.UnloadAsset(config_resource)
	r.MaterialShaderID = shaderSystem.GetShaderID("Shader.Builtin.Material")

	if r.MaterialShaderID != materialShader.ID {
		err := fmt.Errorf("why are the IDs different?")
		core.LogError(err.Error())
		return err
	}

	// Builtin UI shader.
	config_resource, err = r.assetManager.LoadAsset("Shader.Builtin.UI", metadata.ResourceTypeShader, nil)
	if err != nil {
		core.LogError("Failed to load builtin UI shader.")
		return err
	}

	config = config_resource.Data.(*metadata.ShaderConfig)
	uiShader, err := shaderSystem.CreateShader(config)
	if err != nil {
		core.LogError("Failed to load builtin UI shader.")
		return err
	}
	r.assetManager.UnloadAsset(config_resource)
	r.UIShaderID = shaderSystem.GetShaderID("Shader.Builtin.UI")

	if r.UIShaderID != uiShader.ID {
		err := fmt.Errorf("why are the IDs different?")
		core.LogError(err.Error())
		return err
	}

	return nil
}

func (r *RendererSystem) Shutdown() error {
	// Destroy render targets.
	for i := 0; i < int(r.WindowRenderTargetCount); i++ {
		r.backend.RenderTargetDestroy(r.SkyboxRenderPass.Targets[i], true)
		r.backend.RenderTargetDestroy(r.WorldRenderPass.Targets[i], true)
		r.backend.RenderTargetDestroy(r.UIRenderPass.Targets[i], true)
	}
	return r.backend.Shutdow()
}

func (r *RendererSystem) BeginFrame(deltaTime float64) error {
	return r.backend.BeginFrame(deltaTime)
}

func (r *RendererSystem) EndFrame(deltaTime float64) error {
	return r.backend.EndFrame(deltaTime)
}

func (r *RendererSystem) OnResize(width, height uint16) error {
	// Flag as resizing and store the change, but wait to regenerate.
	r.Resizing = true
	r.FramebufferWidth = uint32(width)
	r.FramebufferHeight = uint32(height)
	// Also reset the frame count since the last  resize operation.
	r.FramesSinceResize = 0

	// return r.backend.Resized(width, height)
	return nil
}

func (r *RendererSystem) DrawFrame(packet *metadata.RenderPacket, renderViewSystem *RenderViewSystem) error {
	r.backend.FrameNumber++

	// Make sure the window is not currently being resized by waiting a designated
	// number of frames after the last resize operation before performing the backend updates.
	if r.Resizing {
		r.FramesSinceResize++

		// If the required number of frames have passed since the resize, go ahead and perform the actual updates.
		if r.FramesSinceResize >= 30 {
			width := r.FramebufferWidth
			height := r.FramebufferHeight
			renderViewSystem.OnWindowResize(width, height)
			r.backend.Resized(width, height)

			r.FramesSinceResize = 0
			r.Resizing = false
		} else {
			// Skip rendering the frame and try again next time.
			// NOTE: Simulate a frame being "drawn" at 60 FPS.
			// platform_sleep(16);
			return nil
		}
	}

	// If the begin frame returned successfully, mid-frame operations may continue.
	if err := r.backend.BeginFrame(packet.DeltaTime); err == nil {
		attachmentIndex := r.backend.WindowAttachmentIndexGet()

		// Render each view.
		for i := 0; i < len(packet.Views); i++ {
			if err := renderViewSystem.OnRender(packet.Views[i].View, packet.Views[i], r.backend.FrameNumber, attachmentIndex); err != nil {
				core.LogError("error rendering view index %d", i)
				return err
			}
		}

		// End the frame. If this fails, it is likely unrecoverable.
		if err := r.backend.EndFrame(packet.DeltaTime); err != nil {
			err := fmt.Errorf("backend func EndFrame failed. Application shutting down")
			core.LogError(err.Error())
			return err
		}
	}

	return nil
}

func (r *RendererSystem) TextureCreate(pixels []uint8, texture *metadata.Texture) {
	r.backend.TextureCreate(pixels, texture)
}

func (r *RendererSystem) TextureDestroy(texture *metadata.Texture) {
	r.backend.TextureDestroy(texture)
}

func (r *RendererSystem) TextureCreateWriteable(texture *metadata.Texture) {
	r.backend.TextureCreateWriteable(texture)
}

func (r *RendererSystem) TextureResize(texture *metadata.Texture, new_width, new_height uint32) {
	r.backend.TextureResize(texture, new_width, new_height)
}

func (r *RendererSystem) TextureWriteData(texture *metadata.Texture, offset, size uint32, pixels []uint8) {
	r.backend.TextureWriteData(texture, offset, size, pixels)
}

func (r *RendererSystem) CreateGeometry(geometry *metadata.Geometry, vertex_size, vertex_count uint32, vertices interface{}, index_size uint32, index_count uint32, indices []uint32) bool {
	return r.backend.CreateGeometry(geometry, vertex_size, vertex_count, vertices, index_size, index_count, indices)
}

func (r *RendererSystem) DestroyGeometry(geometry *metadata.Geometry) {
	r.backend.DestroyGeometry(geometry)
}

func (r *RendererSystem) DrawGeometry(data *metadata.GeometryRenderData) {
	r.backend.DrawGeometry(data)
}

func (r *RendererSystem) RenderPassCreate(config *metadata.RenderPassConfig) (*metadata.RenderPass, error) {
	return r.backend.RenderPassCreate(config)
}

func (r *RendererSystem) RenderPassDestroy(pass *metadata.RenderPass) {
	r.backend.RenderPassDestroy(pass)
}

func (r *RendererSystem) RenderPassBegin(pass *metadata.RenderPass, target *metadata.RenderTarget) bool {
	return r.backend.RenderPassBegin(pass, target)
}

func (r *RendererSystem) RenderPassEnd(pass *metadata.RenderPass) bool {
	return r.backend.RenderPassEnd(pass)
}

func (r *RendererSystem) ShaderCreate(shader *metadata.Shader, config *metadata.ShaderConfig, pass *metadata.RenderPass, stage_count uint8, stage_filenames []string, stages []metadata.ShaderStage) bool {
	return r.backend.ShaderCreate(shader, config, pass, stage_count, stage_filenames, stages)
}

func (r *RendererSystem) ShaderDestroy(shader *metadata.Shader) {
	r.backend.ShaderDestroy(shader)
}

func (r *RendererSystem) ShaderInitialize(shader *metadata.Shader) error {
	return r.backend.ShaderInitialize(shader)
}

func (r *RendererSystem) ShaderUse(shader *metadata.Shader) bool {
	return r.backend.ShaderUse(shader)
}

func (r *RendererSystem) ShaderBindGlobals(shader *metadata.Shader) bool {
	return r.backend.ShaderBindGlobals(shader)
}

func (r *RendererSystem) ShaderBindInstance(shader *metadata.Shader, instance_id uint32) bool {
	return r.backend.ShaderBindInstance(shader, instance_id)
}

func (r *RendererSystem) ShaderApplyGlobals(shader *metadata.Shader) bool {
	return r.backend.ShaderApplyGlobals(shader)
}

func (r *RendererSystem) ShaderApplyInstance(shader *metadata.Shader, needs_update bool) bool {
	return r.backend.ShaderApplyInstance(shader, needs_update)
}

func (r *RendererSystem) ShaderAcquireInstanceResources(shader *metadata.Shader, maps []*metadata.TextureMap) (uint32, error) {
	return r.backend.ShaderAcquireInstanceResources(shader, maps)
}

func (r *RendererSystem) ShaderReleaseInstanceResources(shader *metadata.Shader, instance_id uint32) bool {
	return r.backend.ShaderReleaseInstanceResources(shader, instance_id)
}

func (r *RendererSystem) ShaderSetUniform(shader *metadata.Shader, uniform metadata.ShaderUniform, value interface{}) bool {
	return r.backend.SetUniform(shader, uniform, value)
}

func (r *RendererSystem) TextureMapAcquireResources(texture_map *metadata.TextureMap) bool {
	return r.backend.TextureMapAcquireResources(texture_map)
}

func (r *RendererSystem) TextureMapReleaseResources(texture_map *metadata.TextureMap) {
	r.backend.TextureMapReleaseResources(texture_map)
}

func (r *RendererSystem) RenderTargetCreate(attachment_count uint8, attachments []*metadata.RenderTargetAttachment, pass *metadata.RenderPass, width, height uint32) (*metadata.RenderTarget, error) {
	return r.backend.RenderTargetCreate(attachment_count, attachments, pass, width, height)
}

func (r *RendererSystem) RenderTargetDestroy(target *metadata.RenderTarget, free_internal_memory bool) {
	r.backend.RenderTargetDestroy(target, free_internal_memory)
}

func (r *RendererSystem) IsMultithreaded() bool {
	return r.backend.IsMultithreaded()
}

func (r *RendererSystem) RenderBufferCreate(renderbufferType metadata.RenderBufferType, total_size uint64) (*metadata.RenderBuffer, error) {
	buffer := &metadata.RenderBuffer{
		RenderBufferType: renderbufferType,
		TotalSize:        total_size,
		Buffer:           make([]interface{}, total_size),
	}

	// Create the internal buffer from the backend.
	b, err := r.backend.RenderBufferCreateInternal(*buffer)
	if err != nil {
		err := fmt.Errorf("unable to create backing buffer for renderbuffer. Application cannot continue")
		return nil, err
	}

	return b, nil
}

func (r *RendererSystem) RenderBufferDestroy(buffer *metadata.RenderBuffer) {
	if buffer != nil {
		if len(buffer.Buffer) > 0 {
			buffer.Buffer = nil
		}
		// Free up the backend resources.
		r.backend.RenderBufferDestroyInternal(buffer)
		buffer.InternalData = nil
	}
}

func (r *RendererSystem) RenderBufferBind(buffer *metadata.RenderBuffer, offset uint64) error {
	if buffer == nil {
		return fmt.Errorf("buffer cannot be nil")
	}
	return r.backend.RenderBufferBind(buffer, offset)
}

func (r *RendererSystem) RenderBufferUnbind(buffer *metadata.RenderBuffer) bool {
	return r.backend.RenderBufferUnbind(buffer)
}

func (r *RendererSystem) RenderBufferMapMemory(buffer *metadata.RenderBuffer, offset, size uint64) interface{} {
	return r.backend.RenderBufferMapMemory(buffer, offset, size)
}

func (r *RendererSystem) RenderBufferUnmapMemory(buffer *metadata.RenderBuffer, offset, size uint64) {
	r.backend.RenderBufferUnmapMemory(buffer, offset, size)
}

func (r *RendererSystem) RenderBufferFlush(buffer *metadata.RenderBuffer, offset, size uint64) bool {
	return r.backend.RenderBufferFlush(buffer, offset, size)
}

func (r *RendererSystem) RenderBufferRead(buffer *metadata.RenderBuffer, offset, size uint64) (interface{}, error) {
	return r.backend.RenderBufferRead(buffer, offset, size)
}

func (r *RendererSystem) RenderBufferResize(buffer *metadata.RenderBuffer, new_total_size uint64) bool {
	// Sanity check.
	if new_total_size <= buffer.TotalSize {
		err := fmt.Errorf("func RenderBufferResize requires that new size be larger than the old. Not doing this could lead to data loss")
		core.LogError(err.Error())
		return false
	}

	if r.backend.RenderBufferResize(buffer, new_total_size) {
		buffer.TotalSize = new_total_size
		return true
	}

	core.LogError("Failed to resize internal renderbuffer resources.")
	return false
}

func (r *RendererSystem) RenderBufferAllocate(buffer *metadata.RenderBuffer, size uint64) {
	if buffer != nil {
		buffer.Buffer = make([]interface{}, size)
	}
}

func (r *RendererSystem) RenderBufferFree(buffer *metadata.RenderBuffer, size, offset uint64) bool {
	if buffer != nil {
		// Ensure offset and size are within bounds
		if offset+size > uint64(len(buffer.Buffer)) {
			size = uint64(len(buffer.Buffer)) - offset
		}
		// Set the specified range to nil
		for i := offset; i < offset+size; i++ {
			buffer.Buffer[i] = nil
		}
	}
	return true
}

func (r *RendererSystem) RenderBufferLoadRange(buffer *metadata.RenderBuffer, offset, size uint64, data interface{}) bool {
	return r.backend.RenderBufferLoadRange(buffer, offset, size, data)
}

func (r *RendererSystem) RenderBufferCopyRange(source *metadata.RenderBuffer, source_offset uint64, dest *metadata.RenderBuffer, dest_offset uint64, size uint64) bool {
	return r.backend.RenderBufferCopyRange(source, source_offset, dest, dest_offset, size)
}

func (r *RendererSystem) RenderBufferDraw(buffer *metadata.RenderBuffer, offset uint64, element_count uint32, bind_only bool) bool {
	return r.backend.RenderBufferDraw(buffer, offset, element_count, bind_only)
}

// func (r *RendererSystem) regenerateRenderTargets() {
// 	// Create render targets for each. TODO: Should be configurable.
// 	for i := uint8(0); i < r.WindowRenderTargetCount; i++ {
// 		// Destroy the old first if they exist.
// 		r.backend.RenderTargetDestroy(r.SkyboxRenderPass.Targets[i], false)
// 		r.backend.RenderTargetDestroy(r.WorldRenderPass.Targets[i], false)
// 		r.backend.RenderTargetDestroy(r.UIRenderPass.Targets[i], false)

// 		windowTargetTexture := r.backend.WindowAttachmentGet(i)
// 		depthTargetTexture := r.backend.DepthAttachmentGet()

// 		// Skybox render targets
// 		skyboxAttachments := []*metadata.Texture{windowTargetTexture}
// 		var err error
// 		r.SkyboxRenderPass.Targets[i], err = r.backend.RenderTargetCreate(
// 			1,
// 			skyboxAttachments,
// 			r.SkyboxRenderPass,
// 			r.FramebufferWidth,
// 			r.FramebufferHeight,
// 		)
// 		if err != nil {
// 			core.LogError(err.Error())
// 			return
// 		}

// 		// World render targets.
// 		attachments := []*metadata.Texture{windowTargetTexture, depthTargetTexture}
// 		r.WorldRenderPass.Targets[i], err = r.backend.RenderTargetCreate(
// 			2,
// 			attachments,
// 			r.WorldRenderPass,
// 			r.FramebufferWidth,
// 			r.FramebufferHeight,
// 		)
// 		if err != nil {
// 			core.LogError(err.Error())
// 			return
// 		}

// 		// UI render targets
// 		uiAttachments := []*metadata.Texture{windowTargetTexture}
// 		r.UIRenderPass.Targets[i], err = r.backend.RenderTargetCreate(
// 			1,
// 			uiAttachments,
// 			r.UIRenderPass,
// 			r.FramebufferWidth,
// 			r.FramebufferHeight,
// 		)
// 		if err != nil {
// 			core.LogError(err.Error())
// 			return
// 		}
// 	}
// }
