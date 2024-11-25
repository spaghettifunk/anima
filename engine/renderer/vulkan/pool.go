package vulkan

import "sync"

type LockGroup string

const (
	SamplerManagement         LockGroup = "sampler_management"
	ResourceManagement        LockGroup = "resource_management"
	CommandBufferManagement   LockGroup = "command_buffer_management"	
	RenderpassManagement      LockGroup = "renderpass_management"	
	BufferManagement          LockGroup = "buffer_management"
	ImageManagement           LockGroup = "image_management"
	DeviceManagement          LockGroup = "device_management"
	CommandPoolManagement     LockGroup = "command_pool_management"
	QueueManagement           LockGroup = "queue_management"
	PipelineManagement        LockGroup = "pipeline_management"
	MemoryManagement          LockGroup = "memory_management"
	ShaderManagement          LockGroup = "shader_management"
	SynchronizationManagement LockGroup = "synchronization_management"
	SwapchainManagement       LockGroup = "swapchain_management"
	InstanceManagement        LockGroup = "instance_management"
)

// Mutex pool
type VulkanLockPool struct {
	locks map[LockGroup]*sync.Mutex
	mu    sync.Mutex // Protects access to the locks map

	queueMutexes map[uint32]*sync.Mutex // Queue family index as key
}

// Initialize the VulkanSync object
func NewVulkanLockPool() *VulkanLockPool {
	return &VulkanLockPool{
		locks: make(map[LockGroup]*sync.Mutex),
		queueMutexes: make(map[uint32]*sync.Mutex),
	}
}

// Get or create a mutex for a specific group
func (vs *VulkanLockPool) setLock(group LockGroup) *sync.Mutex {
	vs.mu.Lock()
	defer vs.mu.Unlock()

	// Create a new mutex if it doesn't exist
	if _, exists := vs.locks[group]; !exists {
		vs.locks[group] = &sync.Mutex{}
	}
	vs.locks[group].Lock()

	return vs.locks[group]
}

func (vs *VulkanLockPool) SafeCall(group LockGroup, fn func() error) error {
	l := vs.setLock(group)
	defer l.Unlock()

	return fn()
}

func (vs *VulkanLockPool) SetQueueFamily(index uint32) {
	vs.mu.Lock()
	defer vs.mu.Unlock()

	// Create a new mutex if it doesn't exist
	if _, exists := vs.queueMutexes[index]; !exists {
		vs.queueMutexes[index] = &sync.Mutex{}
	}
}

func (vs *VulkanLockPool) SafeQueueCall(queueFamilyIndex uint32, fn func() error) error {
	vs.mu.Lock()
	defer vs.mu.Unlock()

	l := vs.queueMutexes[queueFamilyIndex]
	l.Lock()
	defer l.Unlock()

	return fn()
}
