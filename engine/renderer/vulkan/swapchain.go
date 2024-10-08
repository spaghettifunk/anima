package vulkan

import (
	"fmt"
	"math"

	vk "github.com/goki/vulkan"
	"github.com/spaghettifunk/alaska-engine/engine/core"
)

type VulkanSwapchain struct {
	ImageFormat       vk.SurfaceFormat
	MaxFramesInFlight uint8
	Handle            vk.Swapchain
	ImageCount        uint32
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

func SwapchainCreate(context *VulkanContext, width uint32, height uint32) (*VulkanSwapchain, error) {
	// Simply create a new one.
	return createSwapchain(context, width, height)
}

func (vs *VulkanSwapchain) SwapchainRecreate(context *VulkanContext, width uint32, height uint32) (*VulkanSwapchain, error) {
	// Destroy the old and create a new one.
	vs.destroySwapchain(context)
	return createSwapchain(context, width, height)
}

func (vs *VulkanSwapchain) SwapchainDestroy(context *VulkanContext) {
	vs.destroySwapchain(context)
}

func (vs *VulkanSwapchain) SwapchainAcquireNextImageIndex(context *VulkanContext, timeoutNS uint64, imageAvailableSemaphore vk.Semaphore, fence vk.Fence) (uint32, bool) {
	var outImageIndex *uint32
	result := vk.AcquireNextImage(context.Device.LogicalDevice, vs.Handle, timeoutNS, imageAvailableSemaphore, fence, outImageIndex)

	if result == vk.ErrorOutOfDate {
		// Trigger swapchain recreation, then boot out of the render loop.
		vs.SwapchainRecreate(context, context.FramebufferWidth, context.FramebufferHeight)
		return 0, false
	} else if result != vk.Success && result != vk.Suboptimal {
		core.LogFatal("Failed to acquire swapchain image!")
		return 0, false
	}

	return *outImageIndex, true
}

func (vs *VulkanSwapchain) SwapchainPresent(context *VulkanContext, graphicsQueue vk.Queue, presentQueue vk.Queue, renderCompleteSemaphore vk.Semaphore, presentImageIndex uint32) {
	// Return the image to the swapchain for presentation.
	presentInfo := vk.PresentInfo{
		SType:              vk.StructureTypePresentInfo,
		WaitSemaphoreCount: 1,
		PWaitSemaphores:    []vk.Semaphore{renderCompleteSemaphore},
		SwapchainCount:     1,
		PSwapchains:        []vk.Swapchain{vs.Handle},
		PImageIndices:      []uint32{presentImageIndex},
		PResults:           nil,
	}

	result := vk.QueuePresent(presentQueue, &presentInfo)
	if result == vk.ErrorOutOfDate || result == vk.Suboptimal {
		// Swapchain is out of date, suboptimal or a framebuffer resize has occurred. Trigger swapchain recreation.
		vs.SwapchainRecreate(context, context.FramebufferWidth, context.FramebufferHeight)
	} else if result != vk.Success {
		core.LogFatal("Failed to present swap chain image!")
	}

	// Increment (and loop) the index.
	context.CurrentFrame = (context.CurrentFrame + 1) % uint32(vs.MaxFramesInFlight)
}

func createSwapchain(context *VulkanContext, width, height uint32) (*VulkanSwapchain, error) {
	swapchain := &VulkanSwapchain{}

	swapchainExtent := vk.Extent2D{
		Width:  width,
		Height: height,
	}
	swapchain.MaxFramesInFlight = 2

	// Choose a swap surface format.
	found := false
	for i := 0; i < int(context.Device.SwapchainSupport.FormatCount); i++ {
		format := context.Device.SwapchainSupport.Formats[i]
		// Preferred formats
		if format.Format == vk.FormatB8g8r8a8Unorm &&
			format.ColorSpace == vk.ColorSpaceSrgbNonlinear {
			swapchain.ImageFormat = format
			found = true
		}
	}

	if !found {
		swapchain.ImageFormat = context.Device.SwapchainSupport.Formats[0]
	}

	presentMode := vk.PresentModeFifo
	for i := 0; i < int(context.Device.SwapchainSupport.PresentModeCount); i++ {
		mode := context.Device.SwapchainSupport.PresentModes[i]
		if mode == vk.PresentModeMailbox {
			presentMode = mode
			break
		}
	}

	// Swapchain extent
	if context.Device.SwapchainSupport.Capabilities.CurrentExtent.Width != math.MaxUint32 {
		swapchainExtent = context.Device.SwapchainSupport.Capabilities.CurrentExtent
	}

	// Clamp to the value allowed by the GPU.
	min := context.Device.SwapchainSupport.Capabilities.MinImageExtent
	max := context.Device.SwapchainSupport.Capabilities.MaxImageExtent
	swapchainExtent.Width = MathClamp(swapchainExtent.Width, min.Width, max.Width)
	swapchainExtent.Height = MathClamp(swapchainExtent.Height, min.Height, max.Height)

	imageCount := context.Device.SwapchainSupport.Capabilities.MinImageCount + 1
	if context.Device.SwapchainSupport.Capabilities.MaxImageCount > 0 && imageCount > context.Device.SwapchainSupport.Capabilities.MaxImageCount {
		imageCount = context.Device.SwapchainSupport.Capabilities.MaxImageCount
	}

	// Swapchain create info
	swapchainCreateInfo := vk.SwapchainCreateInfo{
		SType:            vk.StructureTypeSwapchainCreateInfo,
		Surface:          context.Surface,
		MinImageCount:    imageCount,
		ImageFormat:      swapchain.ImageFormat.Format,
		ImageColorSpace:  swapchain.ImageFormat.ColorSpace,
		ImageExtent:      swapchainExtent,
		ImageArrayLayers: 1,
		ImageUsage:       vk.ImageUsageFlags(vk.ImageUsageColorAttachmentBit),
	}

	// Setup the queue family indices
	if context.Device.GraphicsQueueIndex != context.Device.PresentQueueIndex {
		queueFamilyIndices := []uint32{
			uint32(context.Device.GraphicsQueueIndex),
			uint32(context.Device.PresentQueueIndex),
		}
		swapchainCreateInfo.ImageSharingMode = vk.SharingModeConcurrent
		swapchainCreateInfo.QueueFamilyIndexCount = 2
		swapchainCreateInfo.PQueueFamilyIndices = queueFamilyIndices
	} else {
		swapchainCreateInfo.ImageSharingMode = vk.SharingModeExclusive
		swapchainCreateInfo.QueueFamilyIndexCount = 0
		swapchainCreateInfo.PQueueFamilyIndices = nil
	}

	swapchainCreateInfo.PreTransform = context.Device.SwapchainSupport.Capabilities.CurrentTransform
	swapchainCreateInfo.CompositeAlpha = vk.CompositeAlphaOpaqueBit
	swapchainCreateInfo.PresentMode = presentMode
	swapchainCreateInfo.Clipped = vk.True
	swapchainCreateInfo.OldSwapchain = nil

	var swapchainHandle vk.Swapchain
	if res := vk.CreateSwapchain(context.Device.LogicalDevice, &swapchainCreateInfo, context.Allocator, &swapchainHandle); res != vk.Success {
		err := fmt.Errorf("failed to create swapchain")
		core.LogError(err.Error())
		return nil, err
	}
	swapchain.Handle = swapchainHandle

	// Start with a zero frame index.
	context.CurrentFrame = 0

	// Images
	swapchain.ImageCount = 0
	if res := vk.GetSwapchainImages(context.Device.LogicalDevice, swapchain.Handle, &swapchain.ImageCount, nil); res != vk.Success {
		err := fmt.Errorf("failed to get swapchain images")
		core.LogError(err.Error())
		return nil, err
	}
	if len(swapchain.Images) == 0 {
		// swapchain.images = (VkImage*)kallocate(sizeof(VkImage) * swapchain.image_count, MEMORY_TAG_RENDERER);
		swapchain.Images = make([]vk.Image, swapchain.ImageCount)
	}
	if len(swapchain.Views) == 0 {
		// swapchain.views = (VkImageView*)kallocate(sizeof(VkImageView) * swapchain.image_count, MEMORY_TAG_RENDERER);
		swapchain.Views = make([]vk.ImageView, swapchain.ImageCount)
	}
	if res := vk.GetSwapchainImages(context.Device.LogicalDevice, swapchain.Handle, &swapchain.ImageCount, swapchain.Images); res != vk.Success {
		err := fmt.Errorf("failed to get swapchain images")
		core.LogError(err.Error())
		return nil, err
	}

	// Views
	for i := 0; i < int(swapchain.ImageCount); i++ {
		viewInfo := vk.ImageViewCreateInfo{
			SType:    vk.StructureTypeImageViewCreateInfo,
			Image:    swapchain.Images[i],
			ViewType: vk.ImageViewType2d,
			Format:   swapchain.ImageFormat.Format,
			SubresourceRange: vk.ImageSubresourceRange{
				AspectMask:     vk.ImageAspectFlags(vk.ImageAspectColorBit),
				BaseMipLevel:   0,
				LevelCount:     1,
				BaseArrayLayer: 0,
				LayerCount:     1,
			},
		}

		if res := vk.CreateImageView(context.Device.LogicalDevice, &viewInfo, context.Allocator, &swapchain.Views[i]); res != vk.Success {
			err := fmt.Errorf("failed to create image view")
			core.LogError(err.Error())
			return nil, err
		}
	}

	// Depth resources
	if !DeviceDetectDepthFormat(context.Device) {
		context.Device.DepthFormat = vk.FormatUndefined
		core.LogFatal("Failed to find a supported format!")
	}

	// Create depth image and its view.
	depthAttachment, err := ImageCreate(
		context,
		vk.ImageType2d,
		swapchainExtent.Width,
		swapchainExtent.Height,
		context.Device.DepthFormat,
		vk.ImageTilingOptimal,
		vk.ImageUsageFlags(vk.ImageUsageDepthStencilAttachmentBit),
		vk.MemoryPropertyFlags(vk.MemoryPropertyDeviceLocalBit),
		true,
		vk.ImageAspectFlags(vk.ImageAspectDepthBit))
	if err != nil {

	}

	swapchain.DepthAttachment = depthAttachment

	core.LogInfo("Swapchain created successfully.")

	return swapchain, nil
}

func (vs *VulkanSwapchain) destroySwapchain(context *VulkanContext) {
	vk.DeviceWaitIdle(context.Device.LogicalDevice)
	vs.DepthAttachment.ImageDestroy(context)

	// Only destroy the views, not the images, since those are owned by the swapchain and are thus
	// destroyed when it is.
	for i := 0; i < int(vs.ImageCount); i++ {
		vk.DestroyImageView(context.Device.LogicalDevice, vs.Views[i], context.Allocator)
	}

	vk.DestroySwapchain(context.Device.LogicalDevice, vs.Handle, context.Allocator)
}
