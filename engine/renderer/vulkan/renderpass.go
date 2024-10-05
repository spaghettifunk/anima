package vulkan

import (
	vk "github.com/goki/vulkan"
)

type VulkanRenderPassState int

const (
	READY VulkanRenderPassState = iota
	RECORDING
	IN_RENDER_PASS
	RECORDING_ENDED
	SUBMITTED
	NOT_ALLOCATED
)

type VulkanRenderpass struct {
	Handle     vk.RenderPass
	X, Y, W, H float32
	R, G, B, A float32

	Depth   float32
	Stencil uint32

	State VulkanRenderPassState
}
