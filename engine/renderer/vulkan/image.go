package vulkan

import (
	vk "github.com/goki/vulkan"
)

type VulkanImage struct {
	Handle vk.Image
	Memory vk.DeviceMemory
	View   vk.ImageView
	Width  uint32
	Height uint32
}
