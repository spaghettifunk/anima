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

type VulkanRenderPass struct {
	Handle  vk.RenderPass
	Depth   float32
	Stencil uint32
	State   VulkanRenderPassState
}
