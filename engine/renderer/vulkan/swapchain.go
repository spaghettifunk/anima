package vulkan

import (
	"fmt"
	"strconv"
	"strings"

	vk "github.com/goki/vulkan"
	"github.com/spaghettifunk/anima/engine/core"
	"github.com/spaghettifunk/anima/engine/math"
	"github.com/spaghettifunk/anima/engine/renderer/metadata"
)

type VulkanSwapchain struct {
	ImageFormat       vk.SurfaceFormat
	MaxFramesInFlight uint8
	Handle            vk.Swapchain
	ImageCount        uint32

	RenderTextures []*metadata.Texture
	DepthTextures  []*metadata.Texture
	RenderTargets  []*metadata.RenderTarget
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

func (vs *VulkanSwapchain) SwapchainAcquireNextImageIndex(context *VulkanContext, timeoutNS uint64, imageAvailableSemaphore vk.Semaphore, fence vk.Fence) (uint32, bool, error) {
	var outImageIndex uint32
	if err := lockPool.SafeCall(SwapchainManagement, func() error {
		result := vk.AcquireNextImage(context.Device.LogicalDevice, vs.Handle, timeoutNS, imageAvailableSemaphore, fence, &outImageIndex)
		if result == vk.ErrorOutOfDate {
			// Trigger swapchain recreation, then boot out of the render loop.
			sc, err := vs.SwapchainRecreate(context, context.FramebufferWidth, context.FramebufferHeight)
			if err != nil {
				return err
			}
			vs = sc
		} else if result != vk.Success && result != vk.Suboptimal {
			err := fmt.Errorf("failed to acquire swapchain image")
			return err
		}
		return nil
	}); err != nil {
		return 0, false, err
	}
	return outImageIndex, true, nil
}

func (vs *VulkanSwapchain) SwapchainPresent(context *VulkanContext, graphicsQueue vk.Queue, presentQueue vk.Queue, renderCompleteSemaphore vk.Semaphore, presentImageIndex uint32) error {
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
	presentInfo.Deref()

	if err := lockPool.SafeQueueCall(context.Device.PresentQueueIndex, func() error {
		result := vk.QueuePresent(presentQueue, &presentInfo)
		if result == vk.ErrorOutOfDate || result == vk.Suboptimal {
			// Swapchain is out of date, suboptimal or a framebuffer resize has occurred. Trigger swapchain recreation.
			sc, err := vs.SwapchainRecreate(context, context.FramebufferWidth, context.FramebufferHeight)
			if err != nil {
				return err
			}
			vs = sc
		} else if result != vk.Success {
			err := fmt.Errorf("failed to recreate the swapchain with error %s", VulkanResultString(result, true))
			return err
		}
		return nil
	}); err != nil {
		return err
	}

	// Increment (and loop) the index.
	context.CurrentFrame = (context.CurrentFrame + 1) % uint32(vs.MaxFramesInFlight)

	return nil
}

func createSwapchain(context *VulkanContext, width, height uint32) (*VulkanSwapchain, error) {
	swapchain := &VulkanSwapchain{}

	swapchainExtent := vk.Extent2D{
		Width:  width,
		Height: height,
	}

	// Choose a swap surface format.
	found := false
	for i := 0; i < int(context.Device.SwapchainSupport.FormatCount); i++ {
		format := context.Device.SwapchainSupport.Formats[i]
		// Preferred formats
		if format.Format == vk.FormatB8g8r8a8Unorm &&
			format.ColorSpace == vk.ColorSpaceSrgbNonlinear {
			swapchain.ImageFormat = format
			found = true
			break
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

	supportInfo := &VulkanSwapchainSupportInfo{}
	if err := DeviceQuerySwapchainSupport(context.Device.PhysicalDevice, context.Surface, supportInfo); err != nil {
		return nil, err
	}
	context.Device.SwapchainSupport = supportInfo

	// Swapchain extent
	context.Device.SwapchainSupport.Capabilities.CurrentExtent.Deref()
	if context.Device.SwapchainSupport.Capabilities.CurrentExtent.Width != vk.MaxUint32 {
		swapchainExtent = context.Device.SwapchainSupport.Capabilities.CurrentExtent
	}

	// Clamp to the value allowed by the GPU.
	context.Device.SwapchainSupport.Capabilities.MinImageExtent.Deref()
	context.Device.SwapchainSupport.Capabilities.MaxImageExtent.Deref()

	min := context.Device.SwapchainSupport.Capabilities.MinImageExtent
	max := context.Device.SwapchainSupport.Capabilities.MaxImageExtent
	swapchainExtent.Width = math.Clamp(swapchainExtent.Width, min.Width, max.Width)
	swapchainExtent.Height = math.Clamp(swapchainExtent.Height, min.Height, max.Height)

	imageCount := context.Device.SwapchainSupport.Capabilities.MinImageCount + 1
	if context.Device.SwapchainSupport.Capabilities.MaxImageCount > 0 && imageCount > context.Device.SwapchainSupport.Capabilities.MaxImageCount {
		imageCount = context.Device.SwapchainSupport.Capabilities.MaxImageCount
	}

	swapchain.MaxFramesInFlight = uint8(imageCount) - 1

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
	swapchainCreateInfo.Deref()

	var swapchainHandle vk.Swapchain
	if err := lockPool.SafeCall(SwapchainManagement, func() error {

		if res := vk.CreateSwapchain(context.Device.LogicalDevice, &swapchainCreateInfo, context.Allocator, &swapchainHandle); res != vk.Success {
			err := fmt.Errorf("failed to create swapchain with err `%s`", VulkanResultString(res, true))
			return err
		}
		swapchain.Handle = swapchainHandle
		return nil
	}); err != nil {
		return nil, err
	}

	// Start with a zero frame index.
	context.CurrentFrame = 0

	// Images
	swapchain.ImageCount = 0
	if err := lockPool.SafeCall(SwapchainManagement, func() error {
		if res := vk.GetSwapchainImages(context.Device.LogicalDevice, swapchain.Handle, &swapchain.ImageCount, nil); res != vk.Success {
			err := fmt.Errorf("failed to get swapchain images with err `%s`", VulkanResultString(res, true))
			return err
		}
		return nil
	}); err != nil {
		return nil, err
	}

	if swapchain.RenderTextures == nil {
		swapchain.RenderTextures = make([]*metadata.Texture, swapchain.ImageCount)
		// If creating the array, then the internal texture objects aren't created yet either.
		for i := uint32(0); i < swapchain.ImageCount; i++ {
			internal_data := &VulkanImage{}

			tex_name := strings.Builder{}
			tex_name.WriteString("__internal_vulkan_swapchain_image_0__")
			tex_name.WriteString("0")
			tex_name.WriteString(strconv.Itoa(int(i)))

			texture := &metadata.Texture{
				ID:           metadata.InvalidID,
				TextureType:  metadata.TextureType2d,
				Name:         tex_name.String(),
				Width:        width,
				Height:       height,
				ChannelCount: 4,
				Generation:   metadata.InvalidID,
				InternalData: internal_data,
				Flags:        0,
			}

			hasTransparency := false
			isWriteable := true

			if hasTransparency {
				texture.Flags |= metadata.TextureFlagBits(metadata.TextureFlagHasTransparency)
			}
			if isWriteable {
				texture.Flags |= metadata.TextureFlagBits(metadata.TextureFlagIsWriteable)
			}
			texture.Flags |= metadata.TextureFlagBits(metadata.TextureFlagIsWrapped)

			swapchain.RenderTextures[i] = texture

			if swapchain.RenderTextures[i] == nil {
				err := fmt.Errorf("failed to generate new swapchain image texture")
				return nil, err
			}
		}
	} else {
		for i := 0; i < int(swapchain.ImageCount); i++ {
			// Just update the dimensions.
			core.LogWarn("missing update dimensions of the rendered textures")
			// FIXME: this needs to be handled at some point
			// texture_system_resize(&swapchain.render_textures[i], swapchain_extent.width, swapchain_extent.height, false);
		}
	}
	swapchain_images := make([]vk.Image, swapchain.ImageCount)
	if err := lockPool.SafeCall(SwapchainManagement, func() error {
		result := vk.GetSwapchainImages(context.Device.LogicalDevice, swapchain.Handle, &swapchain.ImageCount, swapchain_images)
		if !VulkanResultIsSuccess(result) {
			err := fmt.Errorf("failed to swap-images with error %s", VulkanResultString(result, true))
			return err
		}
		return nil
	}); err != nil {
		return nil, err
	}

	for i := 0; i < int(swapchain.ImageCount); i++ {
		// Update the internal image for each.
		image := swapchain.RenderTextures[i].InternalData.(*VulkanImage)
		image.Handle = swapchain_images[i]
		image.Width = swapchainExtent.Width
		image.Height = swapchainExtent.Height
	}

	// Views
	for i := 0; i < int(swapchain.ImageCount); i++ {
		image := swapchain.RenderTextures[i].InternalData.(*VulkanImage)

		viewInfo := vk.ImageViewCreateInfo{
			SType:    vk.StructureTypeImageViewCreateInfo,
			Image:    image.Handle,
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

		if err := lockPool.SafeCall(ResourceManagement, func() error {
			result := vk.CreateImageView(context.Device.LogicalDevice, &viewInfo, context.Allocator, &image.View)
			if !VulkanResultIsSuccess(result) {
				err := fmt.Errorf("failed to create image view with error %s", VulkanResultString(result, true))
				return err
			}
			return nil
		}); err != nil {
			return nil, err
		}
	}

	// Depth resources
	if err := DeviceDetectDepthFormat(context.Device); err != nil {
		context.Device.DepthFormat = vk.FormatUndefined
		core.LogError("failed to find a supported format")
		return nil, err
	}

	if len(swapchain.DepthTextures) == 0 {
		swapchain.DepthTextures = make([]*metadata.Texture, swapchain.ImageCount)
	}

	for i := 0; i < int(swapchain.ImageCount); i++ {
		image, err := ImageCreate(
			context,
			metadata.TextureType2d,
			swapchainExtent.Width,
			swapchainExtent.Height,
			context.Device.DepthFormat,
			vk.ImageTilingOptimal,
			vk.ImageUsageFlags(vk.ImageUsageDepthStencilAttachmentBit),
			vk.MemoryPropertyFlags(vk.MemoryPropertyDeviceLocalBit),
			true,
			vk.ImageAspectFlags(vk.ImageAspectDepthBit))
		if err != nil {
			return nil, err
		}

		texture := &metadata.Texture{
			ID:           metadata.InvalidID,
			TextureType:  metadata.TextureType2d,
			Name:         "__kohi_default_depth_texture__",
			Width:        width,
			Height:       height,
			ChannelCount: 4,
			Generation:   metadata.InvalidID,
			InternalData: image,
			Flags:        0,
		}

		hasTransparency := false
		isWriteable := true
		if hasTransparency {
			texture.Flags |= metadata.TextureFlagBits(metadata.TextureFlagHasTransparency)
		}
		if isWriteable {
			texture.Flags |= metadata.TextureFlagBits(metadata.TextureFlagIsWriteable)
		}
		texture.Flags |= metadata.TextureFlagBits(metadata.TextureFlagIsWrapped)

		swapchain.DepthTextures[i] = texture
	}

	core.LogInfo("Swapchain created successfully.")

	return swapchain, nil
}

func (vs *VulkanSwapchain) destroySwapchain(context *VulkanContext) error {
	if err := lockPool.SafeCall(DeviceManagement, func() error {
		if res := vk.DeviceWaitIdle(context.Device.LogicalDevice); !VulkanResultIsSuccess(res) {
			err := fmt.Errorf("device wait idle failed with error %s", VulkanResultString(res, true))
			return err
		}
		return nil
	}); err != nil {
		return err
	}

	for i := 0; i < int(context.Swapchain.ImageCount); i++ {
		image := vs.DepthTextures[i].InternalData.(*VulkanImage)
		image.Destroy(context)
		vs.DepthTextures[i].InternalData = nil
	}

	// Only destroy the views, not the images, since those are owned by the swapchain and are thus
	// destroyed when it is.
	for i := 0; i < int(vs.ImageCount); i++ {
		image := vs.RenderTextures[i].InternalData.(*VulkanImage)
		if err := lockPool.SafeCall(ResourceManagement, func() error {
			vk.DestroyImageView(context.Device.LogicalDevice, image.View, context.Allocator)
			return nil
		}); err != nil {
			return err
		}
	}

	if err := lockPool.SafeCall(SwapchainManagement, func() error {
		vk.DestroySwapchain(context.Device.LogicalDevice, vs.Handle, context.Allocator)
		return nil
	}); err != nil {
		return err
	}

	return nil
}
