package vulkan

import (
	vk "github.com/goki/vulkan"
	"github.com/spaghettifunk/anima/engine/core"
)

type VulkanContext struct {
	// The framebuffer's current width.
	FramebufferWidth uint32
	// The framebuffer's current height.
	FramebufferHeight uint32
	// Current generation of framebuffer size. If it does not match framebuffer_size_last_generation,
	// a new one should be generated.
	FramebufferSizeGeneration uint64
	// The generation of the framebuffer when it was last created. Set to framebuffer_size_generation
	// when updated.
	FramebufferSizeLastGeneration uint64

	Instance  vk.Instance
	Allocator *vk.AllocationCallbacks
	Surface   vk.Surface

	// TODO: only in DEBUG mode
	debugMessenger vk.DebugReportCallback

	Device *VulkanDevice

	Swapchain      *VulkanSwapchain
	MainRenderpass *VulkanRenderpass

	// darray
	GraphicsCommandBuffers []*VulkanCommandBuffer

	// darray
	ImageAvailableSemaphores []vk.Semaphore

	// darray
	QueueCompleteSemaphores []vk.Semaphore

	InFlightFenceCount uint32
	InFlightFences     []*VulkanFence

	// Holds pointers to fences which exist and are owned elsewhere.
	ImagesInFlight []*VulkanFence

	ImageIndex   uint32
	CurrentFrame uint32

	RecreatingSwapchain bool
}

func (vc *VulkanContext) FindMemoryIndex(typeFilter, propertyFlags uint32) int32 {
	var memoryProperties vk.PhysicalDeviceMemoryProperties
	vk.GetPhysicalDeviceMemoryProperties(vc.Device.PhysicalDevice, &memoryProperties)
	memoryProperties.Deref()

	for i := uint32(0); i < memoryProperties.MemoryTypeCount; i++ {
		// Check each memory type to see if its bit is set to 1.
		memoryProperties.MemoryTypes[i].Deref()
		if (typeFilter&(1<<i)) != 0 && (uint32(memoryProperties.MemoryTypes[i].PropertyFlags)&propertyFlags) == propertyFlags {
			return int32(i)
		}
	}
	core.LogWarn("Unable to find suitable memory type!")
	return -1
}
