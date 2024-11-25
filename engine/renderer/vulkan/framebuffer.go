package vulkan

import (
	vk "github.com/goki/vulkan"
)

type VulkanFramebuffer struct {
	Handle          vk.Framebuffer
	AttachmentCount uint32
	Attachments     []vk.ImageView
	Renderpass      *VulkanRenderPass
}
