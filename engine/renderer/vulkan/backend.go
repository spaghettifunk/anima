package vulkan

import (
	"fmt"
	m "math"
	"runtime"
	"unsafe"

	"github.com/go-gl/glfw/v3.3/glfw"
	vk "github.com/goki/vulkan"
	"github.com/spaghettifunk/anima/engine/assets"
	"github.com/spaghettifunk/anima/engine/core"
	"github.com/spaghettifunk/anima/engine/math"
	"github.com/spaghettifunk/anima/engine/platform"
	"github.com/spaghettifunk/anima/engine/renderer/metadata"
)

const (
	// The index of the global descriptor set.
	DESC_SET_INDEX_GLOBAL uint32 = 0
	// The index of the instance descriptor set.
	DESC_SET_INDEX_INSTANCE uint32 = 1
)

type VulkanRenderer struct {
	platform *platform.Platform
	context  *VulkanContext

	assetManager   *assets.AssetManager
	defaultTexture *metadata.DefaultTexture

	FrameNumber       uint64
	FramebufferWidth  uint32
	FramebufferHeight uint32

	debug bool
}

var lockPool *VulkanLockPool

func New(p *platform.Platform, am *assets.AssetManager) *VulkanRenderer {
	defaultTextures := metadata.NewDefaultTexture()
	defaultTextures.CreateSkeletonTextures()
	lockPool = NewVulkanLockPool()

	return &VulkanRenderer{
		platform:       p,
		assetManager:   am,
		defaultTexture: defaultTextures,
		FrameNumber:    0,
		context: &VulkanContext{
			Geometries:                    make([]*VulkanGeometryData, VULKAN_MAX_GEOMETRY_COUNT),
			FramebufferWidth:              0,
			FramebufferHeight:             0,
			Allocator:                     nil,
			FramebufferSizeGeneration:     0,
			FramebufferSizeLastGeneration: 0,
		},
		FramebufferWidth:  0,
		FramebufferHeight: 0,
		debug:             true,
	}
}

func (vr *VulkanRenderer) Initialize(config *metadata.RendererBackendConfig, windowRenderTargetCount *uint8) error {
	procAddr := glfw.GetVulkanGetInstanceProcAddress()
	if procAddr == nil {
		return fmt.Errorf("GetInstanceProcAddress is nil")
	}
	vk.SetGetInstanceProcAddr(procAddr)

	if err := vk.Init(); err != nil {
		core.LogFatal("failed to initialize vk: %s", err)
		return err
	}

	// TODO: custom allocator.
	vr.context.Allocator = nil

	vr.context.FramebufferWidth = 1280
	vr.context.FramebufferHeight = 720

	// Setup Vulkan instance.
	appInfo := &vk.ApplicationInfo{
		SType:              vk.StructureTypeApplicationInfo,
		ApiVersion:         uint32(vk.MakeVersion(1, 2, 0)),
		ApplicationVersion: uint32(vk.MakeVersion(1, 0, 0)),
		PApplicationName:   VulkanSafeString(config.ApplicationName),
		PEngineName:        VulkanSafeString("Anima Engine"),
		EngineVersion:      uint32(vk.MakeVersion(1, 0, 0)),
	}
	appInfo.Deref()

	createInfo := vk.InstanceCreateInfo{
		SType:            vk.StructureTypeInstanceCreateInfo,
		PApplicationInfo: appInfo,
	}

	// Obtain a list of required extensions
	requiredExtensions := []string{"VK_KHR_surface"} // Generic surface extension
	en := vr.platform.GetRequiredExtensionNames()
	requiredExtensions = append(requiredExtensions, en...)

	if runtime.GOOS == "darwin" {
		requiredExtensions = append(requiredExtensions,
			"VK_KHR_portability_enumeration",
			"VK_KHR_get_physical_device_properties2",
		)
	}

	if vr.debug {
		requiredExtensions = append(requiredExtensions, vk.ExtDebugUtilsExtensionName, vk.ExtDebugReportExtensionName) // debug utilities
		core.LogInfo("Required extensions:")
		for i := 0; i < len(requiredExtensions); i++ {
			core.LogInfo(requiredExtensions[i])
		}
	}

	createInfo.EnabledExtensionCount = uint32(len(requiredExtensions))
	createInfo.PpEnabledExtensionNames = VulkanSafeStrings(requiredExtensions)

	// Validation layers.
	requiredValidationLayerNames := []string{}

	// If validation should be done, get a list of the required validation layert names
	// and make sure they exist. Validation layers should only be enabled on non-release builds.
	if vr.debug {
		core.LogInfo("Validation layers enabled. Enumerating...")

		// The list of validation layers required.
		requiredValidationLayerNames = []string{"VK_LAYER_KHRONOS_validation"}

		if runtime.GOOS == "darwin" {
			createInfo.Flags |= vk.InstanceCreateFlags(vk.InstanceCreateEnumeratePortabilityBit)
		}

		// Obtain a list of available validation layers
		var availableLayerCount uint32
		if res := vk.EnumerateInstanceLayerProperties(&availableLayerCount, nil); !VulkanResultIsSuccess(res) {
			err := fmt.Errorf("failed to enumerate the instance layer properties with error %s", VulkanResultString(res, true))
			return err
		}

		availableLayers := make([]vk.LayerProperties, availableLayerCount)
		if res := vk.EnumerateInstanceLayerProperties(&availableLayerCount, availableLayers); !VulkanResultIsSuccess(res) {
			err := fmt.Errorf("failed to enumerate the instance layer properties with error %s", VulkanResultString(res, true))
			return err
		}

		// Verify all required layers are available.
		for i := range requiredValidationLayerNames {
			core.LogInfo("Searching for layer: %s...", requiredValidationLayerNames[i])
			found := false
			for j := range availableLayers {
				availableLayers[j].Deref()
				core.LogInfo("Available Layer: `%s`", string(availableLayers[j].LayerName[:]))
				end := FindFirstZeroInByteArray(availableLayers[j].LayerName[:])
				if requiredValidationLayerNames[i] == vk.ToString(availableLayers[j].LayerName[:end+1]) {
					found = true
					core.LogInfo("Found.")
					break
				}
			}

			if !found {
				core.LogFatal("Required validation layer is missing: %s", requiredValidationLayerNames[i])
				return nil
			}
		}
		core.LogInfo("All required validation layers are present.")
	}

	createInfo.EnabledLayerCount = uint32(len(requiredValidationLayerNames))
	createInfo.PpEnabledLayerNames = VulkanSafeStrings(requiredValidationLayerNames)
	createInfo.Deref()

	var instance vk.Instance
	if err := lockPool.SafeCall(InstanceManagement, func() error {
		if res := vk.CreateInstance(&createInfo, vr.context.Allocator, &instance); !VulkanResultIsSuccess(res) {
			err := fmt.Errorf("failed in creating the Vulkan Instance with error `%s`", VulkanResultString(res, true))
			return err
		}
		return nil
	}); err != nil {
		return err
	}
	vr.context.Instance = instance

	if err := vk.InitInstance(vr.context.Instance); err != nil {
		return err
	}

	core.LogInfo("Vulkan Instance created.")

	// Debugger
	if vr.debug {
		core.LogDebug("Creating Vulkan debugger...")

		debugCreateInfo := vk.DebugReportCallbackCreateInfo{
			SType:       vk.StructureTypeDebugReportCallbackCreateInfo,
			Flags:       vk.DebugReportFlags(vk.DebugReportErrorBit | vk.DebugReportWarningBit | vk.DebugReportInformationBit),
			PfnCallback: dbgCallbackFunc,
			PNext:       nil,
		}

		var dbg vk.DebugReportCallback
		if result := vk.CreateDebugReportCallback(vr.context.Instance, &debugCreateInfo, nil, &dbg); !VulkanResultIsSuccess(result) {
			err := fmt.Errorf("vk.CreateDebugReportCallback failed with %s", VulkanResultString(result, true))
			return err
		}
		vr.context.debugMessenger = dbg

		core.LogDebug("Vulkan debugger created.")
	}

	// Surface
	core.LogDebug("Creating Vulkan surface...")
	surface := vr.createVulkanSurface()
	if surface == 0 {
		err := fmt.Errorf("failed to create platform surface")
		return err
	}
	vr.context.Surface = vk.SurfaceFromPointer(surface)
	core.LogDebug("Vulkan surface created.")

	// Device creation
	if err := DeviceCreate(vr.context); err != nil {
		core.LogError("Failed to create device!")
		return err
	}

	// create default textures
	vr.TextureCreate(vr.defaultTexture.TexturePixels, vr.defaultTexture.DefaultTexture)
	vr.TextureCreate(vr.defaultTexture.SpecularTexturePixels, vr.defaultTexture.DefaultSpecularTexture)
	vr.TextureCreate(vr.defaultTexture.DiffuseTexturePixels, vr.defaultTexture.DefaultDiffuseTexture)
	vr.TextureCreate(vr.defaultTexture.NormalTexturePixels, vr.defaultTexture.DefaultNormalTexture)

	// Swapchain
	sc, err := SwapchainCreate(vr.context, vr.context.FramebufferWidth, vr.context.FramebufferHeight)
	if err != nil {
		return nil
	}
	vr.context.Swapchain = sc

	*windowRenderTargetCount = uint8(vr.context.Swapchain.ImageCount)

	// Create command buffers.
	vr.createCommandBuffers()

	// Create sync objects.
	vr.context.ImageAvailableSemaphores = make([]vk.Semaphore, vr.context.Swapchain.MaxFramesInFlight)
	vr.context.QueueCompleteSemaphores = make([]vk.Semaphore, vr.context.Swapchain.MaxFramesInFlight)
	vr.context.InFlightFences = make([]vk.Fence, vr.context.Swapchain.MaxFramesInFlight)

	for i := 0; i < int(vr.context.Swapchain.MaxFramesInFlight); i++ {
		semaphoreCreateInfo := vk.SemaphoreCreateInfo{
			SType: vk.StructureTypeSemaphoreCreateInfo,
		}
		semaphoreCreateInfo.Deref()

		var sem vk.Semaphore
		if err := lockPool.SafeCall(SynchronizationManagement, func() error {
			if res := vk.CreateSemaphore(vr.context.Device.LogicalDevice, &semaphoreCreateInfo, vr.context.Allocator, &sem); !VulkanResultIsSuccess(res) {
				err := fmt.Errorf("failed to create semaphore on image available")
				return err
			}
			return nil
		}); err != nil {
			return err
		}
		vr.context.ImageAvailableSemaphores[i] = sem

		var sem2 vk.Semaphore
		if err := lockPool.SafeCall(SynchronizationManagement, func() error {
			if res := vk.CreateSemaphore(vr.context.Device.LogicalDevice, &semaphoreCreateInfo, vr.context.Allocator, &sem2); !VulkanResultIsSuccess(res) {
				err := fmt.Errorf("failed to create semaphore on queue complete")
				return err
			}
			return nil
		}); err != nil {
			return err
		}
		vr.context.QueueCompleteSemaphores[i] = sem2

		// Create the fence in a signaled state, indicating that the first frame has already been "rendered".
		// This will prevent the application from waiting indefinitely for the first frame to render since it
		// cannot be rendered until a frame is "rendered" before it.
		fenceCreateInfo := vk.FenceCreateInfo{
			SType: vk.StructureTypeFenceCreateInfo,
			Flags: vk.FenceCreateFlags(vk.FenceCreateSignaledBit),
		}

		var pFence vk.Fence
		if err := lockPool.SafeCall(SynchronizationManagement, func() error {
			if res := vk.CreateFence(vr.context.Device.LogicalDevice, &fenceCreateInfo, vr.context.Allocator, &pFence); !VulkanResultIsSuccess(res) {
				err := fmt.Errorf("failed to create fence with error %s", VulkanResultString(res, true))
				return err
			}
			return nil
		}); err != nil {
			return err
		}
		vr.context.InFlightFences[i] = pFence
	}

	// In flight fences should not yet exist at this point, so clear the list. These are stored in pointers
	// because the initial state should be 0, and will be 0 when not in use. Acutal fences are not owned
	// by this list.
	vr.context.ImagesInFlight = make([]vk.Fence, vr.context.Swapchain.ImageCount)
	for i := 0; i < int(vr.context.Swapchain.ImageCount); i++ {
		vr.context.ImagesInFlight[i] = nil
	}

	// Create buffers

	// Geometry vertex buffer
	vertex_buffer_size := 2 * 1024 * 1024
	vr.context.ObjectVertexBuffer, err = vr.RenderBufferCreate(metadata.RENDERBUFFER_TYPE_VERTEX, uint64(vertex_buffer_size))
	if err != nil {
		err := fmt.Errorf("error creating vertex buffer")
		return err
	}
	if err := vr.RenderBufferBind(vr.context.ObjectVertexBuffer, 0); err != nil {
		core.LogError("error binding vertex buffer")
		return err
	}

	// Geometry index buffer
	index_buffer_size := 2 * 1024 * 1024
	vr.context.ObjectIndexBuffer, err = vr.RenderBufferCreate(metadata.RENDERBUFFER_TYPE_INDEX, uint64(index_buffer_size))
	if err != nil {
		err := fmt.Errorf("error creating index buffer")
		return err
	}
	if err := vr.RenderBufferBind(vr.context.ObjectIndexBuffer, 0); err != nil {
		core.LogError("error binding index buffer")
		return err
	}

	core.LogInfo("Vulkan renderer initialized successfully.")

	return nil
}

func (vr *VulkanRenderer) Shutdow() error {
	if err := lockPool.SafeCall(DeviceManagement, func() error {
		if res := vk.DeviceWaitIdle(vr.context.Device.LogicalDevice); !VulkanResultIsSuccess(res) {
			err := fmt.Errorf("device wait idle failed with error %s", VulkanResultString(res, true))
			return err
		}
		return nil
	}); err != nil {
		return err
	}

	// Destroy in the opposite order of creation.
	// Destroy buffers
	vr.RenderBufferDestroy(vr.context.ObjectVertexBuffer)
	vr.RenderBufferDestroy(vr.context.ObjectIndexBuffer)

	// Sync objects
	for i := 0; i < int(vr.context.Swapchain.MaxFramesInFlight); i++ {
		if vr.context.ImageAvailableSemaphores[i] != vk.NullSemaphore {
			if err := lockPool.SafeCall(SynchronizationManagement, func() error {
				vk.DestroySemaphore(vr.context.Device.LogicalDevice, vr.context.ImageAvailableSemaphores[i], vr.context.Allocator)
				return nil
			}); err != nil {
				return err
			}
			vr.context.ImageAvailableSemaphores[i] = vk.NullSemaphore
		}
		if vr.context.QueueCompleteSemaphores[i] != vk.NullSemaphore {
			if err := lockPool.SafeCall(SynchronizationManagement, func() error {
				vk.DestroySemaphore(vr.context.Device.LogicalDevice, vr.context.QueueCompleteSemaphores[i], vr.context.Allocator)
				return nil
			}); err != nil {
				return err
			}
			vr.context.QueueCompleteSemaphores[i] = vk.NullSemaphore
		}
		if err := lockPool.SafeCall(SynchronizationManagement, func() error {
			vk.DestroyFence(vr.context.Device.LogicalDevice, vr.context.InFlightFences[i], vr.context.Allocator)
			return nil
		}); err != nil {
			return err
		}
		vr.context.InFlightFences[i] = vk.NullFence
	}

	vr.context.ImageAvailableSemaphores = nil
	vr.context.QueueCompleteSemaphores = nil

	// Command buffers
	for i := 0; i < int(vr.context.Swapchain.ImageCount); i++ {
		if vr.context.GraphicsCommandBuffers[i].Handle != nil {
			vr.context.GraphicsCommandBuffers[i].Free(vr.context, vr.context.Device.GraphicsCommandPool)
			vr.context.GraphicsCommandBuffers[i].Handle = nil
		}
	}
	vr.context.GraphicsCommandBuffers = nil

	// Swapchain
	vr.context.Swapchain.SwapchainDestroy(vr.context)

	core.LogDebug("Destroying Vulkan device...")
	DeviceDestroy(vr.context)

	core.LogDebug("Destroying Vulkan surface...")
	if vr.context.Surface != vk.NullSurface {
		vk.DestroySurface(vr.context.Instance, vr.context.Surface, vr.context.Allocator)
		vr.context.Surface = vk.NullSurface
	}

	if vr.debug {
		core.LogDebug("Destroying Vulkan debugger...")
		if vr.context.debugMessenger != vk.NullDebugReportCallback {
			vk.DestroyDebugReportCallback(vr.context.Instance, vr.context.debugMessenger, vr.context.Allocator)
			vr.context.debugMessenger = nil
		}
	}

	core.LogDebug("Destroying Vulkan instance...")
	if err := lockPool.SafeCall(DeviceManagement, func() error {
		vk.DestroyInstance(vr.context.Instance, vr.context.Allocator)
		return nil
	}); err != nil {
		return err
	}

	vr.context.Instance = nil

	// Destroy the allocator callbacks if set.
	if vr.context.Allocator != nil {
		vr.context.Allocator = nil
	}

	return nil
}

func (vr *VulkanRenderer) Resized(width, height uint32) error {
	// Update the "framebuffer size generation", a counter which indicates when the
	// framebuffer size has been updated.
	vr.context.FramebufferWidth = width
	vr.context.FramebufferHeight = height
	vr.context.FramebufferSizeGeneration++

	core.LogInfo("Vulkan renderer backend.resized: w/h/gen: %d/%d/%d", width, height, vr.context.FramebufferSizeGeneration)
	return nil
}

func (vr *VulkanRenderer) BeginFrame(deltaTime float64) error {
	vr.context.FrameDeltaTime = float32(deltaTime)
	device := vr.context.Device

	// Check if recreating swap chain and boot out.
	if vr.context.RecreatingSwapchain {
		if err := lockPool.SafeCall(DeviceManagement, func() error {
			if res := vk.DeviceWaitIdle(device.LogicalDevice); !VulkanResultIsSuccess(res) {
				err := fmt.Errorf("device wait idle failed with error %s", VulkanResultString(res, true))
				return err
			}
			return nil
		}); err != nil {
			return err
		}
		return core.ErrSwapchainBooting
	}

	// Check if the framebuffer has been resized. If so, a new swapchain must be created.
	if vr.context.FramebufferSizeGeneration != vr.context.FramebufferSizeLastGeneration {
		if err := lockPool.SafeCall(DeviceManagement, func() error {
			if res := vk.DeviceWaitIdle(device.LogicalDevice); !VulkanResultIsSuccess(res) {
				err := fmt.Errorf("device wait idle failed with error %s", VulkanResultString(res, true))
				return err
			}
			return nil
		}); err != nil {
			return err
		}

		// If the swapchain recreation failed (because, for example, the window was minimized),
		// boot out before unsetting the flag.
		if err := vr.recreateSwapchain(); err != nil {
			return err
		}
		return core.ErrSwapchainBooting
	}

	// Wait for the execution of the current frame to complete. The fence being free will allow this one to move on.
	f := vr.context.InFlightFences[vr.context.CurrentFrame]

	inFlightsFences := []vk.Fence{f}
	if err := lockPool.SafeCall(SynchronizationManagement, func() error {
		result := vk.WaitForFences(vr.context.Device.LogicalDevice, 1, inFlightsFences, vk.True, vk.MaxUint64-1)
		if !VulkanResultIsSuccess(result) {
			err := fmt.Errorf("func BeginFram In-flight fence wait failure! error: %s", VulkanResultString(result, true))
			return err
		}
		return nil
	}); err != nil {
		return err
	}

	// Acquire the next image from the swap chain. Pass along the semaphore that should signaled when this completes.
	// This same semaphore will later be waited on by the queue submission to ensure this image is available.
	imageIndex, ok, err := vr.context.Swapchain.SwapchainAcquireNextImageIndex(vr.context, vk.MaxUint64-1, vr.context.ImageAvailableSemaphores[vr.context.CurrentFrame], vk.NullFence)
	if !ok && err != nil {
		core.LogError("failed to swapchain aquire next image index")
		return err
	}
	vr.context.ImageIndex = imageIndex

	// Begin recording commands.
	commandBuffer := vr.context.GraphicsCommandBuffers[vr.context.ImageIndex]
	commandBuffer.Reset()
	commandBuffer.Begin(false, false, false)

	vr.context.ViewportRect = math.NewVec4(0.0, float32(vr.context.FramebufferHeight), float32(vr.context.FramebufferWidth), -float32(vr.context.FramebufferHeight))
	vr.SetViewport()

	vr.context.ScissorRect = math.NewVec4(0, 0, float32(vr.context.FramebufferWidth), float32(vr.context.FramebufferHeight))
	vr.SetScissor()

	return nil
}

func (vr *VulkanRenderer) EndFrame(deltaTime float64) error {
	commandBuffer := vr.context.GraphicsCommandBuffers[vr.context.ImageIndex]

	commandBuffer.End()

	// Make sure the previous frame is not using this image (i.e. its fence is being waited on)
	if vr.context.ImagesInFlight[vr.context.ImageIndex] != vk.NullFence { // was frame
		if err := lockPool.SafeCall(SynchronizationManagement, func() error {
			result := vk.WaitForFences(vr.context.Device.LogicalDevice, 1, []vk.Fence{vr.context.ImagesInFlight[vr.context.ImageIndex]}, vk.True, m.MaxUint64)
			if !VulkanResultIsSuccess(result) {
				err := fmt.Errorf("func EndFrame vkWaitForFences error: %s", VulkanResultString(result, true))
				return err
			}
			return nil
		}); err != nil {
			return err
		}
	}

	// Mark the image fence as in-use by this frame.
	vr.context.ImagesInFlight[vr.context.ImageIndex] = vr.context.InFlightFences[vr.context.CurrentFrame]

	// Reset the fence for use on the next frame
	if err := lockPool.SafeCall(SynchronizationManagement, func() error {
		if res := vk.ResetFences(vr.context.Device.LogicalDevice, 1, []vk.Fence{vr.context.InFlightFences[vr.context.CurrentFrame]}); !VulkanResultIsSuccess(res) {
			err := fmt.Errorf("func EndFrame failed to reset fences with error %s", VulkanResultString(res, true))
			return err
		}
		return nil
	}); err != nil {
		return err
	}

	// Submit the queue and wait for the operation to complete.
	// Begin queue submission
	submitInfo := vk.SubmitInfo{
		SType: vk.StructureTypeSubmitInfo,
		// Command buffer(s) to be executed.
		CommandBufferCount: 1,
		PCommandBuffers:    []vk.CommandBuffer{commandBuffer.Handle},
		// The semaphore(s) to be signaled when the queue is complete.
		SignalSemaphoreCount: 1,
		PSignalSemaphores:    []vk.Semaphore{vr.context.QueueCompleteSemaphores[vr.context.CurrentFrame]},
		// Wait semaphore ensures that the operation cannot begin until the image is available.
		WaitSemaphoreCount: 1,
		PWaitSemaphores:    []vk.Semaphore{vr.context.ImageAvailableSemaphores[vr.context.CurrentFrame]},
	}
	submitInfo.Deref()

	// Each semaphore waits on the corresponding pipeline stage to complete. 1:1 ratio.
	// VK_PIPELINE_STAGE_COLOR_ATTACHMENT_OUTPUT_BIT prevents subsequent colour attachment
	// writes from executing until the semaphore signals (i.e. one frame is presented at a time)
	flags := vk.PipelineStageFlags(vk.PipelineStageColorAttachmentOutputBit)
	submitInfo.PWaitDstStageMask = []vk.PipelineStageFlags{flags}

	if err := lockPool.SafeQueueCall(vr.context.Device.GraphicsQueueIndex, func() error {
		if result := vk.QueueSubmit(vr.context.Device.GraphicsQueue, 1, []vk.SubmitInfo{submitInfo}, vr.context.InFlightFences[vr.context.CurrentFrame]); !VulkanResultIsSuccess(result) {
			err := fmt.Errorf("vk.QueueSubmit failed with result: %s", VulkanResultString(result, true))
			return err
		}
		return nil
	}); err != nil {
		return err
	}

	commandBuffer.UpdateSubmitted()
	// End queue submission

	// Give the image back to the swapchain.
	return vr.context.Swapchain.SwapchainPresent(
		vr.context,
		vr.context.Device.GraphicsQueue,
		vr.context.Device.PresentQueue,
		vr.context.QueueCompleteSemaphores[vr.context.CurrentFrame],
		vr.context.ImageIndex)
}

func (vr *VulkanRenderer) SetViewport() error {
	// Dynamic state
	viewport := vk.Viewport{
		X:        vr.context.ViewportRect.X,
		Y:        vr.context.ViewportRect.Y,
		Width:    vr.context.ViewportRect.Z,
		Height:   vr.context.ViewportRect.W,
		MinDepth: 0.0,
		MaxDepth: 1.0,
	}
	commandBuffer := vr.context.GraphicsCommandBuffers[vr.context.ImageIndex]

	if err := lockPool.SafeCall(CommandBufferManagement, func() error {
		vk.CmdSetViewport(commandBuffer.Handle, 0, 1, []vk.Viewport{viewport})
		return nil
	}); err != nil {
		return err
	}
	return nil
}

func (vr *VulkanRenderer) ResetViewport() {
	vr.SetViewport()
}

func (vr *VulkanRenderer) SetScissor() error {
	scissor := vk.Rect2D{
		Offset: vk.Offset2D{
			X: int32(vr.context.ScissorRect.X),
			Y: int32(vr.context.ScissorRect.Y),
		},
		Extent: vk.Extent2D{
			Width:  uint32(vr.context.ScissorRect.Z),
			Height: uint32(vr.context.ScissorRect.W),
		},
	}
	commandBuffer := vr.context.GraphicsCommandBuffers[vr.context.ImageIndex]

	if err := lockPool.SafeCall(CommandBufferManagement, func() error {
		vk.CmdSetScissor(commandBuffer.Handle, 0, 1, []vk.Rect2D{scissor})
		return nil
	}); err != nil {
		return err
	}
	return nil
}

func (vr *VulkanRenderer) ResetScissor() {
	vr.SetScissor()
}

func (vr *VulkanRenderer) createCommandBuffers() error {
	if len(vr.context.GraphicsCommandBuffers) == 0 {
		vr.context.GraphicsCommandBuffers = make([]*VulkanCommandBuffer, vr.context.Swapchain.ImageCount)
	}
	for i := 0; i < int(vr.context.Swapchain.ImageCount); i++ {
		if vr.context.GraphicsCommandBuffers[i] != nil && vr.context.GraphicsCommandBuffers[i].Handle != nil {
			vr.context.GraphicsCommandBuffers[i].Free(vr.context, vr.context.Device.GraphicsCommandPool)
		}
		vr.context.GraphicsCommandBuffers[i] = nil
		cb, err := NewVulkanCommandBuffer(vr.context, vr.context.Device.GraphicsCommandPool, true)
		if err != nil {
			return err
		}
		vr.context.GraphicsCommandBuffers[i] = cb
	}

	core.LogDebug("Vulkan command buffers created.")
	return nil
}

func (vr *VulkanRenderer) recreateSwapchain() error {
	// If already being recreated, do not try again.
	if vr.context.RecreatingSwapchain {
		core.LogDebug("recreate_swapchain called when already recreating. Booting.")
		return nil
	}

	// Detect if the window is too small to be drawn to
	if vr.context.FramebufferWidth == 0 || vr.context.FramebufferHeight == 0 {
		core.LogDebug("recreate_swapchain called when window is < 1 in a dimension. Booting.")
		return nil
	}

	// Mark as recreating if the dimensions are valid.
	vr.context.RecreatingSwapchain = true

	// Wait for any operations to complete.
	if err := lockPool.SafeCall(DeviceManagement, func() error {
		if res := vk.DeviceWaitIdle(vr.context.Device.LogicalDevice); !VulkanResultIsSuccess(res) {
			err := fmt.Errorf("device wait idle failed with error %s", VulkanResultString(res, true))
			return err
		}
		return nil
	}); err != nil {
		return err
	}

	// Clear these out just in case.
	for i := 0; i < int(vr.context.Swapchain.ImageCount); i++ {
		vr.context.ImagesInFlight[i] = nil
	}

	// Requery support
	DeviceQuerySwapchainSupport(vr.context.Device.PhysicalDevice, vr.context.Surface, vr.context.Device.SwapchainSupport)
	DeviceDetectDepthFormat(vr.context.Device)

	sc, err := vr.context.Swapchain.SwapchainRecreate(vr.context, vr.context.FramebufferWidth, vr.context.FramebufferHeight)
	if err != nil {
		return err
	}
	vr.context.Swapchain = sc

	// Update framebuffer size generation.
	vr.context.FramebufferSizeLastGeneration = vr.context.FramebufferSizeGeneration

	// cleanup swapchain
	for i := uint32(0); i < vr.context.Swapchain.ImageCount; i++ {
		vr.context.GraphicsCommandBuffers[i].Free(vr.context, vr.context.Device.GraphicsCommandPool)
	}

	eventContext := core.EventContext{
		Type: core.EVENT_CODE_DEFAULT_RENDERTARGET_REFRESH_REQUIRED,
	}
	core.EventFire(eventContext)

	vr.createCommandBuffers()

	// Clear the recreating flag.
	vr.context.RecreatingSwapchain = false

	return nil
}

func (vr *VulkanRenderer) createVulkanSurface() uintptr {
	surface, err := vr.platform.Window.CreateWindowSurface(vr.context.Instance, nil)
	if err != nil {
		core.LogFatal("Vulkan surface creation failed.")
		return 0
	}
	return surface
}

func (vr *VulkanRenderer) CreateGeometry(geometry *metadata.Geometry, vertex_size, vertexCount uint32, vertices interface{}, index_size uint32, indexCount uint32, indices []uint32) error {
	if vertexCount == 0 || vertices == nil {
		err := fmt.Errorf("vulkan_renderer_create_geometry requires vertex data, and none was supplied. VertexCount=%d, vertices=%p", vertexCount, vertices)
		return err
	}

	var internalData *VulkanGeometryData
	// Mark all geometries as invalid
	for i := uint32(0); i < VULKAN_MAX_GEOMETRY_COUNT; i++ {
		if vr.context.Geometries[i] == nil {
			vr.context.Geometries[i] = &VulkanGeometryData{
				ID: metadata.InvalidID,
			}
		}
	}
	for i := uint32(0); i < VULKAN_MAX_GEOMETRY_COUNT; i++ {
		if vr.context.Geometries[i].ID == metadata.InvalidID {
			// Found a free index.
			geometry.InternalID = i
			vr.context.Geometries[i].ID = i
			internalData = vr.context.Geometries[i]
			break
		}
	}
	if internalData == nil {
		err := fmt.Errorf("vulkan_renderer_create_geometry failed to find a free index for a new geometry upload. Adjust config to allow for more")
		return err
	}

	// Vertex data.
	internalData.VertexCount = vertexCount
	internalData.VertexElementSize = uint32(unsafe.Sizeof(math.Vertex3D{}))
	totalSize := uint64(vertexCount * vertex_size)

	// Load the data.
	if err := vr.RenderBufferLoadRange(vr.context.ObjectVertexBuffer, internalData.VertexBufferOffset, totalSize, vertices); err != nil {
		core.LogError("vulkan_renderer_create_geometry failed to upload to the vertex buffer!")
		return err
	}

	// Index data, if applicable
	if indexCount > 0 && len(indices) > 0 {
		internalData.IndexCount = indexCount
		internalData.IndexElementSize = uint32(unsafe.Sizeof(uint32(1)))
		totalSize = uint64(indexCount * index_size)

		if err := vr.RenderBufferLoadRange(vr.context.ObjectIndexBuffer, internalData.IndexBufferOffset, totalSize, indices); err != nil {
			core.LogError("vulkan_renderer_create_geometry failed to upload to the index buffer!")
			return err
		}
	}

	if internalData.Generation == metadata.InvalidID {
		internalData.Generation = 0
	} else {
		internalData.Generation++
	}

	return nil
}

func (vr *VulkanRenderer) TextureCreate(pixels []uint8, texture *metadata.Texture) error {
	// Internal data creation.
	// TODO: Use an allocator for this.
	texture.InternalData = &VulkanImage{}

	cubeVal := uint32(1)
	if texture.TextureType == metadata.TextureTypeCube {
		cubeVal = 6
	}
	size := texture.Width * texture.Height * uint32(texture.ChannelCount) * cubeVal * 2 // * 2 is a test

	// NOTE: Assumes 8 bits per channel.
	imageFormat := vk.FormatR8g8b8a8Unorm

	// NOTE: Lots of assumptions here, different texture types will require
	// different options here.
	image, err := ImageCreate(
		vr.context,
		texture.TextureType,
		texture.Width,
		texture.Height,
		imageFormat,
		vk.ImageTilingOptimal,
		vk.ImageUsageFlags(vk.ImageUsageTransferSrcBit)|vk.ImageUsageFlags(vk.ImageUsageTransferDstBit)|vk.ImageUsageFlags(vk.ImageUsageSampledBit)|vk.ImageUsageFlags(vk.ImageUsageColorAttachmentBit),
		vk.MemoryPropertyFlags(vk.MemoryPropertyDeviceLocalBit),
		true,
		vk.ImageAspectFlags(vk.ImageAspectColorBit),
	)
	if err != nil {
		return err
	}

	texture.InternalData = image

	// Load the data.
	vr.TextureWriteData(texture, 0, size, pixels)

	texture.Generation++

	return nil
}

func (vr *VulkanRenderer) TextureDestroy(texture *metadata.Texture) error {
	if err := lockPool.SafeCall(DeviceManagement, func() error {
		if res := vk.DeviceWaitIdle(vr.context.Device.LogicalDevice); !VulkanResultIsSuccess(res) {
			err := fmt.Errorf("device wait idle failed with error %s", VulkanResultString(res, true))
			return err
		}
		return nil
	}); err != nil {
		return err
	}
	if texture.InternalData != nil {
		image := texture.InternalData.(*VulkanImage)
		if image != nil {
			image.Destroy(vr.context)
			texture.InternalData = nil
		}
	}
	texture = nil

	return nil
}

func (vr *VulkanRenderer) channelCountToFormat(channel_count uint8, default_format vk.Format) vk.Format {
	switch channel_count {
	case 1:
		return vk.FormatR8Unorm
	case 2:
		return vk.FormatR8g8Unorm
	case 3:
		return vk.FormatR8g8b8Unorm
	case 4:
		return vk.FormatR8g8b8a8Unorm
	default:
		return default_format
	}
}

func (vr *VulkanRenderer) TextureCreateWriteable(texture *metadata.Texture) error {
	var usage vk.ImageUsageFlagBits
	var aspect vk.ImageAspectFlagBits
	var imageFormat vk.Format
	if (metadata.TextureFlag(texture.Flags) & metadata.TextureFlagDepth) != 0 {
		usage = vk.ImageUsageDepthStencilAttachmentBit
		aspect = vk.ImageAspectDepthBit
		imageFormat = vr.context.Device.DepthFormat
	} else {
		usage = vk.ImageUsageTransferSrcBit | vk.ImageUsageTransferDstBit | vk.ImageUsageSampledBit | vk.ImageUsageColorAttachmentBit
		aspect = vk.ImageAspectColorBit
		imageFormat = vr.channelCountToFormat(texture.ChannelCount, vk.FormatR8g8b8a8Unorm)
	}
	image, err := ImageCreate(vr.context, texture.TextureType, texture.Width, texture.Height, imageFormat, vk.ImageTilingOptimal, vk.ImageUsageFlags(usage),
		vk.MemoryPropertyFlags(vk.MemoryPropertyDeviceLocalBit), true, vk.ImageAspectFlags(aspect))
	if err != nil {
		return err
	}

	texture.InternalData = image
	texture.Generation++

	return nil
}

func (vr *VulkanRenderer) TextureResize(texture *metadata.Texture, new_width, new_height uint32) error {
	if texture != nil && texture.InternalData != nil {
		// Resizing is really just destroying the old image and creating a new one.
		// Data is not preserved because there's no reliable way to map the old data to the new
		// since the amount of data differs.
		image := texture.InternalData.(*VulkanImage)
		image.Destroy(vr.context)

		image_format := vr.channelCountToFormat(texture.ChannelCount, vk.FormatR8g8b8a8Unorm)

		// TODO: Lots of assumptions here, different texture types will require
		// different options here.
		image, err := ImageCreate(
			vr.context,
			texture.TextureType,
			new_width,
			new_height,
			image_format,
			vk.ImageTilingOptimal,
			vk.ImageUsageFlags(vk.ImageUsageTransferSrcBit)|vk.ImageUsageFlags(vk.ImageUsageTransferDstBit)|vk.ImageUsageFlags(vk.ImageUsageSampledBit)|vk.ImageUsageFlags(vk.ImageUsageColorAttachmentBit),
			vk.MemoryPropertyFlags(vk.MemoryPropertyDeviceLocalBit),
			true,
			vk.ImageAspectFlags(vk.ImageAspectColorBit),
		)
		if err != nil {
			return err
		}
		texture.Generation++
		texture.InternalData = image
	}
	return nil
}

func (vr *VulkanRenderer) TextureWriteData(texture *metadata.Texture, offset, size uint32, pixels []uint8) error {
	image := texture.InternalData.(*VulkanImage)

	imageFormat := vr.channelCountToFormat(texture.ChannelCount, vk.FormatR8g8b8a8Unorm)

	// Create a staging buffer and load data into it.
	staging, err := vr.RenderBufferCreate(metadata.RENDERBUFFER_TYPE_STAGING, uint64(size))
	if err != nil {
		core.LogError("failed to create staging buffer for texture write")
		return err
	}

	if err := vr.RenderBufferBind(staging, 0); err != nil {
		return err
	}

	vr.RenderBufferLoadRange(staging, 0, uint64(size), pixels)

	pool := vr.context.Device.GraphicsCommandPool
	queue := vr.context.Device.GraphicsQueue

	tempBuffer, err := AllocateAndBeginSingleUse(vr.context, pool)
	if err != nil {
		return err
	}

	// Transition the layout from whatever it is currently to optimal for recieving data.
	if err := image.ImageTransitionLayout(
		vr.context,
		texture.TextureType,
		tempBuffer,
		imageFormat,
		vk.ImageLayoutUndefined,
		vk.ImageLayoutTransferDstOptimal); err != nil {
		return err
	}

	// Copy the data from the buffer.
	buff := staging.InternalData.(*VulkanBuffer)
	if err := image.ImageCopyFromBuffer(vr.context, texture.TextureType, buff.Handle, tempBuffer); err != nil {
		return err
	}

	// Transition from optimal for data reciept to shader-read-only optimal layout.
	if err := image.ImageTransitionLayout(
		vr.context,
		texture.TextureType,
		tempBuffer,
		imageFormat,
		vk.ImageLayoutTransferDstOptimal,
		vk.ImageLayoutShaderReadOnlyOptimal,
	); err != nil {
		return err
	}

	if err := tempBuffer.EndSingleUse(vr.context, pool, queue, vr.context.Device.GraphicsQueueIndex); err != nil {
		return err
	}

	if !vr.RenderBufferUnbind(staging) {
		err := fmt.Errorf("func TextureWriteData failed to unbind buffer")
		return err
	}

	vr.RenderBufferDestroy(staging)

	texture.Generation++

	return nil
}

func (vr *VulkanRenderer) DestroyGeometry(geometry *metadata.Geometry) error {
	if geometry != nil && geometry.InternalID != metadata.InvalidID {
		if err := lockPool.SafeCall(DeviceManagement, func() error {
			if res := vk.DeviceWaitIdle(vr.context.Device.LogicalDevice); !VulkanResultIsSuccess(res) {
				err := fmt.Errorf("device wait idle failed with error %s", VulkanResultString(res, true))
				return err
			}
			return nil
		}); err != nil {
			return err
		}

		internalData := vr.context.Geometries[geometry.InternalID]

		// Free vertex data
		if err := vr.RenderBufferFree(vr.context.ObjectVertexBuffer, uint64(internalData.VertexElementSize*internalData.VertexCount), internalData.VertexBufferOffset); err != nil {
			core.LogError("vulkan_renderer_destroy_geometry failed to free vertex buffer range")
			return err
		}

		// Free index data, if applicable
		if internalData.IndexElementSize > 0 {
			if err := vr.RenderBufferFree(vr.context.ObjectIndexBuffer, uint64(internalData.IndexElementSize*internalData.IndexCount), internalData.IndexBufferOffset); err != nil {
				core.LogError("vulkan_renderer_destroy_geometry failed to free index buffer range")
				return err
			}
		}

		// Clean up data.
		internalData.ID = metadata.InvalidID
		internalData.Generation = metadata.InvalidID
	}
	return nil
}

func (vr *VulkanRenderer) DrawGeometry(data *metadata.GeometryRenderData) error {
	// Ignore non-uploaded geometries.
	if data.Geometry != nil && data.Geometry.InternalID == metadata.InvalidID {
		return nil
	}

	bufferData := vr.context.Geometries[data.Geometry.InternalID]
	includesIndexData := bufferData.IndexCount > 0
	if err := vr.RenderBufferDraw(vr.context.ObjectVertexBuffer, bufferData.VertexBufferOffset, bufferData.VertexCount, includesIndexData); err != nil {
		core.LogError("vulkan_renderer_draw_geometry failed to draw vertex buffer")
		return err
	}

	if includesIndexData {
		if err := vr.RenderBufferDraw(vr.context.ObjectIndexBuffer, bufferData.IndexBufferOffset, bufferData.IndexCount, !includesIndexData); err != nil {
			core.LogError("vulkan_renderer_draw_geometry failed to draw index buffer")
			return err
		}
	}
	return nil
}

func (vr *VulkanRenderer) RenderPassCreate(config *metadata.RenderPassConfig) (*metadata.RenderPass, error) {
	if config == nil {
		return nil, fmt.Errorf("renderpass config needs to be a valid pointer")
	}

	if config.RenderTargetCount == 0 {
		return nil, fmt.Errorf("cannot have a renderpass target count of 0")
	}

	pass := &metadata.RenderPass{
		ID:                metadata.InvalidIDUint16,
		InternalData:      &VulkanRenderPass{},
		RenderTargetCount: config.RenderTargetCount,
		Targets:           make([]*metadata.RenderTarget, config.RenderTargetCount),
		ClearColour:       config.ClearColour,
		ClearFlags:        uint8(config.ClearFlags),
		RenderArea:        config.RenderArea,
	}

	// Copy over config for each target.
	for t := 0; t < int(pass.RenderTargetCount); t++ {
		pass.Targets[t] = &metadata.RenderTarget{
			AttachmentCount: uint8(len(config.Target.Attachments)),
			Attachments:     make([]*metadata.RenderTargetAttachment, len(config.Target.Attachments)),
		}
		target := pass.Targets[t]
		// Each attachment for the target.
		for a := 0; a < int(target.AttachmentCount); a++ {
			attachmentConfig := config.Target.Attachments[a]
			target.Attachments[a] = &metadata.RenderTargetAttachment{
				Source:                     attachmentConfig.Source,
				RenderTargetAttachmentType: attachmentConfig.RenderTargetAttachmentType,
				LoadOperation:              attachmentConfig.LoadOperation,
				StoreOperation:             attachmentConfig.StoreOperation,
				Texture:                    nil,
			}
		}
	}

	// Main subpass
	subpass := vk.SubpassDescription{
		PipelineBindPoint: vk.PipelineBindPointGraphics,
		// ColorAttachmentCount: 0,
		// // Input from a shader
		// InputAttachmentCount: 0,
		// // Attachments used for multisampling colour attachments
		// // Attachments not used in this subpass, but must be preserved for the next.
		// PreserveAttachmentCount: 0,
	}

	// Attachments.
	attachmentDescriptions := make([]vk.AttachmentDescription, 0)
	colourAttachmentDescs := make([]vk.AttachmentDescription, 0)
	depthAttachmentDescs := make([]vk.AttachmentDescription, 0)

	// Can always just look at the first target since they are all the same (one per frame).
	for i := 0; i < len(config.Target.Attachments); i++ {
		attachmentConfig := config.Target.Attachments[i]

		attachmentDesc := vk.AttachmentDescription{}
		if attachmentConfig.RenderTargetAttachmentType == metadata.RENDER_TARGET_ATTACHMENT_TYPE_COLOUR {
			// Colour attachment.
			doClearColour := (pass.ClearFlags & uint8(metadata.RENDERPASS_CLEAR_COLOUR_BUFFER_FLAG)) != 0

			if attachmentConfig.Source == metadata.RENDER_TARGET_ATTACHMENT_SOURCE_DEFAULT {
				attachmentDesc.Format = vr.context.Swapchain.ImageFormat.Format
			} else {
				// TODO: configurable format?
				attachmentDesc.Format = vk.FormatR8g8b8a8Unorm
			}

			attachmentDesc.Samples = vk.SampleCount1Bit

			// Determine which load operation to use.
			if attachmentConfig.LoadOperation == metadata.RENDER_TARGET_ATTACHMENT_LOAD_OPERATION_DONT_CARE {
				// If we don't care, the only other thing that needs checking is if the attachment is being cleared.
				attachmentDesc.LoadOp = vk.AttachmentLoadOpClear
				if !doClearColour {
					attachmentDesc.LoadOp = vk.AttachmentLoadOpDontCare
				}
			} else {
				// If we are loading, check if we are also clearing. This combination doesn't make sense, and should be warned about.
				if attachmentConfig.LoadOperation == metadata.RENDER_TARGET_ATTACHMENT_LOAD_OPERATION_LOAD {
					if doClearColour {
						core.LogWarn("colour attachment load operation set to load, but is also set to clear. This combination is invalid, and will err toward clearing. Verify attachment configuration")
						attachmentDesc.LoadOp = vk.AttachmentLoadOpClear
					} else {
						attachmentDesc.LoadOp = vk.AttachmentLoadOpLoad
					}
				} else {
					core.LogFatal("Invalid and unsupported combination of load operation (0x%x) and clear flags (0x%x) for colour attachment.", attachmentDesc.LoadOp, pass.ClearFlags)
					return nil, nil
				}
			}

			// Determine which store operation to use.
			if attachmentConfig.StoreOperation == metadata.RENDER_TARGET_ATTACHMENT_STORE_OPERATION_DONT_CARE {
				attachmentDesc.StoreOp = vk.AttachmentStoreOpDontCare
			} else if attachmentConfig.StoreOperation == metadata.RENDER_TARGET_ATTACHMENT_STORE_OPERATION_STORE {
				attachmentDesc.StoreOp = vk.AttachmentStoreOpStore
			} else {
				core.LogFatal("invalid store operation (0x%d) set for depth attachment. Check configuration", attachmentConfig.StoreOperation)
				return nil, nil
			}

			// NOTE: these will never be used on a colour attachment.
			attachmentDesc.StencilLoadOp = vk.AttachmentLoadOpDontCare
			attachmentDesc.StencilStoreOp = vk.AttachmentStoreOpDontCare
			// If loading, that means coming from another pass, meaning the format should be VK_IMAGE_LAYOUT_COLOR_ATTACHMENT_OPTIMAL. Otherwise it is undefined.
			attachmentDesc.InitialLayout = vk.ImageLayoutColorAttachmentOptimal
			if attachmentConfig.LoadOperation != metadata.RENDER_TARGET_ATTACHMENT_LOAD_OPERATION_LOAD {
				attachmentDesc.InitialLayout = vk.ImageLayoutUndefined
			}

			// If this is the last pass writing to this attachment, present after should be set to true.
			attachmentDesc.FinalLayout = vk.ImageLayoutPresentSrc
			if !attachmentConfig.PresentAfter {
				attachmentDesc.FinalLayout = vk.ImageLayoutColorAttachmentOptimal // Transitioned to after the render pass
			}
			attachmentDesc.Flags = 0
			attachmentDesc.Deref()

			// Push to colour attachments array.
			colourAttachmentDescs = append(colourAttachmentDescs, attachmentDesc)
		} else if attachmentConfig.RenderTargetAttachmentType == metadata.RENDER_TARGET_ATTACHMENT_TYPE_DEPTH {
			// Depth attachment.
			doClearDepth := (pass.ClearFlags & uint8(metadata.RENDERPASS_CLEAR_DEPTH_BUFFER_FLAG)) != 0

			if attachmentConfig.Source == metadata.RENDER_TARGET_ATTACHMENT_SOURCE_DEFAULT {
				attachmentDesc.Format = vr.context.Device.DepthFormat
			} else {
				// TODO: There may be a more optimal format to use when not the default depth target.
				attachmentDesc.Format = vr.context.Device.DepthFormat
			}

			attachmentDesc.Samples = vk.SampleCount1Bit
			// Determine which load operation to use.
			if attachmentConfig.LoadOperation == metadata.RENDER_TARGET_ATTACHMENT_LOAD_OPERATION_DONT_CARE {
				// If we don't care, the only other thing that needs checking is if the attachment is being cleared.
				attachmentDesc.LoadOp = vk.AttachmentLoadOpClear
				if !doClearDepth {
					attachmentDesc.LoadOp = vk.AttachmentLoadOpDontCare
				}
			} else {
				// If we are loading, check if we are also clearing. This combination doesn't make sense, and should be warned about.
				if attachmentConfig.LoadOperation == metadata.RENDER_TARGET_ATTACHMENT_LOAD_OPERATION_LOAD {
					if doClearDepth {
						core.LogWarn("depth attachment load operation set to load, but is also set to clear. This combination is invalid, and will err toward clearing. Verify attachment configuration")
						attachmentDesc.LoadOp = vk.AttachmentLoadOpClear
					} else {
						attachmentDesc.LoadOp = vk.AttachmentLoadOpLoad
					}
				} else {
					core.LogFatal("invalid and unsupported combination of load operation (0x%d) and clear flags (0x%d) for depth attachment.", attachmentDesc.LoadOp, pass.ClearFlags)
					return nil, nil
				}
			}

			// Determine which store operation to use.
			if attachmentConfig.StoreOperation == metadata.RENDER_TARGET_ATTACHMENT_STORE_OPERATION_DONT_CARE {
				attachmentDesc.StoreOp = vk.AttachmentStoreOpDontCare
			} else if attachmentConfig.StoreOperation == metadata.RENDER_TARGET_ATTACHMENT_STORE_OPERATION_STORE {
				attachmentDesc.StoreOp = vk.AttachmentStoreOpStore
			} else {
				core.LogFatal("invalid store operation (0x%d) set for depth attachment. Check configuration", attachmentConfig.StoreOperation)
				return nil, nil
			}

			// TODO: Configurability for stencil attachments.
			attachmentDesc.StencilLoadOp = vk.AttachmentLoadOpDontCare
			attachmentDesc.StencilStoreOp = vk.AttachmentStoreOpDontCare
			// If coming from a previous pass, should already be VK_IMAGE_LAYOUT_DEPTH_STENCIL_ATTACHMENT_OPTIMAL. Otherwise undefined.
			attachmentDesc.InitialLayout = vk.ImageLayoutDepthStencilAttachmentOptimal
			if attachmentConfig.LoadOperation != metadata.RENDER_TARGET_ATTACHMENT_LOAD_OPERATION_LOAD {
				attachmentDesc.InitialLayout = vk.ImageLayoutUndefined
			}
			// Final layout for depth stencil attachments is always this.
			attachmentDesc.FinalLayout = vk.ImageLayoutDepthStencilAttachmentOptimal
			attachmentDesc.Deref()

			// Push to colour attachments array.
			depthAttachmentDescs = append(depthAttachmentDescs, attachmentDesc)
		}
		attachmentDesc.Deref()
		// Push to general array.
		attachmentDescriptions = append(attachmentDescriptions, attachmentDesc)
	}

	// Setup the attachment references.
	attachmentsAdded := uint32(0)

	// Colour attachment reference.
	colourAttachmentReferences := make([]vk.AttachmentReference, 0)
	colourAttachmentCount := len(colourAttachmentDescs)
	if colourAttachmentCount > 0 {
		colourAttachmentReferences = make([]vk.AttachmentReference, colourAttachmentCount)
		for i := 0; i < colourAttachmentCount; i++ {
			colourAttachmentReferences[i].Attachment = attachmentsAdded // Attachment description array index
			colourAttachmentReferences[i].Layout = vk.ImageLayoutColorAttachmentOptimal
			attachmentsAdded++
		}
		subpass.ColorAttachmentCount = uint32(colourAttachmentCount)
		subpass.PColorAttachments = colourAttachmentReferences
	}

	// Depth attachment reference.
	depthAttachmentCount := len(depthAttachmentDescs)
	if depthAttachmentCount > 0 {
		if depthAttachmentCount > 1 {
			core.LogFatal("multiple depth attachments not supported")
			return nil, nil
		}
		// Depth attachment reference
		depthAttachmentReference := vk.AttachmentReference{
			Attachment: 1,
			Layout:     vk.ImageLayoutDepthStencilAttachmentOptimal,
		}
		depthAttachmentReference.Deref()
		// Depth stencil data.
		subpass.PDepthStencilAttachment = &depthAttachmentReference
	}
	subpass.Deref()

	// Render pass dependencies. TODO: make this configurable.
	dependency := vk.SubpassDependency{
		SrcSubpass:      vk.SubpassExternal,
		DstSubpass:      0,
		SrcStageMask:    vk.PipelineStageFlags(vk.PipelineStageColorAttachmentOutputBit),
		SrcAccessMask:   0,
		DstStageMask:    vk.PipelineStageFlags(vk.PipelineStageColorAttachmentOutputBit),
		DstAccessMask:   vk.AccessFlags(vk.AccessColorAttachmentReadBit) | vk.AccessFlags(vk.AccessColorAttachmentWriteBit),
		DependencyFlags: 0,
	}
	dependency.Deref()

	// Render pass create.
	renderpassCreateInfo := vk.RenderPassCreateInfo{
		SType:           vk.StructureTypeRenderPassCreateInfo,
		AttachmentCount: uint32(len(attachmentDescriptions)),
		PAttachments:    attachmentDescriptions,
		SubpassCount:    1,
		PSubpasses:      []vk.SubpassDescription{subpass},
		DependencyCount: 1,
		PDependencies:   []vk.SubpassDependency{dependency},
		PNext:           nil,
		Flags:           0,
	}
	renderpassCreateInfo.Deref()

	var handle vk.RenderPass
	if err := lockPool.SafeCall(RenderpassManagement, func() error {
		result := vk.CreateRenderPass(vr.context.Device.LogicalDevice, &renderpassCreateInfo, vr.context.Allocator, &handle)
		if !VulkanResultIsSuccess(result) {
			err := fmt.Errorf("failed to create render pass with error %s", VulkanResultString(result, true))
			return err
		}
		return nil
	}); err != nil {
		return nil, err
	}
	pass.InternalData.(*VulkanRenderPass).Handle = handle

	return pass, nil
}

func (vr *VulkanRenderer) RenderPassDestroy(pass *metadata.RenderPass) error {
	if pass != nil && pass.InternalData != nil {
		rp := pass.InternalData.(*VulkanRenderPass)
		if err := lockPool.SafeCall(CommandBufferManagement, func() error {
			vk.DestroyRenderPass(vr.context.Device.LogicalDevice, rp.Handle, vr.context.Allocator)
			return nil
		}); err != nil {
			return err
		}
		rp.Handle = vk.NullRenderPass
		pass.InternalData = nil
	}
	return nil
}

func (vr *VulkanRenderer) RenderPassBegin(pass *metadata.RenderPass, target *metadata.RenderTarget) error {
	commandBuffer := vr.context.GraphicsCommandBuffers[vr.context.ImageIndex]

	// Begin the render pass.
	internalData := pass.InternalData.(*VulkanRenderPass)

	beginInfo := vk.RenderPassBeginInfo{
		SType:       vk.StructureTypeRenderPassBeginInfo,
		RenderPass:  internalData.Handle,
		Framebuffer: target.InternalFramebuffer,
		RenderArea: vk.Rect2D{
			Offset: vk.Offset2D{
				X: int32(pass.RenderArea.X),
				Y: int32(pass.RenderArea.Y),
			},
			Extent: vk.Extent2D{
				Width:  uint32(pass.RenderArea.Z),
				Height: uint32(pass.RenderArea.W),
			},
		},
		ClearValueCount: 0,
		PClearValues:    nil,
	}

	clearValues := make([]vk.ClearValue, 2)

	doClearColour := (pass.ClearFlags & uint8(metadata.RENDERPASS_CLEAR_COLOUR_BUFFER_FLAG)) != 0
	if doClearColour {
		clearValues[beginInfo.ClearValueCount].SetColor([]float32{pass.ClearColour.X, pass.ClearColour.Y, pass.ClearColour.Z, pass.ClearColour.W})
		beginInfo.ClearValueCount++
	} else {
		// Still add it anyway, but don't bother copying data since it will be ignored.
		beginInfo.ClearValueCount++
	}

	doClearDepth := (pass.ClearFlags & uint8(metadata.RENDERPASS_CLEAR_DEPTH_BUFFER_FLAG)) != 0
	if doClearDepth {
		clearValues[beginInfo.ClearValueCount].SetColor([]float32{pass.ClearColour.X, pass.ClearColour.Y, pass.ClearColour.Z, pass.ClearColour.W})
		doClearStencil := (pass.ClearFlags & uint8(metadata.RENDERPASS_CLEAR_STENCIL_BUFFER_FLAG)) != 0
		if doClearStencil {
			clearValues[beginInfo.ClearValueCount].SetDepthStencil(internalData.Depth, internalData.Stencil)
		} else {
			clearValues[beginInfo.ClearValueCount].SetDepthStencil(internalData.Depth, 0)
		}
		beginInfo.ClearValueCount++
	} else {
		for i := 0; i < len(target.Attachments); i++ {
			if target.Attachments[i].RenderTargetAttachmentType == metadata.RENDER_TARGET_ATTACHMENT_TYPE_DEPTH {
				// If there is a depth attachment, make sure to add the clear count, but don't bother copying the data.
				beginInfo.ClearValueCount++
			}
		}
	}

	if beginInfo.ClearValueCount > 0 {
		beginInfo.PClearValues = clearValues
	}
	beginInfo.Deref()

	core.LogDebug("CmdBeginRenderPass call...")
	if err := lockPool.SafeCall(CommandBufferManagement, func() error {
		vk.CmdBeginRenderPass(commandBuffer.Handle, &beginInfo, vk.SubpassContentsInline)
		return nil
	}); err != nil {
		return err
	}
	commandBuffer.State = COMMAND_BUFFER_STATE_IN_RENDER_PASS

	return nil
}

func (vr *VulkanRenderer) RenderPassEnd(pass *metadata.RenderPass) error {
	command_buffer := vr.context.GraphicsCommandBuffers[vr.context.ImageIndex]
	// End the renderpass.
	if err := lockPool.SafeCall(CommandBufferManagement, func() error {
		vk.CmdEndRenderPass(command_buffer.Handle)
		return nil
	}); err != nil {
		return err
	}
	core.LogDebug("CmdEndRenderPass called")
	command_buffer.State = COMMAND_BUFFER_STATE_RECORDING
	return nil
}

func (vr *VulkanRenderer) TextureReadData(texture *metadata.Texture, offset, size uint32) (interface{}, error) {
	image := texture.InternalData.(*VulkanImage)
	imageFormat := vr.channelCountToFormat(texture.ChannelCount, vk.FormatR8g8b8a8Unorm)
	// Create a staging buffer and load data into it.
	staging, err := vr.RenderBufferCreate(metadata.RENDERBUFFER_TYPE_READ, uint64(size))
	if err != nil {
		core.LogError("failed to create staging buffer for texture read")
		return nil, err
	}

	vr.RenderBufferBind(staging, 0)

	pool := vr.context.Device.GraphicsCommandPool
	queue := vr.context.Device.GraphicsQueue

	tempBuffer, err := AllocateAndBeginSingleUse(vr.context, pool)
	if err != nil {
		return nil, err
	}

	// NOTE: transition to VK_IMAGE_LAYOUT_TRANSFER_SRC_OPTIMAL
	// Transition the layout from whatever it is currently to optimal for handing out data.
	if err := image.ImageTransitionLayout(
		vr.context,
		texture.TextureType,
		tempBuffer,
		imageFormat,
		vk.ImageLayoutUndefined,
		vk.ImageLayoutTransferSrcOptimal); err != nil {
		return nil, err
	}
	// Copy the data to the buffer.
	buff := staging.InternalData.(*VulkanBuffer)
	if err := image.ImageCopyFromBuffer(vr.context, texture.TextureType, buff.Handle, tempBuffer); err != nil {
		return nil, err
	}
	// Transition from optimal for data reading to shader-read-only optimal layout.
	if err := image.ImageTransitionLayout(
		vr.context,
		texture.TextureType,
		tempBuffer,
		imageFormat,
		vk.ImageLayoutTransferSrcOptimal,
		vk.ImageLayoutShaderReadOnlyOptimal); err != nil {
		return nil, err
	}

	if err := tempBuffer.EndSingleUse(vr.context, pool, queue, vr.context.Device.GraphicsQueueIndex); err != nil {
		return nil, err
	}
	outMemory, err := vr.RenderBufferRead(staging, uint64(offset), uint64(size))
	if err != nil {
		core.LogError("vulkan_buffer_read failed.")
		return nil, err
	}
	if !vr.RenderBufferUnbind(staging) {
		err := fmt.Errorf("failed to unbind renderbuffer")
		return nil, err
	}

	vr.RenderBufferDestroy(staging)

	return outMemory, nil
}

func (vr *VulkanRenderer) TextureReadPixel(texture *metadata.Texture, x, y uint32) ([]uint8, error) {
	image := texture.InternalData.(*VulkanImage)
	imageFormat := vr.channelCountToFormat(texture.ChannelCount, vk.FormatR8g8b8a8Unorm)
	// TODO: creating a buffer every time isn't great. Could optimize this by creating a buffer once
	// and just reusing it.
	// Create a staging buffer and load data into it.
	staging, err := vr.RenderBufferCreate(metadata.RENDERBUFFER_TYPE_READ, uint64(unsafe.Sizeof(uint8(1))*4))
	if err != nil {
		core.LogError("failed to create staging buffer for texture pixel read")
		return nil, err
	}

	vr.RenderBufferBind(staging, 0)

	pool := vr.context.Device.GraphicsCommandPool
	queue := vr.context.Device.GraphicsQueue

	tempBuffer, err := AllocateAndBeginSingleUse(vr.context, pool)
	if err != nil {
		return nil, err
	}

	// NOTE: transition to VK_IMAGE_LAYOUT_TRANSFER_SRC_OPTIMAL
	// Transition the layout from whatever it is currently to optimal for handing out data.
	if err := image.ImageTransitionLayout(
		vr.context,
		texture.TextureType,
		tempBuffer,
		imageFormat,
		vk.ImageLayoutUndefined,
		vk.ImageLayoutTransferSrcOptimal); err != nil {
		return nil, err
	}

	// Copy the data to the buffer.
	buff := staging.InternalData.(*VulkanBuffer)
	image.ImageCopyPixelToBuffer(vr.context, texture.TextureType, buff.Handle, x, y, tempBuffer)

	// Transition from optimal for data reading to shader-read-only optimal layout.
	if err := image.ImageTransitionLayout(
		vr.context,
		texture.TextureType,
		tempBuffer,
		imageFormat,
		vk.ImageLayoutTransferSrcOptimal,
		vk.ImageLayoutShaderReadOnlyOptimal); err != nil {
		return nil, err
	}

	if err := tempBuffer.EndSingleUse(vr.context, pool, queue, vr.context.Device.GraphicsQueueIndex); err != nil {
		return nil, err
	}

	outRGBA, err := vr.RenderBufferRead(staging, 0, uint64(unsafe.Sizeof(uint8(1))*4))
	if err != nil {
		core.LogError("vulkan_buffer_read failed.")
		return nil, err
	}
	if !vr.RenderBufferUnbind(staging) {
		err := fmt.Errorf("failed to unbind renderbuffer")
		return nil, err
	}

	vr.RenderBufferDestroy(staging)

	return outRGBA.([]uint8), nil
}

func (vr *VulkanRenderer) ShaderCreate(shader *metadata.Shader, config *metadata.ShaderConfig, pass *metadata.RenderPass, stageCount uint8, stageFilenames []string, stages []metadata.ShaderStage) error {
	shader.InternalData = &VulkanShader{
		Config: &VulkanShaderConfig{
			PoolSizes:      make([]vk.DescriptorPoolSize, 2),
			DescriptorSets: make([]*VulkanDescriptorSetConfig, 2),
			Attributes:     make([]vk.VertexInputAttributeDescription, len(config.Attributes)),
			Stages:         make([]*VulkanShaderStageConfig, len(config.Stages)),
		},
		Renderpass:           &VulkanRenderPass{},
		DescriptorSetLayouts: []vk.DescriptorSetLayout{},
		Stages:               []*VulkanShaderStage{},
		GlobalDescriptorSets: []vk.DescriptorSet{},
		UniformBuffer:        &metadata.RenderBuffer{},
		Pipeline:             &VulkanPipeline{},
		InstanceStates:       make([]*VulkanShaderInstanceState, 1024),
	}

	// Translate stages
	vkStages := make([]vk.ShaderStageFlags, VULKAN_SHADER_MAX_STAGES)
	for i := uint8(0); i < stageCount; i++ {
		switch stages[i] {
		case metadata.ShaderStageFragment:
			vkStages[i] = vk.ShaderStageFlags(vk.ShaderStageFragmentBit)
		case metadata.ShaderStageVertex:
			vkStages[i] = vk.ShaderStageFlags(vk.ShaderStageVertexBit)
		case metadata.ShaderStageGeometry:
			core.LogWarn("func ShaderCreate: VK_SHADER_STAGE_GEOMETRY_BIT is set but not yet supported.")
			vkStages[i] = vk.ShaderStageFlags(vk.ShaderStageGeometryBit)
		case metadata.ShaderStageCompute:
			core.LogWarn("func ShaderCreate: SHADER_STAGE_COMPUTE is set but not yet supported.")
			vkStages[i] = vk.ShaderStageFlags(vk.ShaderStageComputeBit)
		default:
			core.LogError("Unsupported stage type: %d", stages[i])
		}
	}

	// TODO: configurable max descriptor allocate count.

	maxDescriptorAllocateCount := uint16(1024)

	// Take a copy of the pointer to the context.
	internalShader := shader.InternalData.(*VulkanShader)

	// initialize descriptorsets
	for i := range internalShader.Config.DescriptorSets {
		internalShader.Config.DescriptorSets[i] = &VulkanDescriptorSetConfig{
			Bindings: make([]vk.DescriptorSetLayoutBinding, VULKAN_SHADER_MAX_BINDINGS),
		}
	}

	internalShader.Renderpass = pass.InternalData.(*VulkanRenderPass)

	// Build out the configuration.
	internalShader.Config.MaxDescriptorSetCount = maxDescriptorAllocateCount
	internalShader.Config.Stages = make([]*VulkanShaderStageConfig, len(config.Stages))

	// Iterate provided stages.
	for i := uint8(0); i < stageCount; i++ {
		// Make sure the stage is a supported one.
		var stageFlag vk.ShaderStageFlagBits
		switch stages[i] {
		case metadata.ShaderStageVertex:
			stageFlag = vk.ShaderStageVertexBit
		case metadata.ShaderStageFragment:
			stageFlag = vk.ShaderStageFragmentBit
		default:
			// Go to the next type.
			core.LogError("vulkan_shader_create: Unsupported shader stage flagged: %d. Stage ignored.", stages[i])
			continue
		}

		// Set the stage and bump the counter.
		if internalShader.Config.Stages[i] == nil {
			internalShader.Config.Stages[i] = &VulkanShaderStageConfig{}
		}
		internalShader.Config.Stages[i].Stage = stageFlag
		internalShader.Config.Stages[i].FileName = stageFilenames[i]
	}

	// Zero out arrays and counts.
	internalShader.Config.DescriptorSets[0] = &VulkanDescriptorSetConfig{
		SamplerBindingIndex: metadata.InvalidIDUint8,
	}
	internalShader.Config.DescriptorSets[1] = &VulkanDescriptorSetConfig{
		SamplerBindingIndex: metadata.InvalidIDUint8,
	}

	// Get the uniform counts.
	internalShader.GlobalUniformCount = 0
	internalShader.GlobalUniformSamplerCount = 0
	internalShader.InstanceUniformCount = 0
	internalShader.InstanceUniformSamplerCount = 0
	internalShader.LocalUniformCount = 0

	totalCount := len(config.Uniforms)
	for i := 0; i < totalCount; i++ {
		switch config.Uniforms[i].Scope {
		case metadata.ShaderScopeGlobal:
			if config.Uniforms[i].ShaderUniformType == metadata.ShaderUniformTypeSampler {
				internalShader.GlobalUniformSamplerCount++
			} else {
				internalShader.GlobalUniformCount++
			}
		case metadata.ShaderScopeInstance:
			if config.Uniforms[i].ShaderUniformType == metadata.ShaderUniformTypeSampler {
				internalShader.InstanceUniformSamplerCount++
			} else {
				internalShader.InstanceUniformCount++
			}
		case metadata.ShaderScopeLocal:
			internalShader.LocalUniformCount++
		}
	}

	// For now, shaders will only ever have these 2 types of descriptor pools.
	internalShader.Config.PoolSizes[0] = vk.DescriptorPoolSize{Type: vk.DescriptorTypeUniformBuffer, DescriptorCount: 1024}        // HACK: max number of ubo descriptor sets.
	internalShader.Config.PoolSizes[1] = vk.DescriptorPoolSize{Type: vk.DescriptorTypeCombinedImageSampler, DescriptorCount: 4096} // HACK: max number of image sampler descriptor sets.

	internalShader.Config.PoolSizes[0].Deref()
	internalShader.Config.PoolSizes[1].Deref()

	// Global descriptor set Config.
	descriptorSetCount := 0
	if internalShader.GlobalUniformCount > 0 || internalShader.GlobalUniformSamplerCount > 0 {
		// Global descriptor set Config.
		setConfig := internalShader.Config.DescriptorSets[descriptorSetCount]
		if len(setConfig.Bindings) == 0 {
			// we do not know the size in advance
			setConfig.Bindings = []vk.DescriptorSetLayoutBinding{{}}
		}

		// Global UBO binding is first, if present.
		if internalShader.GlobalUniformCount > 0 {
			bindingIndex := setConfig.BindingCount
			setConfig.Bindings[bindingIndex] = vk.DescriptorSetLayoutBinding{
				Binding:         uint32(bindingIndex),
				DescriptorCount: 1,
				DescriptorType:  vk.DescriptorTypeUniformBuffer,
				StageFlags:      vk.ShaderStageFlags(vk.ShaderStageVertexBit) | vk.ShaderStageFlags(vk.ShaderStageFragmentBit),
			}
			setConfig.Bindings[bindingIndex].Deref()
			setConfig.BindingCount++
		}
		// Add a binding for Samplers if used.
		if internalShader.GlobalUniformSamplerCount > 0 {
			bindingIndex := setConfig.BindingCount
			setConfig.Bindings[bindingIndex] = vk.DescriptorSetLayoutBinding{
				Binding:         uint32(bindingIndex),
				DescriptorCount: uint32(internalShader.GlobalUniformSamplerCount), // One descriptor per sampler.
				DescriptorType:  vk.DescriptorTypeCombinedImageSampler,
				StageFlags:      vk.ShaderStageFlags(vk.ShaderStageVertexBit) | vk.ShaderStageFlags(vk.ShaderStageFragmentBit),
			}
			setConfig.Bindings[bindingIndex].Deref()
			setConfig.SamplerBindingIndex = bindingIndex
			setConfig.BindingCount++
		}
		// Increment the set counter.
		descriptorSetCount++
	}

	// If using instance uniforms, add a UBO descriptor set.
	if internalShader.InstanceUniformCount > 0 || internalShader.InstanceUniformSamplerCount > 0 {
		// In that set, add a binding for UBO if used.
		setConfig := internalShader.Config.DescriptorSets[descriptorSetCount]
		if len(setConfig.Bindings) == 0 {
			// we do not know the size in advance
			setConfig.Bindings = make([]vk.DescriptorSetLayoutBinding, descriptorSetCount+1)
		}

		if internalShader.InstanceUniformCount > 0 {
			bindingIndex := setConfig.BindingCount
			setConfig.Bindings[bindingIndex] = vk.DescriptorSetLayoutBinding{
				Binding:         uint32(bindingIndex),
				DescriptorCount: 1,
				DescriptorType:  vk.DescriptorTypeUniformBuffer,
				StageFlags:      vk.ShaderStageFlags(vk.ShaderStageVertexBit) | vk.ShaderStageFlags(vk.ShaderStageFragmentBit),
			}
			setConfig.Bindings[bindingIndex].Deref()
			setConfig.BindingCount++
		}
		// Add a binding for Samplers if used.
		if internalShader.InstanceUniformSamplerCount > 0 {
			bindingIndex := setConfig.BindingCount
			setConfig.Bindings[bindingIndex] = vk.DescriptorSetLayoutBinding{
				Binding:         uint32(bindingIndex),
				DescriptorCount: uint32(internalShader.InstanceUniformSamplerCount), // One descriptor per sampler.
				DescriptorType:  vk.DescriptorTypeCombinedImageSampler,
				StageFlags:      vk.ShaderStageFlags(vk.ShaderStageVertexBit) | vk.ShaderStageFlags(vk.ShaderStageFragmentBit),
			}
			setConfig.Bindings[bindingIndex].Deref()
			setConfig.SamplerBindingIndex = bindingIndex
			setConfig.BindingCount++
		}
		// Increment the set counter.
		descriptorSetCount++
	}

	if descriptorSetCount != len(internalShader.Config.DescriptorSets) {
		err := fmt.Errorf("created more descriptorsets than what it can hold")
		return err
	}

	// Invalidate all instance states.
	// TODO: dynamic
	for i := 0; i < 1024; i++ {
		if internalShader.InstanceStates[i] == nil {
			internalShader.InstanceStates[i] = &VulkanShaderInstanceState{
				ID: metadata.InvalidID,
				DescriptorSetState: &VulkanShaderDescriptorSetState{
					DescriptorSets:   make([]vk.DescriptorSet, 3),
					DescriptorStates: []*VulkanDescriptorState{},
				},
			}
			continue
		}
		internalShader.InstanceStates[i].ID = metadata.InvalidID
	}

	// Keep a copy of the cull mode.
	internalShader.Config.CullMode = config.CullMode

	return nil
}

func (vr *VulkanRenderer) ShaderDestroy(s *metadata.Shader) error {
	if s != nil && s.InternalData != nil {
		shader := s.InternalData.(*VulkanShader)
		if shader != nil {
			err := fmt.Errorf("vulkan_renderer_shader_destroy requires a valid pointer to a shader")
			return err
		}

		logicalDevice := vr.context.Device.LogicalDevice
		vkAllocator := vr.context.Allocator

		// Descriptor set layouts.
		for i := 0; i < len(shader.Config.DescriptorSets); i++ {
			if shader.DescriptorSetLayouts[i] != vk.NullDescriptorSetLayout {
				if err := lockPool.SafeCall(PipelineManagement, func() error {
					vk.DestroyDescriptorSetLayout(logicalDevice, shader.DescriptorSetLayouts[i], vkAllocator)
					return nil
				}); err != nil {
					return err
				}
				shader.DescriptorSetLayouts[i] = nil
			}
		}

		// Descriptor pool
		if shader.DescriptorPool != nil {
			if err := lockPool.SafeCall(PipelineManagement, func() error {
				vk.DestroyDescriptorPool(logicalDevice, shader.DescriptorPool, vkAllocator)
				return nil
			}); err != nil {
				return err
			}
			shader.DescriptorPool = nil
		}

		// Uniform buffer.
		vr.RenderBufferUnmapMemory(shader.UniformBuffer, 0, vk.WholeSize)

		shader.MappedUniformBufferBlock = 0

		vr.RenderBufferDestroy(shader.UniformBuffer)

		// Pipeline
		shader.Pipeline.Destroy(vr.context)

		// Shader modules
		for i := 0; i < len(shader.Config.Stages); i++ {
			if err := lockPool.SafeCall(ShaderManagement, func() error {
				vk.DestroyShaderModule(vr.context.Device.LogicalDevice, shader.Stages[i].Handle, vr.context.Allocator)
				return nil
			}); err != nil {
				return err
			}
			shader.Stages[i].Handle = nil
		}

		// Destroy the configuration.
		shader.Config = nil

		// Free the internal data memory.
		s.InternalData = nil
	}
	return nil
}

// Define the lookup table for Vulkan formats.
var shaderAttributeFormats = []vk.Format{
	metadata.ShaderAttribTypeFloat32:   vk.FormatR32Sfloat,
	metadata.ShaderAttribTypeFloat32_2: vk.FormatR32g32Sfloat,
	metadata.ShaderAttribTypeFloat32_3: vk.FormatR32g32b32Sfloat,
	metadata.ShaderAttribTypeFloat32_4: vk.FormatR32g32b32a32Sfloat,
	metadata.ShaderAttribTypeInt8:      vk.FormatR8Sint,
	metadata.ShaderAttribTypeUint8:     vk.FormatR8Uint,
	metadata.ShaderAttribTypeInt16:     vk.FormatR16Sint,
	metadata.ShaderAttribTypeUint16:    vk.FormatR16Uint,
	metadata.ShaderAttribTypeInt32:     vk.FormatR32Sint,
	metadata.ShaderAttribTypeUint32:    vk.FormatR32Uint,
}

func (vr *VulkanRenderer) ShaderInitialize(shader *metadata.Shader) error {
	logicalDevice := vr.context.Device.LogicalDevice
	vkAllocator := vr.context.Allocator
	internalShader := shader.InternalData.(*VulkanShader)

	// Create a module for each stage.
	internalShader.Stages = make([]*VulkanShaderStage, VULKAN_SHADER_MAX_STAGES)

	for i := 0; i < len(internalShader.Config.Stages); i++ {
		if internalShader.Stages[i] == nil {
			internalShader.Stages[i] = &VulkanShaderStage{}
		}
		// we have an actual stage
		if internalShader.Config.Stages[i] != nil {
			if err := vr.createModule(internalShader, internalShader.Config.Stages[i], internalShader.Stages[i]); err != nil {
				core.LogError("Unable to create %s shader module for '%s'. Shader will be destroyed", internalShader.Config.Stages[i].FileName, shader.Name)
				return err
			}
		}
	}

	// Process attributes
	offset := uint32(0)
	for i := uint32(0); i < uint32(len(shader.Attributes)); i++ {
		// Setup the new attribute.
		attribute := vk.VertexInputAttributeDescription{
			Location: i,
			Binding:  0,
			Offset:   offset,
			Format:   shaderAttributeFormats[shader.Attributes[i].ShaderUniformAttributeType],
		}
		attribute.Deref()

		// Push into the config's attribute collection and add to the stride.
		internalShader.Config.Attributes[i] = attribute

		offset += shader.Attributes[i].Size
	}

	// Descriptor pool.
	poolInfo := vk.DescriptorPoolCreateInfo{
		SType:         vk.StructureTypeDescriptorPoolCreateInfo,
		PoolSizeCount: 2,
		PPoolSizes:    internalShader.Config.PoolSizes,
		MaxSets:       uint32(internalShader.Config.MaxDescriptorSetCount),
		Flags:         vk.DescriptorPoolCreateFlags(vk.DescriptorPoolCreateFreeDescriptorSetBit),
	}
	poolInfo.Deref()

	// Create descriptor pool.
	var pDescriptorPool vk.DescriptorPool
	if err := lockPool.SafeCall(PipelineManagement, func() error {
		result := vk.CreateDescriptorPool(logicalDevice, &poolInfo, vkAllocator, &pDescriptorPool)
		if !VulkanResultIsSuccess(result) {
			err := fmt.Errorf("vulkan_shader_initialize failed creating descriptor pool: '%s'", VulkanResultString(result, true))
			return err
		}
		return nil
	}); err != nil {
		return err
	}
	internalShader.DescriptorPool = pDescriptorPool

	// Create descriptor set layouts.
	internalShader.DescriptorSetLayouts = make([]vk.DescriptorSetLayout, 2)
	for i := 0; i < len(internalShader.Config.DescriptorSets); i++ {
		layoutInfo := vk.DescriptorSetLayoutCreateInfo{
			SType:        vk.StructureTypeDescriptorSetLayoutCreateInfo,
			BindingCount: uint32(internalShader.Config.DescriptorSets[i].BindingCount),
			PBindings:    internalShader.Config.DescriptorSets[i].Bindings,
		}
		layoutInfo.Deref()

		var pSetLayout vk.DescriptorSetLayout
		if err := lockPool.SafeCall(PipelineManagement, func() error {
			result := vk.CreateDescriptorSetLayout(logicalDevice, &layoutInfo, vkAllocator, &pSetLayout)
			if !VulkanResultIsSuccess(result) {
				err := fmt.Errorf("vulkan_shader_initialize failed creating descriptor pool: '%s'", VulkanResultString(result, true))
				return err
			}
			return nil
		}); err != nil {
			return err
		}
		internalShader.DescriptorSetLayouts[i] = pSetLayout
	}

	// TODO: This feels wrong to have these here, at least in this fashion. Should probably
	// Be configured to pull from someplace instead.
	// Viewport.
	viewport := vk.Viewport{
		X:        0.0,
		Y:        float32(vr.context.FramebufferHeight),
		Width:    float32(vr.context.FramebufferWidth),
		Height:   float32(vr.context.FramebufferHeight),
		MinDepth: 0.0,
		MaxDepth: 1.0,
	}
	viewport.Deref()

	// Scissor
	scissor := vk.Rect2D{
		Offset: vk.Offset2D{
			X: 0,
			Y: 0,
		},
		Extent: vk.Extent2D{
			Width:  vr.context.FramebufferWidth,
			Height: vr.context.FramebufferHeight,
		},
	}
	scissor.Deref()

	stageCreateInfos := make([]vk.PipelineShaderStageCreateInfo, len(internalShader.Config.Stages))
	for i := 0; i < len(internalShader.Config.Stages); i++ {
		stageCreateInfos[i] = internalShader.Stages[i].ShaderStageCreateInfo
		stageCreateInfos[i].Deref()
	}

	pipConfig := &VulkanPipelineConfig{
		Renderpass:           internalShader.Renderpass,
		Stride:               uint32(shader.AttributeStride),
		Attributes:           internalShader.Config.Attributes,
		DescriptorSetLayouts: internalShader.DescriptorSetLayouts,
		Stages:               stageCreateInfos,
		Viewport:             viewport,
		Scissor:              scissor,
		CullMode:             internalShader.Config.CullMode,
		PushConstantRanges:   shader.PushConstantRanges,
		IsWireframe:          false,
		ShaderFlags:          shader.Flags,
	}

	pipeline, err := NewGraphicsPipeline(vr.context, pipConfig)
	internalShader.Pipeline = pipeline

	if err != nil {
		core.LogError("failed to load graphics pipeline for object shader")
		return err
	}

	// Grab the UBO alignment requirement from the device.
	vr.context.Device.Properties.Deref()
	vr.context.Device.Properties.Limits.Deref()
	shader.RequiredUboAlignment = uint64(vr.context.Device.Properties.Limits.MinUniformBufferOffsetAlignment)

	// Make sure the UBO is aligned according to device requirements.
	shader.GlobalUboStride = metadata.GetAligned(shader.GlobalUboSize, shader.RequiredUboAlignment)
	shader.UboStride = metadata.GetAligned(shader.UboSize, shader.RequiredUboAlignment)

	// Uniform  buffer.
	// TODO: max count should be configurable, or perhaps long term support of buffer resizing.
	totalBufferSize := shader.GlobalUboStride + (shader.UboStride * uint64(VULKAN_MAX_MATERIAL_COUNT)) // global + (locals)
	internalShader.UniformBuffer, err = vr.RenderBufferCreate(metadata.RENDERBUFFER_TYPE_UNIFORM, totalBufferSize)
	if err != nil {
		core.LogError("Vulkan buffer creation failed for object shader.")
		return err
	}
	vr.RenderBufferBind(internalShader.UniformBuffer, 0)

	// Map the entire buffer's memory.
	internalShader.MappedUniformBufferBlock, err = vr.RenderBufferMapMemory(internalShader.UniformBuffer, 0, vk.WholeSize)
	if err != nil {
		return err
	}

	// Allocate global descriptor sets, one per frame. Global is always the first set.
	globalLayouts := []vk.DescriptorSetLayout{
		internalShader.DescriptorSetLayouts[DESC_SET_INDEX_GLOBAL],
		internalShader.DescriptorSetLayouts[DESC_SET_INDEX_GLOBAL],
		internalShader.DescriptorSetLayouts[DESC_SET_INDEX_GLOBAL],
	}

	allocInfo := vk.DescriptorSetAllocateInfo{
		SType:              vk.StructureTypeDescriptorSetAllocateInfo,
		DescriptorPool:     internalShader.DescriptorPool,
		DescriptorSetCount: 3,
		PSetLayouts:        globalLayouts,
	}
	allocInfo.Deref()

	internalShader.GlobalDescriptorSets = make([]vk.DescriptorSet, 3)
	for i := 0; i < len(internalShader.GlobalDescriptorSets); i++ {
		gds := internalShader.GlobalDescriptorSets[i]
		if err := lockPool.SafeCall(PipelineManagement, func() error {
			result := vk.AllocateDescriptorSets(vr.context.Device.LogicalDevice, &allocInfo, &gds)
			if !VulkanResultIsSuccess(result) {
				err := fmt.Errorf("%s", VulkanResultString(result, true))
				return err
			}
			return nil
		}); err != nil {
			return err
		}
		internalShader.GlobalDescriptorSets[i] = gds // not necessary in theory but hey...
	}
	return nil
}

func (vr *VulkanRenderer) createModule(shader *VulkanShader, config *VulkanShaderStageConfig, shaderStage *VulkanShaderStage) error {
	// Read the resource.
	binaryResource, err := vr.assetManager.LoadAsset(config.FileName, metadata.ResourceTypeBinary, nil)
	if err != nil {
		return err
	}

	shaderStage.CreateInfo = vk.ShaderModuleCreateInfo{
		SType:    vk.StructureTypeShaderModuleCreateInfo,
		CodeSize: binaryResource.DataSize * 4,
		PCode:    binaryResource.Data.([]uint32),
	}
	shaderStage.CreateInfo.Deref()

	var shaderModule vk.ShaderModule
	if err := lockPool.SafeCall(ShaderManagement, func() error {
		result := vk.CreateShaderModule(vr.context.Device.LogicalDevice, &shaderStage.CreateInfo, vr.context.Allocator, &shaderModule)
		if !VulkanResultIsSuccess(result) {
			err := fmt.Errorf("%s", VulkanResultString(result, true))
			return err
		}
		return nil
	}); err != nil {
		return err
	}
	shaderStage.Handle = shaderModule

	// Release the resource.
	if err := vr.assetManager.UnloadAsset(binaryResource); err != nil {
		return err
	}

	// Shader stage info
	shaderStage.ShaderStageCreateInfo = vk.PipelineShaderStageCreateInfo{
		SType:               vk.StructureTypePipelineShaderStageCreateInfo,
		Stage:               config.Stage,
		Module:              shaderStage.Handle,
		PName:               VulkanSafeString("main"),
		PSpecializationInfo: nil,
		PNext:               vk.NullHandle,
		Flags:               0,
	}
	shaderStage.ShaderStageCreateInfo.Deref()

	return nil
}

func (vr *VulkanRenderer) ShaderUse(shader *metadata.Shader) error {
	s := shader.InternalData.(*VulkanShader)
	return s.Pipeline.Bind(vr.context.GraphicsCommandBuffers[vr.context.ImageIndex], vk.PipelineBindPointGraphics)
}

func (vr *VulkanRenderer) ShaderBindGlobals(shader *metadata.Shader) error {
	if shader == nil {
		return fmt.Errorf("shader is nil")
	}
	shader.BoundUboOffset = uint32(shader.GlobalUboOffset)
	return nil
}

func (vr *VulkanRenderer) ShaderBindInstance(shader *metadata.Shader, instance_id uint32) error {
	if shader == nil {
		return fmt.Errorf("shader is nil")
	}

	internal, ok := shader.InternalData.(*VulkanShader)
	if !ok {
		return fmt.Errorf("shader internal data is not of type *VulkanShader")
	}

	shader.BoundInstanceID = instance_id
	state := internal.InstanceStates[instance_id]
	shader.BoundUboOffset = uint32(state.Offset)

	return nil
}

func (vr *VulkanRenderer) ShaderApplyGlobals(shader *metadata.Shader) error {
	imageIndex := vr.context.ImageIndex
	internal := shader.InternalData.(*VulkanShader)
	commandBuffer := vr.context.GraphicsCommandBuffers[imageIndex].Handle

	// Apply UBO first
	bufferInfo := vk.DescriptorBufferInfo{
		Buffer: (internal.UniformBuffer.InternalData.(*VulkanBuffer)).Handle,
		Offset: vk.DeviceSize(shader.GlobalUboOffset),
		Range:  vk.DeviceSize(shader.GlobalUboStride),
	}
	bufferInfo.Deref()

	// Update descriptor sets.
	uboWrite := vk.WriteDescriptorSet{
		SType:           vk.StructureTypeWriteDescriptorSet,
		DstSet:          internal.GlobalDescriptorSets[imageIndex],
		DstBinding:      0,
		DstArrayElement: 0,
		DescriptorType:  vk.DescriptorTypeUniformBuffer,
		DescriptorCount: 1,
		PBufferInfo:     []vk.DescriptorBufferInfo{bufferInfo},
	}
	uboWrite.Deref()

	descriptorWrites := []vk.WriteDescriptorSet{uboWrite}

	globalSetBindingCount := uint32(internal.Config.DescriptorSets[DESC_SET_INDEX_GLOBAL].BindingCount)
	if globalSetBindingCount > 1 {
		// TODO: There are samplers to be written. Support this.
		globalSetBindingCount = 1
		core.LogError("Global image samplers are not yet supported.")

		// VkWriteDescriptorSet sampler_write = {VK_STRUCTURE_TYPE_WRITE_DESCRIPTOR_SET};
		// descriptor_writes[1] = ...
	}

	if err := lockPool.SafeCall(PipelineManagement, func() error {
		vk.UpdateDescriptorSets(vr.context.Device.LogicalDevice, globalSetBindingCount, descriptorWrites, 0, nil)
		return nil
	}); err != nil {
		return err
	}

	// Bind the global descriptor set to be updated.
	if err := lockPool.SafeCall(CommandBufferManagement, func() error {
		vk.CmdBindDescriptorSets(commandBuffer, vk.PipelineBindPointGraphics, internal.Pipeline.PipelineLayout, 0, 1, internal.GlobalDescriptorSets, 0, nil)
		return nil
	}); err != nil {
		return err
	}

	return nil
}

func (vr *VulkanRenderer) ShaderApplyInstance(shader *metadata.Shader, needsUpdate bool) error {
	internal := shader.InternalData.(*VulkanShader)
	if internal.InstanceUniformCount < 1 && internal.InstanceUniformSamplerCount < 1 {
		err := fmt.Errorf("this shader does not use instances")
		return err
	}
	imageIndex := vr.context.ImageIndex
	commandBuffer := vr.context.GraphicsCommandBuffers[imageIndex].Handle

	// Obtain instance data.
	objectState := internal.InstanceStates[shader.BoundInstanceID]
	objectDescriptorSet := objectState.DescriptorSetState.DescriptorSets[imageIndex]

	if needsUpdate {
		descriptorWrites := make([]vk.WriteDescriptorSet, 2) // Always a max of 2 descriptor sets.

		descriptorCount := uint32(0)
		descriptorIndex := uint32(0)

		bufferInfo := vk.DescriptorBufferInfo{}

		// Descriptor 0 - Uniform buffer
		if internal.InstanceUniformCount > 0 {
			// Only do this if the descriptor has not yet been updated.
			// instance_ubo_generation := object_state.DescriptorSetState.DescriptorSets[descriptor_index] //.generations[image_index]
			// TODO: determine if update is required.
			// if *instance_ubo_generation == metadata.InvalidIDUint8 {
			bufferInfo.Buffer = (internal.UniformBuffer.InternalData.(*VulkanBuffer)).Handle
			bufferInfo.Offset = vk.DeviceSize(objectState.Offset)
			bufferInfo.Range = vk.DeviceSize(shader.UboStride)
			bufferInfo.Deref()

			uboDescriptor := vk.WriteDescriptorSet{
				SType:           vk.StructureTypeWriteDescriptorSet,
				DstSet:          objectDescriptorSet,
				DstBinding:      descriptorIndex,
				DescriptorType:  vk.DescriptorTypeUniformBuffer,
				DescriptorCount: 1,
				PBufferInfo:     []vk.DescriptorBufferInfo{bufferInfo},
			}
			uboDescriptor.Deref()

			descriptorWrites[descriptorCount] = uboDescriptor
			descriptorCount++

			// Update the frame generation. In this case it is only needed once since this is a buffer.
			// *instance_ubo_generation = 1 // material.generation; TODO: some generation from... somewhere
			// }
			descriptorIndex++
		}

		// Iterate samplers.
		if internal.InstanceUniformSamplerCount > 0 {
			samplerBindingIndex := internal.Config.DescriptorSets[DESC_SET_INDEX_INSTANCE].SamplerBindingIndex
			totalSamplerCount := internal.Config.DescriptorSets[DESC_SET_INDEX_INSTANCE].Bindings[samplerBindingIndex].DescriptorCount
			updateSamplerCount := uint32(0)
			imageInfos := make([]vk.DescriptorImageInfo, VULKAN_SHADER_MAX_GLOBAL_TEXTURES)

			for i := uint32(0); i < totalSamplerCount; i++ {
				// TODO: only update in the list if actually needing an update.
				textureMap := internal.InstanceStates[shader.BoundInstanceID].InstanceTextureMaps[i]
				texture := textureMap.Texture

				// Ensure the texture is valid.
				if texture.Generation == metadata.InvalidID {
					switch textureMap.Use {
					case metadata.TextureUseMapDiffuse:
						texture = vr.defaultTexture.DefaultDiffuseTexture
					case metadata.TextureUseMapSpecular:
						texture = vr.defaultTexture.DefaultSpecularTexture
					case metadata.TextureUseMapNormal:
						texture = vr.defaultTexture.DefaultNormalTexture
					default:
						core.LogWarn("Undefined texture use %d", textureMap.Use)
						texture = vr.defaultTexture.DefaultTexture
					}
				}

				image := texture.InternalData.(*VulkanImage)
				imageInfos[i].ImageLayout = vk.ImageLayoutShaderReadOnlyOptimal
				imageInfos[i].ImageView = image.ImageView
				imageInfos[i].Sampler = textureMap.InternalData.(vk.Sampler)

				// TODO: change up descriptor state to handle this properly.
				// Sync frame generation if not using a default texture.
				// if (t.generation != INVALID_ID) {
				//     *descriptor_generation = t.generation;
				//     *descriptor_id = t.id;
				// }

				updateSamplerCount++
			}

			samplerDescriptor := vk.WriteDescriptorSet{
				SType:           vk.StructureTypeWriteDescriptorSet,
				DstSet:          objectDescriptorSet,
				DstBinding:      descriptorIndex,
				DescriptorType:  vk.DescriptorTypeCombinedImageSampler,
				DescriptorCount: updateSamplerCount,
				PImageInfo:      imageInfos,
			}
			samplerDescriptor.Deref()

			descriptorWrites[descriptorCount] = samplerDescriptor
			descriptorCount++
		}

		if descriptorCount > 0 {
			if err := lockPool.SafeCall(PipelineManagement, func() error {
				vk.UpdateDescriptorSets(vr.context.Device.LogicalDevice, descriptorCount, descriptorWrites, 0, nil)
				return nil
			}); err != nil {
				return err
			}
		}
	}

	// Bind the descriptor set to be updated, or in case the shader changed.
	if err := lockPool.SafeCall(CommandBufferManagement, func() error {
		vk.CmdBindDescriptorSets(commandBuffer, vk.PipelineBindPointGraphics, internal.Pipeline.PipelineLayout, 1, 1, objectState.DescriptorSetState.DescriptorSets, 0, nil)
		return nil
	}); err != nil {
		return err
	}

	return nil
}

func (vr *VulkanRenderer) ShaderAcquireInstanceResources(shader *metadata.Shader, maps []*metadata.TextureMap) (uint32, error) {
	internal := shader.InternalData.(*VulkanShader)

	// TODO: dynamic
	outInstanceID := metadata.InvalidID
	for i := uint32(0); i < 1024; i++ {
		if internal.InstanceStates[i].ID == metadata.InvalidID {
			internal.InstanceStates[i].ID = i
			outInstanceID = i
			break
		}
	}
	if outInstanceID == metadata.InvalidID {
		err := fmt.Errorf("vulkan_shader_acquire_instance_resources failed to acquire new id")
		return 0, err
	}

	instanceState := internal.InstanceStates[outInstanceID]
	samplerBindingIndex := internal.Config.DescriptorSets[DESC_SET_INDEX_INSTANCE].SamplerBindingIndex

	if samplerBindingIndex != metadata.InvalidIDUint8 {
		instanceTextureCount := internal.Config.DescriptorSets[DESC_SET_INDEX_INSTANCE].Bindings[samplerBindingIndex].DescriptorCount
		// Only setup if the shader actually requires it.
		if shader.InstanceTextureCount > 0 {
			instanceState.InstanceTextureMaps = make([]*metadata.TextureMap, shader.InstanceTextureCount)
			for i := uint32(0); i < instanceTextureCount; i++ {
				if maps[i].Texture == nil {
					instanceState.InstanceTextureMaps[i] = &metadata.TextureMap{
						Texture:      vr.defaultTexture.DefaultTexture,
						InternalData: *new(vk.Sampler),
					}
				}
			}
		}
	}

	// Allocate some space in the UBO - by the stride, not the size.
	internal.UniformBuffer.Buffer = make([]interface{}, uint32(m.Max(1, float64(shader.UboStride))))
	setState := instanceState.DescriptorSetState

	// Each descriptor binding in the set
	bindingCount := internal.Config.DescriptorSets[DESC_SET_INDEX_INSTANCE].BindingCount
	for i := uint32(0); i < uint32(bindingCount); i++ {
		if len(setState.DescriptorStates) == 0 {
			setState.DescriptorStates = make([]*VulkanDescriptorState, 3)
		}
		if setState.DescriptorStates[i] == nil {
			setState.DescriptorStates[i] = &VulkanDescriptorState{
				Generations: [3]uint8{metadata.InvalidIDUint8, metadata.InvalidIDUint8, metadata.InvalidIDUint8},
				IDs:         [3]uint32{metadata.InvalidID, metadata.InvalidID, metadata.InvalidID},
			}
		}
	}

	// Allocate 3 descriptor sets (one per frame).
	layouts := []vk.DescriptorSetLayout{
		internal.DescriptorSetLayouts[DESC_SET_INDEX_INSTANCE],
		internal.DescriptorSetLayouts[DESC_SET_INDEX_INSTANCE],
		internal.DescriptorSetLayouts[DESC_SET_INDEX_INSTANCE],
	}

	allocInfo := vk.DescriptorSetAllocateInfo{
		SType:              vk.StructureTypeDescriptorSetAllocateInfo,
		DescriptorPool:     internal.DescriptorPool,
		DescriptorSetCount: uint32(len(layouts)),
		PSetLayouts:        layouts,
	}
	allocInfo.Deref()

	for i := 0; i < len(setState.DescriptorSets); i++ {
		var pDescriptorSets vk.DescriptorSet
		if err := lockPool.SafeCall(PipelineManagement, func() error {
			result := vk.AllocateDescriptorSets(vr.context.Device.LogicalDevice, &allocInfo, &pDescriptorSets)
			if !VulkanResultIsSuccess(result) {
				err := fmt.Errorf("error allocating instance descriptor sets in shader: '%s'", VulkanResultString(result, true))
				return err
			}
			return nil
		}); err != nil {
			return 0, err
		}
		setState.DescriptorSets[i] = pDescriptorSets
	}

	return outInstanceID, nil
}

func (vr *VulkanRenderer) ShaderReleaseInstanceResources(shader *metadata.Shader, instance_id uint32) error {
	internal := shader.InternalData.(*VulkanShader)
	instanceState := internal.InstanceStates[instance_id]

	// Wait for any pending operations using the descriptor set to finish.
	if err := lockPool.SafeCall(DeviceManagement, func() error {
		if res := vk.DeviceWaitIdle(vr.context.Device.LogicalDevice); !VulkanResultIsSuccess(res) {
			err := fmt.Errorf("device wait idle failed with error %s", VulkanResultString(res, true))
			return err
		}
		return nil
	}); err != nil {
		return err
	}

	// Free 3 descriptor sets (one per frame)
	for _, ds := range instanceState.DescriptorSetState.DescriptorSets {
		if err := lockPool.SafeCall(PipelineManagement, func() error {
			result := vk.FreeDescriptorSets(vr.context.Device.LogicalDevice, internal.DescriptorPool, 1, &ds)
			if !VulkanResultIsSuccess(result) {
				err := fmt.Errorf("error freeing object shader descriptor sets")
				return err
			}
			return nil
		}); err != nil {
			return err
		}
	}

	// Destroy descriptor states.
	instanceState.DescriptorSetState.DescriptorStates = nil
	instanceState.InstanceTextureMaps = nil

	if err := vr.RenderBufferFree(internal.UniformBuffer, shader.UboStride, instanceState.Offset); err != nil {
		core.LogError("vulkan_renderer_shader_release_instance_resources failed to free range from render buffer.")
		return err
	}
	instanceState.Offset = metadata.InvalidIDUint64
	instanceState.ID = metadata.InvalidID

	return nil
}

func (vr *VulkanRenderer) SetUniform(shader *metadata.Shader, uniform metadata.ShaderUniform, value interface{}) error {
	internal := shader.InternalData.(*VulkanShader)
	if uniform.ShaderUniformType == metadata.ShaderUniformTypeSampler {
		if uniform.Scope == metadata.ShaderScopeGlobal {
			shader.GlobalTextureMaps[uniform.Location] = value.(*metadata.TextureMap)
		} else {
			internal.InstanceStates[shader.BoundInstanceID].InstanceTextureMaps[uniform.Location] = value.(*metadata.TextureMap)
		}
	} else {
		if uniform.Scope == metadata.ShaderScopeLocal {
			// Is local, using push constants. Do this immediately.
			commandBuffer := vr.context.GraphicsCommandBuffers[vr.context.ImageIndex].Handle

			var dataPtr unsafe.Pointer
			var dataSize int
			switch v := value.(type) {
			case math.Mat4:
				dataSize = int(unsafe.Sizeof(v))
				dataPtr = unsafe.Pointer(&v.Data[0])
			case math.Vec3:
				dataSize = int(unsafe.Sizeof(v))
				dataPtr = unsafe.Pointer(&v)
			case float32:
				dataSize = int(unsafe.Sizeof(v))
				dataPtr = unsafe.Pointer(&v)
			default:
				err := fmt.Errorf("unsupported push constant type: %T", value)
				return err
			}

			// Check size consistency
			if dataSize != int(uniform.Size) {
				err := fmt.Errorf("size mismatch: expected %d, got %d", uniform.Size, dataSize)
				return err
			}

			if err := lockPool.SafeCall(CommandBufferManagement, func() error {
				vk.CmdPushConstants(commandBuffer, internal.Pipeline.PipelineLayout,
					vk.ShaderStageFlags(vk.ShaderStageVertexBit)|vk.ShaderStageFlags(vk.ShaderStageFragmentBit),
					uint32(uniform.Offset), uint32(uniform.Size), dataPtr,
				)
				return nil
			}); err != nil {
				return err
			}

			// Ensure the Go pointer is kept alive during the Vulkan call
			runtime.KeepAlive(value)
		} else {
			// Map the appropriate memory location and copy the data over.
			addr := internal.MappedUniformBufferBlock.(uint64)
			addr += uint64(shader.BoundUboOffset) + uniform.Offset
		}
	}
	return nil
}

func (vr *VulkanRenderer) TextureMapAcquireResources(texture_map *metadata.TextureMap) error {
	// Create a sampler for the texture
	samplerInfo := vk.SamplerCreateInfo{
		SType:        vk.StructureTypeSamplerCreateInfo,
		MinFilter:    vr.convertFilterType("min", texture_map.FilterMinify),
		MagFilter:    vr.convertFilterType("mag", texture_map.FilterMagnify),
		AddressModeU: vr.convertRepeatType("U", texture_map.RepeatU),
		AddressModeV: vr.convertRepeatType("V", texture_map.RepeatV),
		AddressModeW: vr.convertRepeatType("W", texture_map.RepeatW),
		// TODO: Configurable
		AnisotropyEnable:        vk.True,
		MaxAnisotropy:           16,
		BorderColor:             vk.BorderColorIntOpaqueBlack,
		UnnormalizedCoordinates: vk.False,
		CompareEnable:           vk.False,
		CompareOp:               vk.CompareOpAlways,
		MipmapMode:              vk.SamplerMipmapModeLinear,
		MipLodBias:              0.0,
		MinLod:                  0.0,
		MaxLod:                  0.0,
	}
	samplerInfo.Deref()

	var pSampler vk.Sampler
	if err := lockPool.SafeCall(ResourceManagement, func() error {
		result := vk.CreateSampler(vr.context.Device.LogicalDevice, &samplerInfo, vr.context.Allocator, &pSampler)
		if !VulkanResultIsSuccess(result) {
			err := fmt.Errorf("error creating texture sampler: %s", VulkanResultString(result, true))
			return err
		}
		return nil
	}); err != nil {
		return err
	}
	texture_map.InternalData = pSampler
	return nil
}

func (vr *VulkanRenderer) TextureMapReleaseResources(texture_map *metadata.TextureMap) error {
	if texture_map != nil {
		// Make sure there's no way this is in use.
		if err := lockPool.SafeCall(DeviceManagement, func() error {
			if res := vk.DeviceWaitIdle(vr.context.Device.LogicalDevice); !VulkanResultIsSuccess(res) {
				err := fmt.Errorf("device wait idle failed with error %s", VulkanResultString(res, true))
				return err
			}
			return nil
		}); err != nil {
			return err
		}

		if err := lockPool.SafeCall(ResourceManagement, func() error {
			vk.DestroySampler(vr.context.Device.LogicalDevice, texture_map.InternalData.(vk.Sampler), vr.context.Allocator)
			return nil
		}); err != nil {
			return err
		}

		texture_map.InternalData = 0
	}
	return nil
}

func (vr *VulkanRenderer) RenderTargetCreate(attachmentCount uint8, attachments []*metadata.RenderTargetAttachment, pass *metadata.RenderPass, width, height uint32) (*metadata.RenderTarget, error) {
	// Max number of attachments
	attachmentViews := make([]vk.ImageView, attachmentCount)
	for i := uint32(0); i < uint32(attachmentCount); i++ {
		image := attachments[i].Texture.InternalData.(*VulkanImage)
		attachmentViews[i] = image.ImageView
		if attachmentViews[i] == vk.NullImageView {
			return nil, fmt.Errorf("attachment view %d is null", i)
		}
	}

	if attachmentCount != uint8(len(attachments)) {
		err := fmt.Errorf("attachments are not correct")
		return nil, err
	}

	// Take a copy of the attachments and count.
	outTarget := &metadata.RenderTarget{
		AttachmentCount:     attachmentCount,
		Attachments:         attachments,
		InternalFramebuffer: vk.NullFramebuffer,
	}

	rp := pass.InternalData.(*VulkanRenderPass)
	framebufferCreateInfo := vk.FramebufferCreateInfo{
		SType:           vk.StructureTypeFramebufferCreateInfo,
		RenderPass:      rp.Handle,
		AttachmentCount: uint32(attachmentCount),
		PAttachments:    attachmentViews,
		Width:           width,
		Height:          height,
		Layers:          1,
	}
	framebufferCreateInfo.Deref()

	var fb vk.Framebuffer
	if err := lockPool.SafeCall(PipelineManagement, func() error {
		result := vk.CreateFramebuffer(vr.context.Device.LogicalDevice, &framebufferCreateInfo, vr.context.Allocator, &fb)
		if !VulkanResultIsSuccess(result) {
			err := fmt.Errorf("%s", VulkanResultString(result, true))
			return err
		}
		return nil
	}); err != nil {
		return nil, err
	}

	if fb == vk.NullFramebuffer {
		return nil, fmt.Errorf("framebuffer handle is null")
	}

	outTarget.InternalFramebuffer = fb

	return outTarget, nil
}

func (vr *VulkanRenderer) RenderTargetDestroy(target *metadata.RenderTarget, freeInternalMemory bool) error {
	if target != nil && target.InternalFramebuffer != nil {
		if err := lockPool.SafeCall(PipelineManagement, func() error {
			vk.DestroyFramebuffer(vr.context.Device.LogicalDevice, target.InternalFramebuffer, vr.context.Allocator)
			return nil
		}); err != nil {
			return err
		}
		target.InternalFramebuffer = vk.NullFramebuffer
		if freeInternalMemory {
			target.Attachments = nil
			target.AttachmentCount = 0
		}
	}
	return nil
}

func (vr *VulkanRenderer) IsMultithreaded() bool {
	return vr.context.MultithreadingEnabled
}

func (vr *VulkanRenderer) RenderBufferCreate(renderbufferType metadata.RenderBufferType, totalSize uint64) (*metadata.RenderBuffer, error) {
	outBuffer := &metadata.RenderBuffer{
		RenderBufferType: renderbufferType,
		TotalSize:        totalSize,
	}

	internalBuffer := &VulkanBuffer{}

	switch outBuffer.RenderBufferType {
	case metadata.RENDERBUFFER_TYPE_VERTEX:
		internalBuffer.Usage = vk.BufferUsageFlags(vk.BufferUsageVertexBufferBit) | vk.BufferUsageFlags(vk.BufferUsageTransferDstBit) | vk.BufferUsageFlags(vk.BufferUsageTransferSrcBit)
		internalBuffer.MemoryPropertyFlags = uint32(vk.MemoryPropertyDeviceLocalBit)
	case metadata.RENDERBUFFER_TYPE_INDEX:
		internalBuffer.Usage = vk.BufferUsageFlags(vk.BufferUsageIndexBufferBit) | vk.BufferUsageFlags(vk.BufferUsageTransferDstBit) | vk.BufferUsageFlags(vk.BufferUsageTransferSrcBit)
		internalBuffer.MemoryPropertyFlags = uint32(vk.MemoryPropertyDeviceLocalBit)
	case metadata.RENDERBUFFER_TYPE_UNIFORM:
		deviceLocalBits := uint32(vk.MemoryPropertyDeviceLocalBit)
		if vr.context.Device.SupportsDeviceLocalHostVisible {
			deviceLocalBits = 0
		}
		internalBuffer.Usage = vk.BufferUsageFlags(vk.BufferUsageUniformBufferBit) | vk.BufferUsageFlags(vk.BufferUsageTransferDstBit)
		internalBuffer.MemoryPropertyFlags = uint32(vk.MemoryPropertyHostVisibleBit) | uint32(vk.MemoryPropertyHostCoherentBit) | deviceLocalBits
	case metadata.RENDERBUFFER_TYPE_STAGING:
		internalBuffer.Usage = vk.BufferUsageFlags(vk.BufferUsageTransferSrcBit)
		internalBuffer.MemoryPropertyFlags = uint32(vk.MemoryPropertyHostVisibleBit) | uint32(vk.MemoryPropertyHostCoherentBit)
	case metadata.RENDERBUFFER_TYPE_READ:
		internalBuffer.Usage = vk.BufferUsageFlags(vk.BufferUsageTransferDstBit)
		internalBuffer.MemoryPropertyFlags = uint32(vk.MemoryPropertyHostVisibleBit) | uint32(vk.MemoryPropertyHostCoherentBit)
	case metadata.RENDERBUFFER_TYPE_STORAGE:
		err := fmt.Errorf("storage buffer not yet supported")
		return nil, err
	default:
		err := fmt.Errorf("unsupported buffer type: %d", outBuffer.RenderBufferType)
		return nil, err
	}

	bufferInfo := vk.BufferCreateInfo{
		SType:       vk.StructureTypeBufferCreateInfo,
		Size:        vk.DeviceSize(outBuffer.TotalSize),
		Usage:       internalBuffer.Usage,
		SharingMode: vk.SharingModeExclusive, // NOTE: Only used in one queue.
	}
	bufferInfo.Deref()

	var pBuffer vk.Buffer
	if err := lockPool.SafeCall(ResourceManagement, func() error {
		result := vk.CreateBuffer(vr.context.Device.LogicalDevice, &bufferInfo, vr.context.Allocator, &pBuffer)
		if !VulkanResultIsSuccess(result) {
			err := fmt.Errorf("%s", VulkanResultString(result, true))
			return err
		}
		return nil
	}); err != nil {
		return nil, err
	}
	internalBuffer.Handle = pBuffer

	// Gather memory requirements.
	var pMemoryRequirements vk.MemoryRequirements

	if err := lockPool.SafeCall(MemoryManagement, func() error {
		vk.GetBufferMemoryRequirements(vr.context.Device.LogicalDevice, internalBuffer.Handle, &pMemoryRequirements)
		return nil
	}); err != nil {
		return nil, err
	}
	pMemoryRequirements.Deref()
	internalBuffer.MemoryRequirements = pMemoryRequirements

	internalBuffer.MemoryIndex = vr.context.FindMemoryIndex(internalBuffer.MemoryRequirements.MemoryTypeBits, internalBuffer.MemoryPropertyFlags)
	if internalBuffer.MemoryIndex == -1 {
		err := fmt.Errorf("unable to create vulkan buffer because the required memory type index was not found")
		return nil, err
	}

	// Allocate memory info
	allocateInfo := vk.MemoryAllocateInfo{
		SType:           vk.StructureTypeMemoryAllocateInfo,
		AllocationSize:  internalBuffer.MemoryRequirements.Size,
		MemoryTypeIndex: uint32(internalBuffer.MemoryIndex),
	}
	allocateInfo.Deref()

	// Allocate the memory.
	var mem vk.DeviceMemory

	if err := lockPool.SafeCall(DeviceManagement, func() error {
		result := vk.AllocateMemory(vr.context.Device.LogicalDevice, &allocateInfo, vr.context.Allocator, &mem)
		if !VulkanResultIsSuccess(result) {
			err := fmt.Errorf("%s", VulkanResultString(result, true))
			return err
		}
		return nil
	}); err != nil {
		return nil, err
	}
	internalBuffer.Memory = mem

	// Allocate the internal state block of memory at the end once we are sure everything was created successfully.
	outBuffer.InternalData = internalBuffer

	return outBuffer, nil
}

func (vr *VulkanRenderer) RenderBufferDestroy(buffer *metadata.RenderBuffer) {
	if buffer != nil {
		buffer = nil
	}
}

func (vr *VulkanRenderer) RenderBufferBind(buffer *metadata.RenderBuffer, offset uint64) error {
	if buffer == nil {
		err := fmt.Errorf("renderer_renderbuffer_bind requires a valid pointer to a buffer")
		return err
	}
	internalBuffer := buffer.InternalData.(*VulkanBuffer)

	if err := lockPool.SafeCall(MemoryManagement, func() error {
		result := vk.BindBufferMemory(vr.context.Device.LogicalDevice, internalBuffer.Handle, internalBuffer.Memory, vk.DeviceSize(offset))
		if !VulkanResultIsSuccess(result) {
			err := fmt.Errorf("%s", VulkanResultString(result, true))
			return err
		}
		return nil
	}); err != nil {
		return err
	}
	return nil
}

func (vr *VulkanRenderer) RenderBufferUnbind(buffer *metadata.RenderBuffer) bool {
	if buffer == nil || buffer.InternalData == nil {
		core.LogError("vulkan_buffer_unbind requires valid pointer to a buffer.")
		return false
	}
	// NOTE: Does nothing, for now.
	return true
}

func (vr *VulkanRenderer) RenderBufferMapMemory(buffer *metadata.RenderBuffer, offset, size uint64) (interface{}, error) {
	if buffer == nil || buffer.InternalData == nil {
		err := fmt.Errorf("vulkan_buffer_map_memory requires a valid pointer to a buffer")
		return nil, err
	}
	internalBuffer := buffer.InternalData.(*VulkanBuffer)

	var dataPtr unsafe.Pointer
	if err := lockPool.SafeCall(MemoryManagement, func() error {
		result := vk.MapMemory(vr.context.Device.LogicalDevice, internalBuffer.Memory, vk.DeviceSize(offset), vk.DeviceSize(size), 0, &dataPtr)
		if !VulkanResultIsSuccess(result) {
			err := fmt.Errorf("failed to map memory with error %s", VulkanResultString(result, true))
			return err
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return uint64(uintptr(dataPtr)), nil
}

func (vr *VulkanRenderer) RenderBufferUnmapMemory(buffer *metadata.RenderBuffer, offset, size uint64) error {
	if buffer == nil || buffer.InternalData == nil {
		err := fmt.Errorf("vulkan_buffer_unmap_memory requires a valid pointer to a buffer")
		return err
	}
	internalBuffer := buffer.InternalData.(*VulkanBuffer)

	if err := lockPool.SafeCall(MemoryManagement, func() error {
		vk.UnmapMemory(vr.context.Device.LogicalDevice, internalBuffer.Memory)
		return nil
	}); err != nil {
		return err
	}
	return nil
}

func (vr *VulkanRenderer) RenderBufferFlush(buffer *metadata.RenderBuffer, offset, size uint64) error {
	if buffer == nil || buffer.InternalData == nil {
		err := fmt.Errorf("vulkan_buffer_flush requires a valid pointer to a buffer")
		return err
	}
	// NOTE: If not host-coherent, flush the mapped memory range.
	internalBuffer := buffer.InternalData.(*VulkanBuffer)
	if !vr.vulkanBufferIsHostCoherent(internalBuffer) {
		mrange := vk.MappedMemoryRange{
			SType:  vk.StructureTypeMappedMemoryRange,
			Memory: internalBuffer.Memory,
			Offset: vk.DeviceSize(offset),
			Size:   vk.DeviceSize(size),
		}
		if err := lockPool.SafeCall(MemoryManagement, func() error {
			result := vk.FlushMappedMemoryRanges(vr.context.Device.LogicalDevice, 1, []vk.MappedMemoryRange{mrange})
			if !VulkanResultIsSuccess(result) {
				err := fmt.Errorf("failed to flush mapped memory ranges with error %s", VulkanResultString(result, true))
				return err
			}
			return nil
		}); err != nil {
			return err
		}
	}

	return nil
}

func (vr *VulkanRenderer) RenderBufferRead(buffer *metadata.RenderBuffer, offset, size uint64) (interface{}, error) {
	if buffer == nil || buffer.InternalData == nil {
		err := fmt.Errorf("vulkan_buffer_read requires a valid pointer to a buffer and out_memory, and the size must be nonzero")
		return nil, err
	}

	var outMemory interface{}
	internalBuffer := buffer.InternalData.(*VulkanBuffer)
	if vr.vulkanBufferIsDeviceLocal(internalBuffer) && !vr.vulkanBufferIsHostVisible(internalBuffer) {
		// NOTE: If a read buffer is needed (i.e.) the target buffer's memory is not host visible but is device-local,
		// create the read buffer, copy data to it, then read from that buffer.

		// Create a host-visible staging buffer to copy to. Mark it as the destination of the transfer.
		read, err := vr.RenderBufferCreate(metadata.RENDERBUFFER_TYPE_READ, size)
		if err != nil {
			core.LogError("vulkan_buffer_read() - Failed to create read buffer.")
			return nil, err
		}
		vr.RenderBufferBind(read, 0)
		readInternal := read.InternalData.(*VulkanBuffer)

		// Perform the copy from device local to the read buffer.
		vr.RenderBufferCopyRange(buffer, offset, read, 0, size)

		// Map/copy/unmap
		var dataPtr unsafe.Pointer
		if err := lockPool.SafeCall(MemoryManagement, func() error {
			result := vk.MapMemory(vr.context.Device.LogicalDevice, readInternal.Memory, 0, vk.DeviceSize(size), 0, &dataPtr)
			if !VulkanResultIsSuccess(result) {
				err := fmt.Errorf("%s", VulkanResultString(result, true))
				return err
			}
			return nil
		}); err != nil {
			return nil, err
		}

		slice := unsafe.Slice(&dataPtr, size)
		outMemory = *(*[]uint8)(unsafe.Pointer(&slice))

		if err := lockPool.SafeCall(MemoryManagement, func() error {
			vk.UnmapMemory(vr.context.Device.LogicalDevice, readInternal.Memory)
			return nil
		}); err != nil {
			return nil, err
		}

		// Clean up the read buffer.
		vr.RenderBufferUnbind(read)
		vr.RenderBufferDestroy(read)
	} else {
		// If no staging buffer is needed, map/copy/unmap.
		var dataPtr unsafe.Pointer
		if err := lockPool.SafeCall(MemoryManagement, func() error {
			result := vk.MapMemory(vr.context.Device.LogicalDevice, internalBuffer.Memory, vk.DeviceSize(offset), vk.DeviceSize(size), 0, &dataPtr)
			if !VulkanResultIsSuccess(result) {
				err := fmt.Errorf("%s", VulkanResultString(result, true))
				return err
			}
			return nil
		}); err != nil {
			return nil, err
		}
		// Create a slice header
		slice := unsafe.Slice(&dataPtr, size)

		// Convert the slice header to a []uint8
		outMemory = *(*[]uint8)(unsafe.Pointer(&slice))
		if err := lockPool.SafeCall(MemoryManagement, func() error {
			vk.UnmapMemory(vr.context.Device.LogicalDevice, internalBuffer.Memory)
			return nil
		}); err != nil {
			return nil, err
		}
	}
	return outMemory, nil
}

func (vr *VulkanRenderer) RenderBufferResize(buffer *metadata.RenderBuffer, new_total_size uint64) error {
	if buffer == nil || buffer.InternalData == nil {
		err := fmt.Errorf("buffer or internaldata need to be a valid pointer")
		return err
	}

	internalBuffer := buffer.InternalData.(*VulkanBuffer)

	// Create new buffer.
	bufferInfo := vk.BufferCreateInfo{
		SType:       vk.StructureTypeBufferCreateInfo,
		Size:        vk.DeviceSize(new_total_size),
		Usage:       internalBuffer.Usage,
		SharingMode: vk.SharingModeExclusive, // NOTE: Only used in one queue.
	}

	var newBuffer vk.Buffer
	if err := lockPool.SafeCall(ResourceManagement, func() error {
		result := vk.CreateBuffer(vr.context.Device.LogicalDevice, &bufferInfo, vr.context.Allocator, &newBuffer)
		if !VulkanResultIsSuccess(result) {
			err := fmt.Errorf("failed to create buffer with error %s", VulkanResultString(result, true))
			return err
		}
		return nil
	}); err != nil {
		return err
	}

	// Gather memory requirements.
	requirements := vk.MemoryRequirements{}
	if err := lockPool.SafeCall(MemoryManagement, func() error {
		vk.GetBufferMemoryRequirements(vr.context.Device.LogicalDevice, newBuffer, &requirements)
		return nil
	}); err != nil {
		return err
	}

	// Allocate memory info
	allocateInfo := vk.MemoryAllocateInfo{
		SType:           vk.StructureTypeMemoryAllocateInfo,
		AllocationSize:  requirements.Size,
		MemoryTypeIndex: uint32(internalBuffer.MemoryIndex),
	}
	allocateInfo.Deref()

	// Allocate the memory.
	var newMemory vk.DeviceMemory
	if err := lockPool.SafeCall(DeviceManagement, func() error {
		result := vk.AllocateMemory(vr.context.Device.LogicalDevice, &allocateInfo, vr.context.Allocator, &newMemory)
		if !VulkanResultIsSuccess(result) {
			err := fmt.Errorf("unable to resize vulkan buffer because the required memory allocation failed. Error: %s", VulkanResultString(result, true))
			return err
		}
		return nil
	}); err != nil {
		return err
	}

	// Bind the new buffer's memory
	if err := lockPool.SafeCall(ResourceManagement, func() error {
		result := vk.BindBufferMemory(vr.context.Device.LogicalDevice, newBuffer, vk.DeviceMemory(newMemory), vk.DeviceSize(0))
		if !VulkanResultIsSuccess(result) {
			err := fmt.Errorf("failed to bind buffer memory with error %s", VulkanResultString(result, true))
			return err
		}
		return nil
	}); err != nil {
		return err
	}

	// Copy over the data.
	vr.vulkanBufferCopyRangeInternal(internalBuffer.Handle, 0, newBuffer, 0, buffer.TotalSize)

	// Make sure anything potentially using these is finished.
	// NOTE: We could use vkQueueWaitIdle here if we knew what queue this buffer would be used with...
	if err := lockPool.SafeCall(DeviceManagement, func() error {
		if res := vk.DeviceWaitIdle(vr.context.Device.LogicalDevice); !VulkanResultIsSuccess(res) {
			err := fmt.Errorf("device wait idle failed with error %s", VulkanResultString(res, true))
			return err
		}
		return nil
	}); err != nil {
		return err
	}

	// Destroy the old
	if internalBuffer.Memory != nil {
		if err := lockPool.SafeCall(DeviceManagement, func() error {
			vk.FreeMemory(vr.context.Device.LogicalDevice, internalBuffer.Memory, vr.context.Allocator)
			return nil
		}); err != nil {
			return err
		}
		internalBuffer.Memory = nil
	}
	if internalBuffer.Handle != nil {
		if err := lockPool.SafeCall(ResourceManagement, func() error {
			vk.DestroyBuffer(vr.context.Device.LogicalDevice, internalBuffer.Handle, vr.context.Allocator)
			return nil
		}); err != nil {
			return err
		}
		internalBuffer.Handle = nil
	}

	// Report free of the old, allocate of the new.
	isDeviceMemory := (internalBuffer.MemoryPropertyFlags & uint32(vk.MemoryPropertyDeviceLocalBit)) == uint32(vk.MemoryPropertyDeviceLocalBit)

	internalBuffer.MemoryRequirements = requirements
	internalBuffer.MemoryRequirements.Size = 1 //MEMORY_TAG_GPU_LOCAL
	if !isDeviceMemory {
		internalBuffer.MemoryRequirements.Size = 2 //MEMORY_TAG_VULKAN
	}

	// Set new properties
	internalBuffer.Memory = newMemory
	internalBuffer.Handle = newBuffer

	return nil
}

func (vr *VulkanRenderer) RenderBufferFree(buffer *metadata.RenderBuffer, size, offset uint64) error {
	if buffer != nil {
		buffer.Buffer = make([]interface{}, size)
		buffer.InternalData = nil
		buffer.TotalSize = 0
	}
	return nil
}

func (vr *VulkanRenderer) RenderBufferLoadRange(buffer *metadata.RenderBuffer, offset, size uint64, data interface{}) error {
	if buffer == nil || buffer.InternalData == nil || size == 0 || data == nil {
		err := fmt.Errorf("vulkan_buffer_load_range requires a valid pointer to a buffer, a nonzero size and a valid pointer to data")
		return err
	}

	internalBuffer := buffer.InternalData.(*VulkanBuffer)
	if vr.vulkanBufferIsDeviceLocal(internalBuffer) && !vr.vulkanBufferIsHostVisible(internalBuffer) {
		// NOTE: If a staging buffer is needed (i.e.) the target buffer's memory is not host visible but is device-local,
		// create a staging buffer to load the data into first. Then copy from it to the target buffer.

		// Create a host-visible staging buffer to upload to. Mark it as the source of the transfer.
		staging, err := vr.RenderBufferCreate(metadata.RENDERBUFFER_TYPE_STAGING, size)
		if err != nil {
			err := fmt.Errorf("vulkan_buffer_load_range() - Failed to create staging buffer")
			return err
		}

		vr.RenderBufferBind(staging, 0)
		// Load the data into the staging buffer.
		vr.RenderBufferLoadRange(staging, 0, size, data)

		// Perform the copy from staging to the device local buffer.
		vr.RenderBufferCopyRange(staging, 0, buffer, offset, size)

		// Clean up the staging buffer.
		vr.RenderBufferUnbind(staging)
		vr.RenderBufferDestroy(staging)
	} else {
		// If no staging buffer is needed, map/copy/unmap.
		var dataPtr unsafe.Pointer
		if err := lockPool.SafeCall(MemoryManagement, func() error {
			if result := vk.MapMemory(vr.context.Device.LogicalDevice, internalBuffer.Memory, vk.DeviceSize(offset), vk.DeviceSize(size), 0, &dataPtr); !VulkanResultIsSuccess(result) {
				err := fmt.Errorf("%s", VulkanResultString(result, true))
				return err
			}
			return nil
		}); err != nil {
			return err
		}

		data = dataPtr

		if err := lockPool.SafeCall(MemoryManagement, func() error {
			vk.UnmapMemory(vr.context.Device.LogicalDevice, internalBuffer.Memory)
			return nil
		}); err != nil {
			return err
		}
	}

	return nil
}

func (vr *VulkanRenderer) RenderBufferCopyRange(source *metadata.RenderBuffer, sourceOffset uint64, dest *metadata.RenderBuffer, destOffset uint64, size uint64) error {
	if source == nil || source.InternalData == nil || dest == nil || dest.InternalData == nil || size == 0 {
		err := fmt.Errorf("vulkan_buffer_copy_range requires a valid pointers to source and destination buffers as well as a nonzero size")
		return err
	}

	return vr.vulkanBufferCopyRangeInternal(
		(source.InternalData.(*VulkanBuffer)).Handle,
		sourceOffset,
		(dest.InternalData.(*VulkanBuffer)).Handle,
		destOffset,
		size,
	)
}

// Indicates if the provided buffer has device-local memory.
func (vr *VulkanRenderer) vulkanBufferIsDeviceLocal(buffer *VulkanBuffer) bool {
	return (buffer.MemoryPropertyFlags & uint32(vk.MemoryPropertyDeviceLocalBit)) == uint32(vk.MemoryPropertyDeviceLocalBit)
}

// Indicates if the provided buffer has host-visible memory.
func (vr *VulkanRenderer) vulkanBufferIsHostVisible(buffer *VulkanBuffer) bool {
	return (buffer.MemoryPropertyFlags & uint32(vk.MemoryPropertyHostVisibleBit)) == uint32(vk.MemoryPropertyHostVisibleBit)
}

// Indicates if the provided buffer has host-coherent memory.
func (vr *VulkanRenderer) vulkanBufferIsHostCoherent(buffer *VulkanBuffer) bool {
	return (buffer.MemoryPropertyFlags & uint32(vk.MemoryPropertyHostCoherentBit)) == uint32(vk.MemoryPropertyHostCoherentBit)
}

func (vr *VulkanRenderer) vulkanBufferCopyRangeInternal(source vk.Buffer, sourceOffset uint64, dest vk.Buffer, destOffset, size uint64) error {
	// TODO: Assuming queue and pool usage here. Might want dedicated queue.
	queue := vr.context.Device.GraphicsQueue

	if err := lockPool.SafeQueueCall(vr.context.Device.GraphicsQueueIndex, func() error {
		if res := vk.QueueWaitIdle(queue); !VulkanResultIsSuccess(res) {
			err := fmt.Errorf("queue wait idle failed with error %s", VulkanResultString(res, true))
			return err
		}
		return nil
	}); err != nil {
		return err
	}

	// Create a one-time-use command buffer.
	tempCommandBuffer, err := AllocateAndBeginSingleUse(vr.context, vr.context.Device.GraphicsCommandPool)
	if err != nil {
		return err
	}

	// Prepare the copy command and add it to the command buffer.
	copyRegion := vk.BufferCopy{
		SrcOffset: vk.DeviceSize(sourceOffset),
		DstOffset: vk.DeviceSize(destOffset),
		Size:      vk.DeviceSize(size),
	}
	if err := lockPool.SafeCall(CommandBufferManagement, func() error {
		vk.CmdCopyBuffer(tempCommandBuffer.Handle, source, dest, 1, []vk.BufferCopy{copyRegion})
		return nil
	}); err != nil {
		return err
	}

	// Submit the buffer for execution and wait for it to complete.
	tempCommandBuffer.EndSingleUse(vr.context, vr.context.Device.GraphicsCommandPool, queue, vr.context.Device.GraphicsQueueIndex)

	return nil
}

func (vr *VulkanRenderer) RenderBufferDraw(buffer *metadata.RenderBuffer, offset uint64, elementCount uint32, bindOnly bool) error {
	commandBuffer := vr.context.GraphicsCommandBuffers[vr.context.ImageIndex]

	if buffer.RenderBufferType == metadata.RENDERBUFFER_TYPE_VERTEX {
		// Bind vertex buffer at offset.
		offsets := []vk.DeviceSize{vk.DeviceSize(offset)}
		if err := lockPool.SafeCall(CommandBufferManagement, func() error {
			vk.CmdBindVertexBuffers(commandBuffer.Handle, 0, 1, []vk.Buffer{buffer.InternalData.(*VulkanBuffer).Handle}, offsets)
			return nil
		}); err != nil {
			return err
		}

		if !bindOnly {
			if err := lockPool.SafeCall(CommandBufferManagement, func() error {
				vk.CmdDraw(commandBuffer.Handle, elementCount, 1, 0, 0)
				return nil
			}); err != nil {
				return err
			}
		}
		return nil
	} else if buffer.RenderBufferType == metadata.RENDERBUFFER_TYPE_INDEX {
		// Bind index buffer at offset.
		if err := lockPool.SafeCall(CommandBufferManagement, func() error {
			vk.CmdBindIndexBuffer(commandBuffer.Handle, (buffer.InternalData.(*VulkanBuffer)).Handle, vk.DeviceSize(offset), vk.IndexTypeUint32)
			return nil
		}); err != nil {
			return err
		}

		if !bindOnly {
			if err := lockPool.SafeCall(CommandBufferManagement, func() error {
				vk.CmdDrawIndexed(commandBuffer.Handle, elementCount, 1, 0, 0, 0)
				return nil
			}); err != nil {
				return err
			}
		}
		return nil
	} else {
		err := fmt.Errorf("cannot draw buffer of type: %d", buffer.RenderBufferType)
		return err
	}
}

func (vr *VulkanRenderer) WindowAttachmentGet(index uint8) *metadata.Texture {
	if index >= uint8(vr.context.Swapchain.ImageCount) {
		core.LogFatal("attempting to get colour attachment index out of range: %d. Attachment count: %d", index, vr.context.Swapchain.ImageCount)
		return nil
	}
	return vr.context.Swapchain.RenderTextures[index]
}

func (vr *VulkanRenderer) WindowAttachmentIndexGet() uint64 {
	return uint64(vr.context.ImageIndex)
}

func (vr *VulkanRenderer) DepthAttachmentGet(index uint8) *metadata.Texture {
	if index >= uint8(vr.context.Swapchain.ImageCount) {
		core.LogFatal("attempting to get depth attachment index out of range: %d. Attachment count: %d", index, vr.context.Swapchain.ImageCount)
		return nil
	}
	return vr.context.Swapchain.DepthTextures[index]
}

func (vr *VulkanRenderer) GetWindowAttachmentCount() uint8 {
	return uint8(vr.context.Swapchain.ImageCount)
}

func (vr *VulkanRenderer) convertRepeatType(axis string, repeat metadata.TextureRepeat) vk.SamplerAddressMode {
	switch repeat {
	case metadata.TextureRepeatRepeat:
		return vk.SamplerAddressModeRepeat
	case metadata.TextureRepeatMirroredRepeat:
		return vk.SamplerAddressModeMirroredRepeat
	case metadata.TextureRepeatClampToEdge:
		return vk.SamplerAddressModeClampToEdge
	case metadata.TextureRepeatClampToBorder:
		return vk.SamplerAddressModeClampToBorder
	default:
		core.LogWarn("convert_repeat_type(axis='%s') Type '%x' not supported, defaulting to repeat.", axis, repeat)
		return vk.SamplerAddressModeRepeat
	}
}

func (vr *VulkanRenderer) convertFilterType(op string, filter metadata.TextureFilter) vk.Filter {
	switch filter {
	case metadata.TextureFilterModeNearest:
		return vk.FilterNearest
	case metadata.TextureFilterModeLinear:
		return vk.FilterLinear
	default:
		core.LogWarn("convert_filter_type(op='%s'): Unsupported filter type '%x', defaulting to linear.", op, filter)
		return vk.FilterLinear
	}
}

func dbgCallbackFunc(flags vk.DebugReportFlags, objectType vk.DebugReportObjectType, object uint64, location uint64, messageCode int32, pLayerPrefix string, pMessage string, pUserData unsafe.Pointer) vk.Bool32 {
	switch {
	case flags&vk.DebugReportFlags(vk.DebugReportInformationBit) != 0:
		core.LogInfo("INFORMATION: [%s] Code %d : %s", pLayerPrefix, messageCode, pMessage)
	case flags&vk.DebugReportFlags(vk.DebugReportWarningBit) != 0:
		core.LogWarn("WARNING: [%s] Code %d : %s", pLayerPrefix, messageCode, pMessage)
	case flags&vk.DebugReportFlags(vk.DebugReportPerformanceWarningBit) != 0:
		core.LogWarn("PERFORMANCE WARNING: [%s] Code %d : %s", pLayerPrefix, messageCode, pMessage)
	case flags&vk.DebugReportFlags(vk.DebugReportErrorBit) != 0:
		core.LogError("ERROR: [%s] Code %d : %s", pLayerPrefix, messageCode, pMessage)
	case flags&vk.DebugReportFlags(vk.DebugReportDebugBit) != 0:
		core.LogInfo("DEBUG: [%s] Code %d : %s", pLayerPrefix, messageCode, pMessage)
	default:
		core.LogInfo("INFORMATION: [%s] Code %d : %s", pLayerPrefix, messageCode, pMessage)
	}
	return vk.Bool32(vk.False)
}
