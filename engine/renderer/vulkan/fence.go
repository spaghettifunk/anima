package vulkan

import (
	vk "github.com/goki/vulkan"
)

type VulkanFence struct {
	Handle     vk.Fence
	IsSignaled bool
}
