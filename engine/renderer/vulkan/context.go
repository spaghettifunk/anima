package vulkan

import (
	vk "github.com/goki/vulkan"
	"github.com/spaghettifunk/anima/engine/core"
	"github.com/spaghettifunk/anima/engine/renderer/metadata"
)

const VULKAN_MAX_REGISTERED_RENDERPASSES uint32 = 31

/**
 * @brief Represents a Vulkan-specific buffer.
 * Used to load data onto the GPU.
 */
type VulkanBuffer struct {
	/** @brief The Handle to the internal buffer. */
	Handle vk.Buffer
	/** @brief The Usage flags. */
	Usage vk.BufferUsageFlags
	/** @brief Indicates if the buffer's memory is currently locked. */
	IsLocked bool
	/** @brief The Memory used by the buffer. */
	Memory vk.DeviceMemory
	/** @brief The memory requirements for this buffer. */
	MemoryRequirements vk.MemoryRequirements
	/** @brief The index of the memory used by the buffer. */
	MemoryIndex int32
	/** @brief The property flags for the memory used by the buffer. */
	MemoryPropertyFlags uint32
}

/**
 * @brief Internal buffer data for geometry. This data gets loaded
 * directly into a buffer.
 */
type VulkanGeometryData struct {
	/** @brief The unique geometry identifier. */
	ID uint32
	/** @brief The geometry Generation. Incremented every time the geometry data changes. */
	Generation uint32
	/** @brief The vertex count. */
	VertexCount uint32
	/** @brief The size of each vertex. */
	VertexElementSize uint32
	/** @brief The offset in bytes in the vertex buffer. */
	VertexBufferOffset uint64
	/** @brief The index count. */
	IndexCount uint32
	/** @brief The size of each index. */
	IndexElementSize uint32
	/** @brief The offset in bytes in the index buffer. */
	IndexBufferOffset uint64
}

type VulkanContext struct {
	/** @brief The time in seconds since the last frame. */
	FrameDeltaTime float32
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

	Swapchain *VulkanSwapchain

	// TODO: not sure about the type here
	RenderPassTableBlock interface{}
	RenderPassTable      map[string]uint32
	/** @brief Registered renderpasses. */
	RegisteredPasses []*metadata.RenderPass
	/** @brief The object vertex buffer, used to hold geometry vertices. */
	ObjectVertexBuffer *metadata.RenderBuffer
	/** @brief The object index buffer, used to hold geometry indices. */
	ObjectIndexBuffer *metadata.RenderBuffer

	GraphicsCommandBuffers   []*VulkanCommandBuffer
	ImageAvailableSemaphores []vk.Semaphore
	QueueCompleteSemaphores  []vk.Semaphore

	InFlightFenceCount uint32
	InFlightFences     []vk.Fence

	// Holds pointers to fences which exist and are owned elsewhere.
	ImagesInFlight []vk.Fence

	ImageIndex   uint32
	CurrentFrame uint32

	RecreatingSwapchain bool

	/** @brief The A collection of loaded Geometries. @todo TODO: make dynamic */
	Geometries []*VulkanGeometryData

	/** @brief Render targets used for world rendering. @note One per frame. */
	WorldRenderTargets [3]metadata.RenderTarget

	/** @brief Indicates if multi-threading is supported by this device. */
	MultithreadingEnabled bool

	OnRenderTargetRefreshRequired metadata.OnRenderTargetRefreshRequired
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
