package vulkan

import (
	vk "github.com/goki/vulkan"
)

type VulkanSwapchain struct {
	ImageFormat       vk.SurfaceFormat
	MaxFramesInFlight uint8
	Handle            vk.Swapchain
	Image_count       uint32
	Images            []vk.Image
	Views             []vk.ImageView

	DepthAttachment *VulkanImage

	// framebuffers used for on-screen rendering.
	Framebuffers []*VulkanFramebuffer
}

type VulkanSwapchainSupportInfo struct {
	Capabilities     vk.SurfaceCapabilities
	FormatCount      uint32
	Formats          []vk.SurfaceFormat
	PresentModeCount uint32
	PresentModes     []vk.PresentMode
}
