package systems

import (
	"errors"
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

			if err := r.backend.Resized(width, height); err != nil {
				return err
			}

			renderViewSystem.OnWindowResize(width, height)

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
		if errors.Is(err, core.ErrSwapchainBooting) {
			core.LogInfo(err.Error())
			return nil
		}
		return err
	}

	attachmentIndex := r.backend.WindowAttachmentIndexGet()

	// Render each view.
	for i := 0; i < len(packet.ViewPackets); i++ {
		if err := renderViewSystem.OnRender(packet.ViewPackets[i], r.backend.FrameNumber, attachmentIndex); err != nil {
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

func (r *RendererSystem) TextureDestroy(texture *metadata.Texture) error {
	return r.backend.TextureDestroy(texture)
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

func (r *RendererSystem) CreateGeometry(geometry *metadata.Geometry, vertex_size, vertex_count uint32, vertices interface{}, index_size uint32, index_count uint32, indices []uint32) error {
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

func (r *RendererSystem) GetWindowAttachmentCount() uint8 {
	return r.backend.GetWindowAttachmentCount()
}

func (r *RendererSystem) RenderPassDestroy(pass *metadata.RenderPass, freeInternalMemory bool) error {
	// Destroy its rendertargets.
	for i := 0; i < int(pass.RenderTargetCount); i++ {
		if err := r.backend.RenderTargetDestroy(pass.Targets[i], freeInternalMemory); err != nil {
			return err
		}
	}
	return r.backend.RenderPassDestroy(pass)
}

func (r *RendererSystem) RenderPassBegin(pass *metadata.RenderPass, target *metadata.RenderTarget) error {
	return r.backend.RenderPassBegin(pass, target)
}

func (r *RendererSystem) RenderPassEnd(pass *metadata.RenderPass) error {
	return r.backend.RenderPassEnd(pass)
}

func (r *RendererSystem) ShaderCreate(shader *metadata.Shader, config *metadata.ShaderConfig, pass *metadata.RenderPass, stage_count uint8, stage_filenames []string, stages []metadata.ShaderStage) error {
	return r.backend.ShaderCreate(shader, config, pass, stage_count, stage_filenames, stages)
}

func (r *RendererSystem) ShaderDestroy(shader *metadata.Shader) {
	r.backend.ShaderDestroy(shader)
}

func (r *RendererSystem) ShaderInitialize(shader *metadata.Shader) error {
	return r.backend.ShaderInitialize(shader)
}

func (r *RendererSystem) ShaderUse(shader *metadata.Shader) error {
	return r.backend.ShaderUse(shader)
}

func (r *RendererSystem) ShaderBindGlobals(shader *metadata.Shader) error {
	return r.backend.ShaderBindGlobals(shader)
}

func (r *RendererSystem) ShaderBindInstance(shader *metadata.Shader, instance_id uint32) error {
	return r.backend.ShaderBindInstance(shader, instance_id)
}

func (r *RendererSystem) ShaderApplyGlobals(shader *metadata.Shader) error {
	return r.backend.ShaderApplyGlobals(shader)
}

func (r *RendererSystem) ShaderApplyInstance(shader *metadata.Shader, needs_update bool) error {
	return r.backend.ShaderApplyInstance(shader, needs_update)
}

func (r *RendererSystem) ShaderAcquireInstanceResources(shader *metadata.Shader, maps []*metadata.TextureMap) (uint32, error) {
	return r.backend.ShaderAcquireInstanceResources(shader, maps)
}

func (r *RendererSystem) ShaderReleaseInstanceResources(shader *metadata.Shader, instance_id uint32) error {
	return r.backend.ShaderReleaseInstanceResources(shader, instance_id)
}

func (r *RendererSystem) ShaderSetUniform(shader *metadata.Shader, uniform metadata.ShaderUniform, value interface{}) error {
	return r.backend.SetUniform(shader, uniform, value)
}

func (r *RendererSystem) TextureMapAcquireResources(texture_map *metadata.TextureMap) error {
	return r.backend.TextureMapAcquireResources(texture_map)
}

func (r *RendererSystem) TextureMapReleaseResources(texture_map *metadata.TextureMap) {
	r.backend.TextureMapReleaseResources(texture_map)
}

func (r *RendererSystem) RenderTargetCreate(attachment_count uint8, attachments []*metadata.RenderTargetAttachment, pass *metadata.RenderPass, width, height uint32) (*metadata.RenderTarget, error) {
	return r.backend.RenderTargetCreate(attachment_count, attachments, pass, width, height)
}

func (r *RendererSystem) RenderTargetDestroy(target *metadata.RenderTarget, freeInternalMemory bool) error {
	if err := r.backend.RenderTargetDestroy(target, freeInternalMemory); err != nil {
		return err
	}

	if freeInternalMemory {
		target = &metadata.RenderTarget{
			AttachmentCount:     0,
			Attachments:         []*metadata.RenderTargetAttachment{},
			InternalFramebuffer: nil,
		}
	}

	return nil
}

func (r *RendererSystem) IsMultithreaded() bool {
	return r.backend.IsMultithreaded()
}

func (r *RendererSystem) RenderBufferCreate(renderbufferType metadata.RenderBufferType, total_size uint64) (*metadata.RenderBuffer, error) {
	// Create the internal buffer from the backend.
	b, err := r.backend.RenderBufferCreate(renderbufferType, total_size)
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
		r.backend.RenderBufferDestroy(buffer)
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

func (r *RendererSystem) RenderBufferMapMemory(buffer *metadata.RenderBuffer, offset, size uint64) (interface{}, error) {
	return r.backend.RenderBufferMapMemory(buffer, offset, size)
}

func (r *RendererSystem) RenderBufferUnmapMemory(buffer *metadata.RenderBuffer, offset, size uint64) {
	r.backend.RenderBufferUnmapMemory(buffer, offset, size)
}

func (r *RendererSystem) RenderBufferFlush(buffer *metadata.RenderBuffer, offset, size uint64) error {
	return r.backend.RenderBufferFlush(buffer, offset, size)
}

func (r *RendererSystem) RenderBufferRead(buffer *metadata.RenderBuffer, offset, size uint64) (interface{}, error) {
	return r.backend.RenderBufferRead(buffer, offset, size)
}

func (r *RendererSystem) RenderBufferResize(buffer *metadata.RenderBuffer, new_total_size uint64) error {
	// Sanity check.
	if new_total_size <= buffer.TotalSize {
		err := fmt.Errorf("func RenderBufferResize requires that new size be larger than the old. Not doing this could lead to data loss")
		return err
	}

	if err := r.backend.RenderBufferResize(buffer, new_total_size); err != nil {
		buffer.TotalSize = new_total_size
		return err
	}

	core.LogError("Failed to resize internal renderbuffer resources.")
	return nil
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

func (r *RendererSystem) RenderBufferLoadRange(buffer *metadata.RenderBuffer, offset, size uint64, data interface{}) error {
	return r.backend.RenderBufferLoadRange(buffer, offset, size, data)
}

func (r *RendererSystem) RenderBufferCopyRange(source *metadata.RenderBuffer, source_offset uint64, dest *metadata.RenderBuffer, dest_offset uint64, size uint64) error {
	return r.backend.RenderBufferCopyRange(source, source_offset, dest, dest_offset, size)
}

func (r *RendererSystem) RenderBufferDraw(buffer *metadata.RenderBuffer, offset uint64, element_count uint32, bind_only bool) error {
	return r.backend.RenderBufferDraw(buffer, offset, element_count, bind_only)
}
