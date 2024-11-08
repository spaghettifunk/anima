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
		PassConfigs: []metadata.RenderPassConfig{
			{
				Name:        SkyboxRenderPassName,
				PrevName:    "",
				NextName:    WorldRenderPassName,
				RenderArea:  math.NewVec4Create(0, 0, 1280, 720),
				ClearColour: math.NewVec4Create(0.0, 0.0, 0.2, 1.0),
				ClearFlags:  metadata.RENDERPASS_CLEAR_COLOUR_BUFFER_FLAG,
			},
			{
				Name:        WorldRenderPassName,
				PrevName:    SkyboxRenderPassName,
				NextName:    UIRenderPassName,
				RenderArea:  math.NewVec4Create(0, 0, 1280, 720),
				ClearColour: math.NewVec4Create(0.0, 0.0, 0.2, 1.0),
				ClearFlags:  metadata.RENDERPASS_CLEAR_DEPTH_BUFFER_FLAG | metadata.RENDERPASS_CLEAR_STENCIL_BUFFER_FLAG,
			},
			{
				Name:        UIRenderPassName,
				PrevName:    WorldRenderPassName,
				NextName:    "",
				RenderArea:  math.NewVec4Create(0, 0, 1280, 720),
				ClearColour: math.NewVec4Create(0.0, 0.0, 0.2, 1.0),
				ClearFlags:  metadata.RENDERPASS_CLEAR_NONE_FLAG,
			},
		},
		OnRenderTargetRefreshRequired: r.regenerateRenderTargets,
	}

	if err := r.backend.Initialize(rbc, &r.WindowRenderTargetCount); err != nil {
		return err
	}

	// TODO: Will know how to get these when we define views.
	r.SkyboxRenderPass = r.backend.RenderPassGet(SkyboxRenderPassName)
	r.SkyboxRenderPass.RenderTargetCount = r.WindowRenderTargetCount
	r.SkyboxRenderPass.Targets = make([]*metadata.RenderTarget, r.WindowRenderTargetCount)

	r.WorldRenderPass = r.backend.RenderPassGet(WorldRenderPassName)
	r.WorldRenderPass.RenderTargetCount = r.WindowRenderTargetCount
	r.WorldRenderPass.Targets = make([]*metadata.RenderTarget, r.WindowRenderTargetCount)

	r.UIRenderPass = r.backend.RenderPassGet(UIRenderPassName)
	r.UIRenderPass.RenderTargetCount = r.WindowRenderTargetCount
	r.UIRenderPass.Targets = make([]*metadata.RenderTarget, r.WindowRenderTargetCount)

	r.regenerateRenderTargets()

	// Update the skybox renderpass dimensions.
	r.SkyboxRenderPass.RenderArea.X = 0
	r.SkyboxRenderPass.RenderArea.Y = 0
	r.SkyboxRenderPass.RenderArea.Z = float32(r.FramebufferWidth)
	r.SkyboxRenderPass.RenderArea.W = float32(r.FramebufferHeight)

	// Update the main/world renderpass dimensions.
	r.WorldRenderPass.RenderArea.X = 0
	r.WorldRenderPass.RenderArea.Y = 0
	r.WorldRenderPass.RenderArea.Z = float32(r.FramebufferWidth)
	r.WorldRenderPass.RenderArea.W = float32(r.FramebufferHeight)

	// Also update the UI renderpass dimensions.
	r.UIRenderPass.RenderArea.X = 0
	r.UIRenderPass.RenderArea.Y = 0
	r.UIRenderPass.RenderArea.Z = float32(r.FramebufferWidth)
	r.UIRenderPass.RenderArea.W = float32(r.FramebufferHeight)

	// Shaders

	// Builtin skybox shader.
	config_resource, err := r.assetManager.LoadAsset(metadata.BUILTIN_SHADER_NAME_SKYBOX, metadata.ResourceTypeShader, nil)
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
	r.SkyboxShaderID = shaderSystem.GetShaderID(metadata.BUILTIN_SHADER_NAME_SKYBOX)

	if r.SkyboxShaderID != skyboxShader.ID {
		err := fmt.Errorf("why are the IDs different?")
		core.LogError(err.Error())
		return err
	}

	// Builtin material shader.
	config_resource, err = r.assetManager.LoadAsset(metadata.BUILTIN_SHADER_NAME_MATERIAL, metadata.ResourceTypeShader, nil)
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
	r.MaterialShaderID = shaderSystem.GetShaderID(metadata.BUILTIN_SHADER_NAME_MATERIAL)

	if r.MaterialShaderID != materialShader.ID {
		err := fmt.Errorf("why are the IDs different?")
		core.LogError(err.Error())
		return err
	}

	// Builtin UI shader.
	config_resource, err = r.assetManager.LoadAsset(metadata.BUILTIN_SHADER_NAME_UI, metadata.ResourceTypeShader, nil)
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
	r.UIShaderID = shaderSystem.GetShaderID(metadata.BUILTIN_SHADER_NAME_UI)

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
			if !renderViewSystem.OnRender(packet.Views[i].View, packet.Views[i], r.backend.FrameNumber, attachmentIndex) {
				err := fmt.Errorf("error rendering view index %d", i)
				core.LogError(err.Error())
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

func (r *RendererSystem) RenderPassCreate(depth float32, stencil uint32, has_prev_pass, has_next_pass bool) (*metadata.RenderPass, error) {
	return r.backend.RenderPassCreate(depth, stencil, has_prev_pass, has_next_pass)
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

func (r *RendererSystem) RenderPassGet(name string) *metadata.RenderPass {
	return r.backend.RenderPassGet(name)
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

func (r *RendererSystem) RenderTargetCreate(attachment_count uint8, attachments []*metadata.Texture, pass *metadata.RenderPass, width, height uint32) (*metadata.RenderTarget, error) {
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

func (r *RendererSystem) RenderBufferRead(buffer *metadata.RenderBuffer, offset, size uint64) ([]interface{}, error) {
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

func (r *RendererSystem) regenerateRenderTargets() {
	// Create render targets for each. TODO: Should be configurable.
	for i := uint8(0); i < r.WindowRenderTargetCount; i++ {
		// Destroy the old first if they exist.
		r.backend.RenderTargetDestroy(r.SkyboxRenderPass.Targets[i], false)
		r.backend.RenderTargetDestroy(r.WorldRenderPass.Targets[i], false)
		r.backend.RenderTargetDestroy(r.UIRenderPass.Targets[i], false)

		windowTargetTexture := r.backend.WindowAttachmentGet(i)
		depthTargetTexture := r.backend.DepthAttachmentGet()

		// Skybox render targets
		skyboxAttachments := []*metadata.Texture{windowTargetTexture}
		var err error
		r.SkyboxRenderPass.Targets[i], err = r.backend.RenderTargetCreate(
			1,
			skyboxAttachments,
			r.SkyboxRenderPass,
			r.FramebufferWidth,
			r.FramebufferHeight,
		)
		if err != nil {
			core.LogError(err.Error())
			return
		}

		// World render targets.
		attachments := []*metadata.Texture{windowTargetTexture, depthTargetTexture}
		r.WorldRenderPass.Targets[i], err = r.backend.RenderTargetCreate(
			2,
			attachments,
			r.WorldRenderPass,
			r.FramebufferWidth,
			r.FramebufferHeight,
		)
		if err != nil {
			core.LogError(err.Error())
			return
		}

		// UI render targets
		uiAttachments := []*metadata.Texture{windowTargetTexture}
		r.UIRenderPass.Targets[i], err = r.backend.RenderTargetCreate(
			1,
			uiAttachments,
			r.UIRenderPass,
			r.FramebufferWidth,
			r.FramebufferHeight,
		)
		if err != nil {
			core.LogError(err.Error())
			return
		}
	}
}
