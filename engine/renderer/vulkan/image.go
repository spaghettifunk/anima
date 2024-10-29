package vulkan

import (
	"fmt"

	vk "github.com/goki/vulkan"
	"github.com/spaghettifunk/anima/engine/core"
)

type VulkanImage struct {
	Handle vk.Image
	Memory vk.DeviceMemory
	View   vk.ImageView
	Width  uint32
	Height uint32
}

func ImageCreate(context *VulkanContext, imageType vk.ImageType, width uint32, height uint32,
	format vk.Format, tiling vk.ImageTiling, usage vk.ImageUsageFlags, memoryFlags vk.MemoryPropertyFlags,
	createView bool, viewAspectFlags vk.ImageAspectFlags) (*VulkanImage, error) {

	outImage := &VulkanImage{
		Width:  width,
		Height: height,
	}

	// Creation info.
	imageCreateInfo := vk.ImageCreateInfo{
		SType:     vk.StructureTypeImageCreateInfo,
		ImageType: vk.ImageType2d,
		Extent: vk.Extent3D{
			Width:  width,
			Height: height,
			Depth:  1, // TODO: Support configurable depth.
		},
		MipLevels:     4, // TODO: Support mip mapping
		ArrayLayers:   1, // TODO: Support number of layers in the image.
		Format:        format,
		Tiling:        tiling,
		InitialLayout: vk.ImageLayoutUndefined,
		Usage:         usage,
		Samples:       vk.SampleCount1Bit,      // TODO: Configurable sample count.
		SharingMode:   vk.SharingModeExclusive, // TODO: Configurable sharing mode.
	}

	var handle vk.Image
	if res := vk.CreateImage(context.Device.LogicalDevice, &imageCreateInfo, context.Allocator, &handle); res != vk.Success {
		return nil, nil
	}
	outImage.Handle = handle

	// Query memory requirements.
	var memoryRequirements vk.MemoryRequirements
	vk.GetImageMemoryRequirements(context.Device.LogicalDevice, outImage.Handle, &memoryRequirements)
	memoryRequirements.Deref()

	memoryType := context.FindMemoryIndex(memoryRequirements.MemoryTypeBits, uint32(memoryFlags))
	if memoryType == -1 {
		err := fmt.Errorf("required memory type not found. Image not valid")
		core.LogError(err.Error())
		return nil, err
	}

	// Allocate memory
	memoryAllocateInfo := vk.MemoryAllocateInfo{
		SType:           vk.StructureTypeMemoryAllocateInfo,
		AllocationSize:  memoryRequirements.Size,
		MemoryTypeIndex: uint32(memoryType),
	}
	var pMemory vk.DeviceMemory
	if res := vk.AllocateMemory(context.Device.LogicalDevice, &memoryAllocateInfo, context.Allocator, &pMemory); res != vk.Success {
		err := fmt.Errorf("failed to allocate memory for image")
		core.LogError(err.Error())
		return nil, err
	}
	outImage.Memory = pMemory

	// Bind the memory
	// TODO: configurable memory offset.
	if res := vk.BindImageMemory(context.Device.LogicalDevice, outImage.Handle, outImage.Memory, 0); res != vk.Success {
		err := fmt.Errorf("failed to bind image memory")
		core.LogError(err.Error())
		return nil, err
	}

	// Create view
	if createView {
		view, err := ImageViewCreate(context, format, viewAspectFlags, outImage)
		if err != nil {
			core.LogError(err.Error())
			return nil, err
		}
		outImage.View = *view
	}
	return outImage, nil
}

func ImageViewCreate(context *VulkanContext, format vk.Format, aspectFlags vk.ImageAspectFlags, image *VulkanImage) (*vk.ImageView, error) {
	viewCreateInfo := vk.ImageViewCreateInfo{
		SType:    vk.StructureTypeImageViewCreateInfo,
		Image:    image.Handle,
		ViewType: vk.ImageViewType2d, // TODO: Make configurable
		Format:   format,
		SubresourceRange: vk.ImageSubresourceRange{
			AspectMask: aspectFlags,
			// TODO: Make configurable
			BaseMipLevel:   0,
			LevelCount:     1,
			BaseArrayLayer: 0,
			LayerCount:     1,
		},
	}

	var view vk.ImageView
	if res := vk.CreateImageView(context.Device.LogicalDevice, &viewCreateInfo, context.Allocator, &view); res != vk.Success {
		return nil, nil
	}
	return &view, nil
}

func ImageTransitionLayout(vulkan_context* context, texture_type type, vulkan_command_buffer* command_buffer, vulkan_image* image, VkFormat format, VkImageLayout old_layout, VkImageLayout new_layout) {
    VkImageMemoryBarrier barrier = {VK_STRUCTURE_TYPE_IMAGE_MEMORY_BARRIER};
    barrier.oldLayout = old_layout;
    barrier.newLayout = new_layout;
    barrier.srcQueueFamilyIndex = context->device.graphics_queue_index;
    barrier.dstQueueFamilyIndex = context->device.graphics_queue_index;
    barrier.image = image->handle;
    barrier.subresourceRange.aspectMask = VK_IMAGE_ASPECT_COLOR_BIT;
    barrier.subresourceRange.baseMipLevel = 0;
    barrier.subresourceRange.levelCount = 1;
    barrier.subresourceRange.baseArrayLayer = 0;
    barrier.subresourceRange.layerCount = type == TEXTURE_TYPE_CUBE ? 6 : 1;

    VkPipelineStageFlags source_stage;
    VkPipelineStageFlags dest_stage;

    // Don't care about the old layout - transition to optimal layout (for the underlying implementation).
    if (old_layout == VK_IMAGE_LAYOUT_UNDEFINED && new_layout == VK_IMAGE_LAYOUT_TRANSFER_DST_OPTIMAL) {
        barrier.srcAccessMask = 0;
        barrier.dstAccessMask = VK_ACCESS_TRANSFER_WRITE_BIT;

        // Don't care what stage the pipeline is in at the start.
        source_stage = VK_PIPELINE_STAGE_TOP_OF_PIPE_BIT;

        // Used for copying
        dest_stage = VK_PIPELINE_STAGE_TRANSFER_BIT;
    } else if (old_layout == VK_IMAGE_LAYOUT_TRANSFER_DST_OPTIMAL && new_layout == VK_IMAGE_LAYOUT_SHADER_READ_ONLY_OPTIMAL) {
        // Transitioning from a transfer destination layout to a shader-readonly layout.
        barrier.srcAccessMask = VK_ACCESS_TRANSFER_WRITE_BIT;
        barrier.dstAccessMask = VK_ACCESS_SHADER_READ_BIT;

        // From a copying stage to...
        source_stage = VK_PIPELINE_STAGE_TRANSFER_BIT;

        // The fragment stage.
        dest_stage = VK_PIPELINE_STAGE_FRAGMENT_SHADER_BIT;
    } else {
        KFATAL("unsupported layout transition!");
        return;
    }

    vkCmdPipelineBarrier(
        command_buffer->handle,
        source_stage, dest_stage,
        0,
        0, 0,
        0, 0,
        1, &barrier);
}

func ImageCopyFromBuffer(vulkan_context* context,texture_type type,vulkan_image* image,VkBuffer buffer,vulkan_command_buffer* command_buffer) {
    // Region to copy
    VkBufferImageCopy region;
    kzero_memory(&region, sizeof(VkBufferImageCopy));
    region.bufferOffset = 0;
    region.bufferRowLength = 0;
    region.bufferImageHeight = 0;

    region.imageSubresource.aspectMask = VK_IMAGE_ASPECT_COLOR_BIT;
    region.imageSubresource.mipLevel = 0;
    region.imageSubresource.baseArrayLayer = 0;
    region.imageSubresource.layerCount = type == TEXTURE_TYPE_CUBE ? 6 : 1;

    region.imageExtent.width = image->width;
    region.imageExtent.height = image->height;
    region.imageExtent.depth = 1;

    vkCmdCopyBufferToImage(
        command_buffer->handle,
        buffer,
        image->handle,
        VK_IMAGE_LAYOUT_TRANSFER_DST_OPTIMAL,
        1,
        &region);
}

func (vi *VulkanImage) Destroy(context *VulkanContext) {
	if vi.View != nil {
		vk.DestroyImageView(context.Device.LogicalDevice, vi.View, context.Allocator)
		vi.View = nil
	}
	if vi.Memory != nil {
		vk.FreeMemory(context.Device.LogicalDevice, vi.Memory, context.Allocator)
		vi.Memory = nil
	}
	if vi.Handle != nil {
		vk.DestroyImage(context.Device.LogicalDevice, vi.Handle, context.Allocator)
		vi.Handle = nil
	}
}
