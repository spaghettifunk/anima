package systems

import (
	"fmt"

	"github.com/spaghettifunk/anima/engine/assets"
	"github.com/spaghettifunk/anima/engine/core"
	"github.com/spaghettifunk/anima/engine/math"
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

	// The number of render targets. Typically lines up with the amount of swapchain images.
	WindowRenderTargetCount uint8
	// The current window framebuffer width.
	FramebufferWidth uint32
	// The current window framebuffer height.
	FramebufferHeight uint32

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

func (r *RendererSystem) Initialize(shaderSystem *ShaderSystem, renderViewSystem *RenderViewSystem) error {
	// Default framebuffer size. Overridden when window is created.
	r.FramebufferWidth = 1280
	r.FramebufferHeight = 720
	r.Resizing = false
	r.FramesSinceResize = 0
	r.backend.FrameNumber = 0

	rbc := &metadata.RendererBackendConfig{
		ApplicationName: r.AppName,
	}

	if err := r.backend.Initialize(rbc, &r.WindowRenderTargetCount); err != nil {
		return err
	}

	// Load render views

	// Skybox view
	skybox_config := &metadata.RenderViewConfig{
		RenderViewType:   metadata.RENDERER_VIEW_KNOWN_TYPE_SKYBOX,
		Width:            uint16(r.FramebufferWidth),
		Height:           uint16(r.FramebufferHeight),
		Name:             "skybox",
		ViewMatrixSource: metadata.RENDER_VIEW_VIEW_MATRIX_SOURCE_SCENE_CAMERA,
		PassCount:        1,
		Passes: []*metadata.RenderPassConfig{
			{
				Name:        "Renderpass.Builtin.Skybox",
				RenderArea:  math.NewVec4(0, 0, 1280, 720), // Default render area resolution
				ClearColour: math.NewVec4(0.0, 0.0, 0.2, 1.0),
				ClearFlags:  metadata.RENDERPASS_CLEAR_COLOUR_BUFFER_FLAG,
				Depth:       1.0,
				Stencil:     0,
				Target: &metadata.RenderTargetConfig{
					Attachments: []*metadata.RenderTargetAttachmentConfig{
						{
							RenderTargetAttachmentType: metadata.RENDER_TARGET_ATTACHMENT_TYPE_COLOUR,
							Source:                     metadata.RENDER_TARGET_ATTACHMENT_SOURCE_DEFAULT,
							LoadOperation:              metadata.RENDER_TARGET_ATTACHMENT_LOAD_OPERATION_DONT_CARE,
							StoreOperation:             metadata.RENDER_TARGET_ATTACHMENT_STORE_OPERATION_STORE,
							PresentAfter:               false,
						},
					},
				},
				RenderTargetCount: r.backend.GetWindowAttachmentCount(),
			},
		},
	}

	if err := renderViewSystem.Create(skybox_config); err != nil {
		core.LogError("Failed to create skybox view. Aborting application.")
		return err
	}

	// World view.
	world_view_config := &metadata.RenderViewConfig{
		RenderViewType:   metadata.RENDERER_VIEW_KNOWN_TYPE_WORLD,
		Width:            uint16(r.FramebufferWidth),
		Height:           uint16(r.FramebufferHeight),
		Name:             "world",
		ViewMatrixSource: metadata.RENDER_VIEW_VIEW_MATRIX_SOURCE_SCENE_CAMERA,
		PassCount:        1,
		Passes: []*metadata.RenderPassConfig{
			{
				Name:        "Renderpass.Builtin.World",
				RenderArea:  math.NewVec4(0, 0, 1280, 720), // Default render area resolution
				ClearColour: math.NewVec4(0.0, 0.0, 0.2, 1.0),
				ClearFlags:  metadata.RENDERPASS_CLEAR_DEPTH_BUFFER_FLAG | metadata.RENDERPASS_CLEAR_STENCIL_BUFFER_FLAG,
				Depth:       1.0,
				Stencil:     0,
				Target: &metadata.RenderTargetConfig{
					Attachments: []*metadata.RenderTargetAttachmentConfig{
						// Colour attachment
						{
							RenderTargetAttachmentType: metadata.RENDER_TARGET_ATTACHMENT_TYPE_COLOUR,
							Source:                     metadata.RENDER_TARGET_ATTACHMENT_SOURCE_DEFAULT,
							LoadOperation:              metadata.RENDER_TARGET_ATTACHMENT_LOAD_OPERATION_LOAD,
							StoreOperation:             metadata.RENDER_TARGET_ATTACHMENT_STORE_OPERATION_STORE,
							PresentAfter:               false,
						},
						{ // Depth attachment
							RenderTargetAttachmentType: metadata.RENDER_TARGET_ATTACHMENT_TYPE_DEPTH,
							Source:                     metadata.RENDER_TARGET_ATTACHMENT_SOURCE_DEFAULT,
							LoadOperation:              metadata.RENDER_TARGET_ATTACHMENT_LOAD_OPERATION_DONT_CARE,
							StoreOperation:             metadata.RENDER_TARGET_ATTACHMENT_STORE_OPERATION_STORE,
							PresentAfter:               false,
						},
					},
				},
				RenderTargetCount: r.backend.GetWindowAttachmentCount(),
			},
		},
	}

	if err := renderViewSystem.Create(world_view_config); err != nil {
		core.LogError("Failed to create world view. Aborting application.")
		return err
	}

	// UI view
	ui_view_config := &metadata.RenderViewConfig{
		RenderViewType:   metadata.RENDERER_VIEW_KNOWN_TYPE_UI,
		Width:            0,
		Height:           0,
		Name:             "ui",
		ViewMatrixSource: metadata.RENDER_VIEW_VIEW_MATRIX_SOURCE_SCENE_CAMERA,
		PassCount:        1,
		Passes: []*metadata.RenderPassConfig{
			{
				Name:        "Renderpass.Builtin.UI",
				RenderArea:  math.NewVec4(0, 0, 1280, 720),
				ClearColour: math.NewVec4(0.0, 0.0, 0.2, 1.0),
				ClearFlags:  metadata.RENDERPASS_CLEAR_NONE_FLAG,
				Depth:       1.0,
				Stencil:     0,
				Target: &metadata.RenderTargetConfig{
					Attachments: []*metadata.RenderTargetAttachmentConfig{
						{
							// Colour attachment.
							RenderTargetAttachmentType: metadata.RENDER_TARGET_ATTACHMENT_TYPE_COLOUR,
							Source:                     metadata.RENDER_TARGET_ATTACHMENT_SOURCE_DEFAULT,
							LoadOperation:              metadata.RENDER_TARGET_ATTACHMENT_LOAD_OPERATION_LOAD,
							StoreOperation:             metadata.RENDER_TARGET_ATTACHMENT_STORE_OPERATION_STORE,
							PresentAfter:               true,
						},
					},
				},
				RenderTargetCount: r.backend.GetWindowAttachmentCount(),
			},
		},
	}

	if err := renderViewSystem.Create(ui_view_config); err != nil {
		core.LogError("failed to create UI view. Aborting application")
		return err
	}

	// Pick pass.
	pick_view_config := &metadata.RenderViewConfig{
		RenderViewType:   metadata.RENDERER_VIEW_KNOWN_TYPE_PICK,
		Width:            uint16(r.FramebufferWidth),
		Height:           uint16(r.FramebufferHeight),
		Name:             "pick",
		ViewMatrixSource: metadata.RENDER_VIEW_VIEW_MATRIX_SOURCE_SCENE_CAMERA,
		PassCount:        2,
		Passes: []*metadata.RenderPassConfig{
			{
				// World pass
				Name:        "Renderpass.Builtin.WorldPick",
				RenderArea:  math.NewVec4(0, 0, 1280, 720),
				ClearColour: math.NewVec4(1.0, 1.0, 1.0, 1.0), // HACK: clearing to white for better visibility// TODO: Clear to black, as 0 is invalid id,
				ClearFlags:  metadata.RENDERPASS_CLEAR_COLOUR_BUFFER_FLAG | metadata.RENDERPASS_CLEAR_DEPTH_BUFFER_FLAG,
				Depth:       1.0,
				Stencil:     0,
				Target: &metadata.RenderTargetConfig{
					Attachments: []*metadata.RenderTargetAttachmentConfig{
						{
							RenderTargetAttachmentType: metadata.RENDER_TARGET_ATTACHMENT_TYPE_COLOUR,
							Source:                     metadata.RENDER_TARGET_ATTACHMENT_SOURCE_VIEW, // Obtain the attachment from the view,
							LoadOperation:              metadata.RENDER_TARGET_ATTACHMENT_LOAD_OPERATION_DONT_CARE,
							StoreOperation:             metadata.RENDER_TARGET_ATTACHMENT_STORE_OPERATION_STORE,
							PresentAfter:               false,
						},
						{
							RenderTargetAttachmentType: metadata.RENDER_TARGET_ATTACHMENT_TYPE_DEPTH,
							Source:                     metadata.RENDER_TARGET_ATTACHMENT_SOURCE_VIEW, // Obtain the attachment from the view,
							LoadOperation:              metadata.RENDER_TARGET_ATTACHMENT_LOAD_OPERATION_DONT_CARE,
							StoreOperation:             metadata.RENDER_TARGET_ATTACHMENT_STORE_OPERATION_STORE,
							PresentAfter:               false,
						},
					},
				},
				RenderTargetCount: 1,
			},
			{
				Name:        "Renderpass.Builtin.UIPick",
				RenderArea:  math.NewVec4(0, 0, 1280, 720),
				ClearColour: math.NewVec4(1.0, 1.0, 1.0, 1.0),
				ClearFlags:  metadata.RENDERPASS_CLEAR_NONE_FLAG,
				Depth:       1.0,
				Stencil:     0,
				Target: &metadata.RenderTargetConfig{
					Attachments: []*metadata.RenderTargetAttachmentConfig{
						{
							RenderTargetAttachmentType: metadata.RENDER_TARGET_ATTACHMENT_TYPE_COLOUR,
							// Obtain the attachment from the view.
							Source:        metadata.RENDER_TARGET_ATTACHMENT_SOURCE_VIEW,
							LoadOperation: metadata.RENDER_TARGET_ATTACHMENT_LOAD_OPERATION_LOAD,
							// Need to store it so it can be sampled afterward.
							StoreOperation: metadata.RENDER_TARGET_ATTACHMENT_STORE_OPERATION_STORE,
							PresentAfter:   false,
						},
					},
				},
				RenderTargetCount: 1,
			},
		},
	}

	if err := renderViewSystem.Create(pick_view_config); err != nil {
		core.LogError("Failed to create pick view. Aborting application.")
		return err
	}

	return nil
}

func (r *RendererSystem) Shutdown() error {
	return r.backend.Shutdow()
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
			r.Platform.Sleep(16)
			return nil
		}
	}

	// If the begin frame returned successfully, mid-frame operations may continue.
	if err := r.backend.BeginFrame(packet.DeltaTime); err != nil {
		return err
	}

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
		core.LogError("backend func EndFrame failed. Application shutting down")
		return err
	}
	return nil
}

func (r *RendererSystem) TextureCreate(pixels []uint8, texture *metadata.Texture) {
	r.backend.TextureCreate(pixels, texture)
}

func (r *RendererSystem) TextureDestroy(texture *metadata.Texture) {
	r.backend.TextureDestroy(texture)
}

func (r *RendererSystem) TextureCreateWriteable(texture *metadata.Texture) error {
	return r.backend.TextureCreateWriteable(texture)
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

func (r *RendererSystem) RenderPassCreate(config *metadata.RenderPassConfig, pass *metadata.RenderPass) (*metadata.RenderPass, error) {
	return r.backend.RenderPassCreate(config, pass)
}

func (r *RendererSystem) RenderPassDestroy(pass *metadata.RenderPass) {
	// Destroy its rendertargets.
	for i := 0; i < int(pass.RenderTargetCount); i++ {
		r.backend.RenderTargetDestroy(pass.Targets[i], true)
	}
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
