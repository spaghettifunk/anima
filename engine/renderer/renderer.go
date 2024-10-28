package renderer

import (
	"github.com/spaghettifunk/anima/engine/core"
	"github.com/spaghettifunk/anima/engine/platform"
	"github.com/spaghettifunk/anima/engine/renderer/metadata"
	"github.com/spaghettifunk/anima/engine/renderer/vulkan"
)

type RendererType uint8

const (
	Vulkan RendererType = iota
	DirectX
	Metal
	OpenGL
)

type Renderer struct {
	backend RendererBackend

	AppName   string
	AppWidth  uint32
	AppHeight uint32
	Platform  *platform.Platform
}

func NewRenderer(appName string, appWidth, appHeight uint32, platform *platform.Platform) (*Renderer, error) {
	renderer := &Renderer{
		backend:   vulkan.New(platform),
		AppName:   appName,
		AppWidth:  appWidth,
		AppHeight: appHeight,
	}
	return renderer, nil
}

func (r *Renderer) Initialize() error {
	return r.backend.Initialize(r.AppName, r.AppWidth, r.AppHeight)
}

func (r *Renderer) Shutdown() error {
	return r.backend.Shutdow()
}

func (r *Renderer) BeginFrame(deltaTime float64) error {
	return r.backend.BeginFrame(deltaTime)
}

func (r *Renderer) EndFrame(deltaTime float64) error {
	return r.backend.EndFrame(deltaTime)
}

func (r *Renderer) OnResize(width, height uint16) error {
	return r.backend.Resized(width, height)
}

func (r *Renderer) DrawFrame(renderPacket *metadata.RenderPacket) error {
	if err := r.BeginFrame(renderPacket.DeltaTime); err != nil {
		core.LogError(err.Error())
		return err
	}
	if err := r.EndFrame(renderPacket.DeltaTime); err != nil {
		core.LogError("RendererEndFrame failed. Application shutting down...")
		return err
	}
	return nil
}

func (r *Renderer) TextureCreate(pixels []uint8, texture *metadata.Texture) {}

func (r *Renderer) TextureDestroy(texture *metadata.Texture) {}

func (r *Renderer) TextureCreateWriteable(texture *metadata.Texture) {}

func (r *Renderer) TextureResize(texture *metadata.Texture, new_width, new_height uint32) {}

func (r *Renderer) TextureWriteData(texture *metadata.Texture, offset, size uint32, pixels []uint8) {}

func (r *Renderer) CreateGeometry(geometry *metadata.Geometry, vertex_size, vertex_count uint32, vertices interface{}, index_size uint32, index_count uint32, indices []uint32) bool {
	return r.backend.CreateGeometry(geometry, vertex_size, vertex_count, vertices, index_size, index_count, indices)
}

func (r *Renderer) DestroyGeometry(geometry *metadata.Geometry) {}

func (r *Renderer) DrawGeometry(data *metadata.GeometryRenderData) {}

func (r *Renderer) RenderPassCreate(depth float32, stencil uint32, has_prev_pass, has_next_pass bool) (*metadata.RenderPass, error) {
	return nil, nil
}

func (r *Renderer) RenderPassDestroy(pass *metadata.RenderPass) {}

func (r *Renderer) RenderPassBegin(pass *metadata.RenderPass, target *metadata.RenderTarget) bool {
	return false
}

func (r *Renderer) RenderPassEnd(pass *metadata.RenderPass) bool { return false }

func (r *Renderer) RenderPassGet(name string) *metadata.RenderPass { return nil }

func (r *Renderer) ShaderCreate(shader *metadata.Shader, config *metadata.ShaderConfig, pass *metadata.RenderPass, stage_count uint8, stage_filenames []string, stages []metadata.ShaderStage) bool {
	return false
}

func (r *Renderer) ShaderDestroy(shader *metadata.Shader) {}

func (r *Renderer) ShaderInitialize(shader *metadata.Shader) bool { return false }

func (r *Renderer) ShaderUse(shader *metadata.Shader) bool { return false }

func (r *Renderer) ShaderBindGlobals(shader *metadata.Shader) bool { return false }

func (r *Renderer) ShaderBindInstance(shader *metadata.Shader, instance_id uint32) bool { return false }

func (r *Renderer) ShaderApplyGlobals(shader *metadata.Shader) bool { return false }

func (r *Renderer) ShaderApplyInstance(shader *metadata.Shader, needs_update bool) bool { return false }

func (r *Renderer) ShaderAcquireInstanceResources(shader *metadata.Shader, maps []*metadata.TextureMap) (out_instance_id uint32) {
	return 0
}

func (r *Renderer) ShaderReleaseInstanceResources(shader *metadata.Shader, instance_id uint32) bool {
	return false
}

func (r *Renderer) SetUniform(shader *metadata.Shader, uniform metadata.ShaderUniform, value interface{}) bool {
	return false
}

func (r *Renderer) TextureMapAcquireResources(texture_map *metadata.TextureMap) bool { return false }

func (r *Renderer) TextureMapReleaseResources(texture_map *metadata.TextureMap) {}

func (r *Renderer) RenderTargetCreate(attachment_count uint8, attachments []*metadata.Texture, pass *metadata.RenderPass, width, height uint32) (out_target *metadata.RenderTarget) {
	return nil
}

func (r *Renderer) RenderTargetDestroy(target *metadata.RenderTarget) {}

func (r *Renderer) IsMultithreaded() bool { return false }

func (r *Renderer) RenderBufferCreate(renderbufferType metadata.RenderBufferType, total_size uint64, use_freelist bool) *metadata.RenderBuffer {
	return nil
}

func (r *Renderer) RenderBufferDestroy(buffer *metadata.RenderBuffer) {}

func (r *Renderer) RenderBufferBind(buffer *metadata.RenderBuffer, offset uint64) bool { return false }

func (r *Renderer) RenderBufferUnbind(buffer *metadata.RenderBuffer) bool { return false }

func (r *Renderer) RenderBufferMapMemory(buffer *metadata.RenderBuffer, offset, size uint64) interface{} {
	return nil
}

func (r *Renderer) RenderBufferUnmapMemory(buffer *metadata.RenderBuffer, offset, size uint64) {}

func (r *Renderer) RenderBufferFlush(buffer *metadata.RenderBuffer, offset, size uint64) bool {
	return false
}

func (r *Renderer) RenderBufferRead(buffer *metadata.RenderBuffer, offset, size uint64) (out_memory []interface{}) {
	return nil
}

func (r *Renderer) RenderBufferResize(buffer *metadata.RenderBuffer, new_total_size uint64) bool {
	return false
}

func (r *Renderer) RenderBufferAllocate(buffer *metadata.RenderBuffer, size uint64) (out_offset uint64) {
	return 0
}

func (r *Renderer) RenderBufferFree(buffer *metadata.RenderBuffer, size, offset uint64) bool {
	return false
}

func (r *Renderer) RenderBufferLoadRange(buffer *metadata.RenderBuffer, offset, size uint64, data interface{}) bool {
	return false
}

func (r *Renderer) RenderBufferCopyRange(source *metadata.RenderBuffer, source_offset uint64, dest *metadata.RenderBuffer, dest_offset uint64, size uint64) bool {
	return false
}

func (r *Renderer) RenderBufferDraw(buffer *metadata.RenderBuffer, offset uint64, element_count uint32, bind_only bool) bool {
	return false
}
