package vulkan

import (
	"fmt"

	vk "github.com/goki/vulkan"
	"github.com/spaghettifunk/alaska-engine/engine/core"
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

	if res := vk.CreateImage(context.Device.LogicalDevice, &imageCreateInfo, context.Allocator, &outImage.Handle); res != vk.Success {
		return nil, nil
	}

	// Query memory requirements.
	memoryRequirements := vk.MemoryRequirements{}
	vk.GetImageMemoryRequirements(context.Device.LogicalDevice, outImage.Handle, &memoryRequirements)

	memoryType := context.FindMemoryIndex(memoryRequirements.MemoryTypeBits, uint32(memoryFlags))
	if memoryType == -1 {
		core.LogError("Required memory type not found. Image not valid.")
		return nil, nil
	}

	// Allocate memory
	memoryAllocateInfo := vk.MemoryAllocateInfo{
		SType:           vk.StructureTypeMemoryAllocateInfo,
		AllocationSize:  memoryRequirements.Size,
		MemoryTypeIndex: uint32(memoryType),
	}
	if res := vk.AllocateMemory(context.Device.LogicalDevice, &memoryAllocateInfo, context.Allocator, &outImage.Memory); res != vk.Success {
		err := fmt.Errorf("failed to allocate memory for image")
		core.LogError(err.Error())
		return nil, err
	}

	// Bind the memory
	// TODO: configurable memory offset.
	if res := vk.BindImageMemory(context.Device.LogicalDevice, outImage.Handle, outImage.Memory, 0); res != vk.Success {
		err := fmt.Errorf("failed to bind image memory")
		core.LogError(err.Error())
		return nil, err
	}

	// Create view
	if createView {
		outImage.View = nil
		outImage.ImageViewCreate(context, format, viewAspectFlags)
	}
	return outImage, nil
}

func (vi *VulkanImage) ImageViewCreate(context *VulkanContext, format vk.Format, aspectFlags vk.ImageAspectFlags) error {
	viewCreateInfo := vk.ImageViewCreateInfo{
		SType:    vk.StructureTypeImageViewCreateInfo,
		Image:    vi.Handle,
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

	if res := vk.CreateImageView(context.Device.LogicalDevice, &viewCreateInfo, context.Allocator, &vi.View); res != vk.Success {
		return nil
	}
	return nil
}

func (vi *VulkanImage) ImageDestroy(context *VulkanContext) {
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
