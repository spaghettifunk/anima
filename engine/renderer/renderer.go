package renderer

import (
	"sync"

	"github.com/spaghettifunk/anima/engine/core"
	"github.com/spaghettifunk/anima/engine/platform"
	"github.com/spaghettifunk/anima/engine/renderer/metadata"
	"github.com/spaghettifunk/anima/engine/renderer/vulkan"
	"github.com/spaghettifunk/anima/engine/resources"
)

type RendererBackend interface {
	Initialize(appName string, appWidth, appHeight uint32) error
	Shutdow() error
	Resized(width, height uint16) error
	BeginFrame(deltaTime float64) error
	EndFrame(deltaTime float64) error
	TextureCreate(pixels []uint8, texture *resources.Texture)
	TextureDestroy(texture *resources.Texture)
	TextureCreateWriteable(texture *resources.Texture)
	TextureResize(texture *resources.Texture, new_width, new_height uint32)
	TextureWriteData(texture *resources.Texture, offset, size uint32, pixels []uint8)
	CreateGeometry(geometry *resources.Geometry, vertex_size, vertex_count uint32, vertices interface{}, index_size uint32, index_count uint32, indices []uint32) bool
	DestroyGeometry(geometry *resources.Geometry)
	DrawGeometry(data *metadata.GeometryRenderData)
	RenderPassCreate(depth float32, stencil uint32, has_prev_pass, has_next_pass bool) (*metadata.RenderPass, error)
	RenderpassDestroy(pass *metadata.RenderPass)
	RenderPassBegin(pass *metadata.RenderPass, target *metadata.RenderTarget) bool
	RenderPassEnd(pass *metadata.RenderPass) bool
	RenderPassGet(name string) *metadata.RenderPass
	ShaderCreate(shader *metadata.Shader, config *resources.ShaderConfig, pass *metadata.RenderPass, stage_count uint8, stage_filenames []string, stages []resources.ShaderStage) bool
	ShaderDestroy(shader *metadata.Shader)
	ShaderInitialize(shader *metadata.Shader) bool
	ShaderUse(shader *metadata.Shader) bool
	ShaderBindGlobals(shader *metadata.Shader) bool
	ShaderBindInstance(shader *metadata.Shader, instance_id uint32) bool
	ShaderApplyGlobals(shader *metadata.Shader) bool
	ShaderApplyInstance(shader *metadata.Shader, needs_update bool) bool
	ShaderAcquireInstanceResources(shader *metadata.Shader, maps []*resources.TextureMap) (out_instance_id uint32)
	ShaderReleaseInstanceResources(shader *metadata.Shader, instance_id uint32) bool
	SetUniform(shader *metadata.Shader, uniform resources.ShaderUniformType, value interface{}) bool
	TextureMapAcquireResources(texture_map *resources.TextureMap) bool
	TextureMapReleaseResources(texture_map *resources.TextureMap)
	RenderTargetCreate(attachment_count uint8, attachments []*resources.Texture, pass *metadata.RenderPass, width, height uint32) (out_target *metadata.RenderTarget)
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

type RendererType uint8

const (
	Vulkan RendererType = iota
	DirectX
	Metal
	OpenGL
)

type Renderer struct {
	backend RendererBackend
}

var initRenderer sync.Once
var renderer *Renderer

func Initialize(appName string, appWidth, appHeight uint32, platform *platform.Platform) error {
	initRenderer.Do(func() {
		renderer = &Renderer{
			backend: vulkan.New(platform),
		}
	})
	return renderer.backend.Initialize(appName, appWidth, appHeight)
}

func Shutdown() error {
	return renderer.backend.Shutdow()
}

func BeginFrame(deltaTime float64) error {
	return renderer.backend.BeginFrame(deltaTime)
}

func EndFrame(deltaTime float64) error {
	return renderer.backend.EndFrame(deltaTime)
}

func OnResize(width, height uint16) error {
	return renderer.backend.Resized(width, height)
}

func DrawFrame(renderPacket *metadata.RenderPacket) error {
	if err := BeginFrame(renderPacket.DeltaTime); err != nil {
		core.LogError(err.Error())
		return err
	}
	if err := EndFrame(renderPacket.DeltaTime); err != nil {
		core.LogError("RendererEndFrame failed. Application shutting down...")
		return err
	}
	return nil
}

func CreateGeometry(geometry *resources.Geometry, vertex_size, vertex_count uint32, vertices interface{}, index_size uint32, index_count uint32, indices []uint32) bool {
	return renderer.backend.CreateGeometry(geometry, vertex_size, vertex_count, vertices, index_size, index_count, indices)
}
