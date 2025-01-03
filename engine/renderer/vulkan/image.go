package vulkan

import (
	"fmt"

	vk "github.com/goki/vulkan"
	"github.com/spaghettifunk/anima/engine/renderer/metadata"
)

type VulkanImage struct {
	Handle             vk.Image
	Memory             vk.DeviceMemory
	MemoryRequirements vk.MemoryRequirements // The GPU memory requirements for this image
	MemoryFlags        vk.MemoryPropertyFlags
	ImageView          vk.ImageView
	Width              uint32
	Height             uint32
}

func ImageCreate(context *VulkanContext, textureType metadata.TextureType, width uint32, height uint32,
	format vk.Format, tiling vk.ImageTiling, usage vk.ImageUsageFlags, memoryFlags vk.MemoryPropertyFlags,
	createView bool, viewAspectFlags vk.ImageAspectFlags) (*VulkanImage, error) {

	outImage := &VulkanImage{
		Width:       width,
		Height:      height,
		MemoryFlags: memoryFlags,
	}

	al := uint32(1)
	flags := vk.ImageCreateFlags(0)
	if textureType == metadata.TextureTypeCube {
		al = 6
		flags = vk.ImageCreateFlags(vk.ImageCreateCubeCompatibleBit)
	}

	// Creation info.
	imageCreateInfo := vk.ImageCreateInfo{
		SType:     vk.StructureTypeImageCreateInfo,
		ImageType: vk.ImageType2d, // Intentional, there is no cube image type.
		Extent: vk.Extent3D{
			Width:  width,
			Height: height,
			Depth:  1, // TODO: Support configurable depth.
		},
		MipLevels:     4, // TODO: Support mip mapping
		ArrayLayers:   al,
		Format:        format,
		Tiling:        tiling,
		InitialLayout: vk.ImageLayoutUndefined,
		Usage:         usage,
		Samples:       vk.SampleCount1Bit,      // TODO: Configurable sample count.
		SharingMode:   vk.SharingModeExclusive, // TODO: Configurable sharing mode.
		Flags:         flags,
	}
	imageCreateInfo.Deref()
	imageCreateInfo.Extent.Deref()

	if err := lockPool.SafeCall(ResourceManagement, func() error {
		if res := vk.CreateImage(context.Device.LogicalDevice, &imageCreateInfo, context.Allocator, &outImage.Handle); !VulkanResultIsSuccess(res) {
			err := fmt.Errorf("failed to create image with error %s", VulkanResultString(res, true))
			return err
		}
		return nil
	}); err != nil {
		return nil, err
	}

	// Query memory requirements.
	var memoryRequirements vk.MemoryRequirements
	if err := lockPool.SafeCall(ResourceManagement, func() error {
		vk.GetImageMemoryRequirements(context.Device.LogicalDevice, outImage.Handle, &memoryRequirements)
		return nil
	}); err != nil {
		return nil, err
	}
	memoryRequirements.Deref()

	memoryType := context.FindMemoryIndex(memoryRequirements.MemoryTypeBits, uint32(memoryFlags))
	if memoryType == -1 {
		err := fmt.Errorf("required memory type not found. Image not valid")
		return nil, err
	}

	// Allocate memory
	memoryAllocateInfo := vk.MemoryAllocateInfo{
		SType:           vk.StructureTypeMemoryAllocateInfo,
		AllocationSize:  memoryRequirements.Size,
		MemoryTypeIndex: uint32(memoryType),
	}
	memoryAllocateInfo.Deref()

	if err := lockPool.SafeCall(DeviceManagement, func() error {
		if res := vk.AllocateMemory(context.Device.LogicalDevice, &memoryAllocateInfo, context.Allocator, &outImage.Memory); !VulkanResultIsSuccess(res) {
			err := fmt.Errorf("failed to allocate memory for image")
			return err
		}
		return nil
	}); err != nil {
		return nil, err
	}

	// Bind the memory
	// TODO: configurable memory offset.
	if err := lockPool.SafeCall(ResourceManagement, func() error {
		if res := vk.BindImageMemory(context.Device.LogicalDevice, outImage.Handle, outImage.Memory, 0); !VulkanResultIsSuccess(res) {
			err := fmt.Errorf("failed to bind image memory")
			return err
		}
		return nil
	}); err != nil {
		return nil, err
	}

	// Create view
	if createView {
		if err := ImageViewCreate(context, textureType, format, viewAspectFlags, outImage); err != nil {
			return nil, err
		}
	}
	return outImage, nil
}

func ImageViewCreate(context *VulkanContext, textureType metadata.TextureType, format vk.Format, aspectFlags vk.ImageAspectFlags, image *VulkanImage) error {
	lc := uint32(1)
	viewType := vk.ImageViewType2d
	if textureType == metadata.TextureTypeCube {
		lc = 6
		viewType = vk.ImageViewTypeCube
	}

	viewCreateInfo := vk.ImageViewCreateInfo{
		SType:    vk.StructureTypeImageViewCreateInfo,
		Image:    image.Handle,
		ViewType: viewType,
		Format:   format,
		SubresourceRange: vk.ImageSubresourceRange{
			AspectMask: aspectFlags,
			// TODO: Make configurable
			BaseMipLevel:   0,
			LevelCount:     1,
			BaseArrayLayer: 0,
			LayerCount:     lc,
		},
	}
	viewCreateInfo.Deref()

	if err := lockPool.SafeCall(ResourceManagement, func() error {
		if res := vk.CreateImageView(context.Device.LogicalDevice, &viewCreateInfo, context.Allocator, &image.ImageView); !VulkanResultIsSuccess(res) {
			err := fmt.Errorf("create image view failed with error %s", VulkanResultString(res, true))
			return err
		}
		return nil
	}); err != nil {
		return err
	}
	return nil
}

func (image *VulkanImage) ImageTransitionLayout(context *VulkanContext, textureType metadata.TextureType, commandBuffer *VulkanCommandBuffer, format vk.Format, oldLayout, newLayout vk.ImageLayout) error {
	lc := uint32(1)
	if textureType == metadata.TextureTypeCube {
		lc = 6
	}
	barrier := vk.ImageMemoryBarrier{
		SType:               vk.StructureTypeImageMemoryBarrier,
		OldLayout:           oldLayout,
		NewLayout:           newLayout,
		SrcQueueFamilyIndex: uint32(context.Device.GraphicsQueueIndex),
		DstQueueFamilyIndex: uint32(context.Device.GraphicsQueueIndex),
		Image:               image.Handle,
		SubresourceRange: vk.ImageSubresourceRange{
			AspectMask:     vk.ImageAspectFlags(vk.ImageAspectColorBit),
			BaseMipLevel:   0,
			LevelCount:     1,
			BaseArrayLayer: 0,
			LayerCount:     lc,
		},
	}

	var sourceStage vk.PipelineStageFlags
	var destStage vk.PipelineStageFlags
	// Don't care about the old layout - transition to optimal layout (for the underlying implementation).
	if oldLayout == vk.ImageLayoutUndefined && newLayout == vk.ImageLayoutTransferDstOptimal {
		barrier.SrcAccessMask = 0
		barrier.DstAccessMask = vk.AccessFlags(vk.AccessTransferWriteBit)
		// Don't care what stage the pipeline is in at the start.
		sourceStage = vk.PipelineStageFlags(vk.PipelineStageTopOfPipeBit)
		// Used for copying
		destStage = vk.PipelineStageFlags(vk.PipelineStageTransferBit)
	} else if oldLayout == vk.ImageLayoutTransferDstOptimal && newLayout == vk.ImageLayoutShaderReadOnlyOptimal {
		// Transitioning from a transfer destination layout to a shader-readonly layout.
		barrier.SrcAccessMask = vk.AccessFlags(vk.AccessTransferWriteBit)
		barrier.DstAccessMask = vk.AccessFlags(vk.AccessShaderReadBit)
		// From a copying stage to...
		sourceStage = vk.PipelineStageFlags(vk.PipelineStageTransferBit)
		// The fragment stage.
		destStage = vk.PipelineStageFlags(vk.PipelineStageFragmentShaderBit)
	} else if oldLayout == vk.ImageLayoutTransferSrcOptimal && newLayout == vk.ImageLayoutShaderReadOnlyOptimal {
		// Transitioning from a transfer source layout to a shader-readonly layout.
		barrier.SrcAccessMask = vk.AccessFlags(vk.AccessTransferReadBit)
		barrier.DstAccessMask = vk.AccessFlags(vk.AccessShaderReadBit)
		// From a copying stage to...
		sourceStage = vk.PipelineStageFlags(vk.PipelineStageTransferBit)
		// The fragment stage.
		destStage = vk.PipelineStageFlags(vk.PipelineStageFragmentShaderBit)
	} else if oldLayout == vk.ImageLayoutUndefined && newLayout == vk.ImageLayoutTransferSrcOptimal {
		barrier.SrcAccessMask = 0
		barrier.DstAccessMask = vk.AccessFlags(vk.AccessTransferReadBit)
		// Don't care what stage the pipeline is in at the start.
		sourceStage = vk.PipelineStageFlags(vk.PipelineStageTopOfPipeBit)
		// Used for copying
		destStage = vk.PipelineStageFlags(vk.PipelineStageTransferBit)
	} else {
		err := fmt.Errorf("unsupported layout transition")
		return err
	}
	barrier.Deref()

	if err := lockPool.SafeCall(CommandBufferManagement, func() error {
		vk.CmdPipelineBarrier(commandBuffer.Handle, sourceStage, destStage,
			0,
			0, nil,
			0, nil,
			1, []vk.ImageMemoryBarrier{barrier},
		)
		return nil
	}); err != nil {
		return err
	}
	return nil
}

func (image *VulkanImage) ImageCopyFromBuffer(context *VulkanContext, textureType metadata.TextureType, buffer vk.Buffer, commandBuffer *VulkanCommandBuffer) error {
	lc := uint32(1)
	if textureType == metadata.TextureTypeCube {
		lc = 6
	}
	// Region to copy
	region := vk.BufferImageCopy{
		BufferOffset:      0,
		BufferRowLength:   0,
		BufferImageHeight: 0,
		ImageSubresource: vk.ImageSubresourceLayers{
			AspectMask:     vk.ImageAspectFlags(vk.ImageAspectColorBit),
			MipLevel:       0,
			BaseArrayLayer: 0,
			LayerCount:     lc,
		},
		ImageExtent: vk.Extent3D{
			Width:  image.Width,
			Height: image.Height,
			Depth:  1,
		},
	}
	region.Deref()

	if err := lockPool.SafeCall(CommandBufferManagement, func() error {
		vk.CmdCopyBufferToImage(
			commandBuffer.Handle,
			buffer,
			image.Handle,
			vk.ImageLayoutTransferDstOptimal,
			1,
			[]vk.BufferImageCopy{region},
		)
		return nil
	}); err != nil {
		return err
	}
	return nil
}

func (image *VulkanImage) ImageCopyToBuffer(context *VulkanContext, textureType metadata.TextureType, buffer vk.Buffer, commandBuffer *VulkanCommandBuffer) error {
	lc := uint32(1)
	if textureType == metadata.TextureTypeCube {
		lc = 6
	}
	// Region to copy
	region := vk.BufferImageCopy{
		BufferOffset:      0,
		BufferRowLength:   0,
		BufferImageHeight: 0,
		ImageSubresource: vk.ImageSubresourceLayers{
			AspectMask:     vk.ImageAspectFlags(vk.ImageAspectColorBit),
			MipLevel:       0,
			BaseArrayLayer: 0,
			LayerCount:     lc,
		},
		ImageExtent: vk.Extent3D{
			Width:  image.Width,
			Height: image.Height,
			Depth:  1,
		},
	}
	region.Deref()

	if err := lockPool.SafeCall(CommandBufferManagement, func() error {
		vk.CmdCopyImageToBuffer(
			commandBuffer.Handle,
			image.Handle,
			vk.ImageLayoutTransferSrcOptimal,
			buffer,
			1,
			[]vk.BufferImageCopy{region},
		)
		return nil
	}); err != nil {
		return err
	}
	return nil
}

func (vi *VulkanImage) Destroy(context *VulkanContext) error {
	if vi.ImageView != nil {
		if err := lockPool.SafeCall(ResourceManagement, func() error {
			vk.DestroyImageView(context.Device.LogicalDevice, vi.ImageView, context.Allocator)
			return nil
		}); err != nil {
			return err
		}
		vi.ImageView = nil
	}
	if vi.Memory != nil {
		if err := lockPool.SafeCall(DeviceManagement, func() error {
			vk.FreeMemory(context.Device.LogicalDevice, vi.Memory, context.Allocator)
			return nil
		}); err != nil {
			return err
		}
		vi.Memory = nil
	}
	if vi.Handle != nil {
		if err := lockPool.SafeCall(ResourceManagement, func() error {
			vk.DestroyImage(context.Device.LogicalDevice, vi.Handle, context.Allocator)
			return nil
		}); err != nil {
			return err
		}
		vi.Handle = nil
	}
	return nil
}

func (vi *VulkanImage) ImageCopyPixelToBuffer(context *VulkanContext, textureType metadata.TextureType, buffer vk.Buffer, x, y uint32, commandBuffer *VulkanCommandBuffer) error {
	lc := uint32(1)
	if textureType == metadata.TextureTypeCube {
		lc = 6
	}
	region := vk.BufferImageCopy{
		BufferOffset:      0,
		BufferRowLength:   0,
		BufferImageHeight: 0,
		ImageSubresource: vk.ImageSubresourceLayers{
			AspectMask:     vk.ImageAspectFlags(vk.ImageAspectColorBit),
			MipLevel:       0,
			BaseArrayLayer: 0,
			LayerCount:     lc,
		},
		ImageOffset: vk.Offset3D{
			X: int32(x),
			Y: int32(y),
		},
		ImageExtent: vk.Extent3D{
			Width:  1,
			Height: 1,
			Depth:  1,
		},
	}
	region.Deref()

	if err := lockPool.SafeCall(CommandBufferManagement, func() error {
		vk.CmdCopyImageToBuffer(commandBuffer.Handle, vi.Handle, vk.ImageLayoutTransferSrcOptimal, buffer, 1, []vk.BufferImageCopy{region})
		return nil
	}); err != nil {
		return err
	}
	return nil
}
