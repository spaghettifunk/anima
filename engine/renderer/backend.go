package renderer

import "github.com/spaghettifunk/anima/engine/renderer/metadata"

type RendererBackend interface {
	Initialize(appName string, appWidth, appHeight uint32) error
	Shutdow() error
	Resized(width, height uint16) error
	BeginFrame(deltaTime float64) error
	EndFrame(deltaTime float64) error
	TextureCreate(pixels []uint8, texture *metadata.Texture)
	TextureDestroy(texture *metadata.Texture)
	TextureCreateWriteable(texture *metadata.Texture)
	TextureResize(texture *metadata.Texture, new_width, new_height uint32)
	TextureWriteData(texture *metadata.Texture, offset, size uint32, pixels []uint8)
	CreateGeometry(geometry *metadata.Geometry, vertex_size, vertex_count uint32, vertices interface{}, index_size uint32, index_count uint32, indices []uint32) bool
	DestroyGeometry(geometry *metadata.Geometry)
	DrawGeometry(data *metadata.GeometryRenderData)
	RenderPassCreate(depth float32, stencil uint32, has_prev_pass, has_next_pass bool) (*metadata.RenderPass, error)
	RenderpassDestroy(pass *metadata.RenderPass)
	RenderPassBegin(pass *metadata.RenderPass, target *metadata.RenderTarget) bool
	RenderPassEnd(pass *metadata.RenderPass) bool
	RenderPassGet(name string) *metadata.RenderPass
	ShaderCreate(shader *metadata.Shader, config *metadata.ShaderConfig, pass *metadata.RenderPass, stage_count uint8, stage_filenames []string, stages []metadata.ShaderStage) bool
	ShaderDestroy(shader *metadata.Shader)
	ShaderInitialize(shader *metadata.Shader) bool
	ShaderUse(shader *metadata.Shader) bool
	ShaderBindGlobals(shader *metadata.Shader) bool
	ShaderBindInstance(shader *metadata.Shader, instance_id uint32) bool
	ShaderApplyGlobals(shader *metadata.Shader) bool
	ShaderApplyInstance(shader *metadata.Shader, needs_update bool) bool
	ShaderAcquireInstanceResources(shader *metadata.Shader, maps []*metadata.TextureMap) (out_instance_id uint32)
	ShaderReleaseInstanceResources(shader *metadata.Shader, instance_id uint32) bool
	SetUniform(shader *metadata.Shader, uniform metadata.ShaderUniformType, value interface{}) bool
	TextureMapAcquireResources(texture_map *metadata.TextureMap) bool
	TextureMapReleaseResources(texture_map *metadata.TextureMap)
	RenderTargetCreate(attachment_count uint8, attachments []*metadata.Texture, pass *metadata.RenderPass, width, height uint32) (out_target *metadata.RenderTarget)
	RenderTargetDestroy(target *metadata.RenderTarget)
	IsMultithreaded() bool
	RenderBufferCreate(renderbufferType metadata.RenderBufferType, total_size uint64, use_freelist bool) *metadata.RenderBuffer
	RenderBufferDestroy(buffer *metadata.RenderBuffer)
	RenderBufferBind(buffer *metadata.RenderBuffer, offset uint64) bool
	RenderBufferUnbind(buffer *metadata.RenderBuffer) bool
	RenderBufferMapMemory(buffer *metadata.RenderBuffer, offset, size uint64) interface{}
	RenderBufferUnmapMemory(buffer *metadata.RenderBuffer, offset, size uint64)
	RenderBufferFlush(buffer *metadata.RenderBuffer, offset, size uint64) bool
	RenderBufferRead(buffer *metadata.RenderBuffer, offset, size uint64) (out_memory []interface{})
	RenderBufferResize(buffer *metadata.RenderBuffer, new_total_size uint64) bool
	RenderBufferAllocate(buffer *metadata.RenderBuffer, size uint64) (out_offset uint64)
	RenderBufferFree(buffer *metadata.RenderBuffer, size, offset uint64) bool
	RenderBufferLoadRange(buffer *metadata.RenderBuffer, offset, size uint64, data interface{}) bool
	RenderBufferCopyRange(source *metadata.RenderBuffer, source_offset uint64, dest *metadata.RenderBuffer, dest_offset uint64, size uint64) bool
	RenderBufferDraw(buffer *metadata.RenderBuffer, offset uint64, element_count uint32, bind_only bool) bool
}
