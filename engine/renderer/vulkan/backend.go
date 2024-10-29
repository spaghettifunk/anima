package vulkan

import (
	"fmt"
	m "math"
	"runtime"
	"unsafe"

	"github.com/go-gl/glfw/v3.3/glfw"
	vk "github.com/goki/vulkan"
	"github.com/spaghettifunk/anima/engine/core"
	"github.com/spaghettifunk/anima/engine/platform"
	"github.com/spaghettifunk/anima/engine/renderer/metadata"
	"github.com/spaghettifunk/anima/engine/systems/loaders"
)

type VulkanRenderer struct {
	platform          *platform.Platform
	FrameNumber       uint64
	context           *VulkanContext
	FramebufferWidth  uint32
	FramebufferHeight uint32

	debug bool
}

func New(p *platform.Platform) *VulkanRenderer {
	return &VulkanRenderer{
		platform:    p,
		FrameNumber: 0,
		context: &VulkanContext{
			FramebufferWidth:  0,
			FramebufferHeight: 0,
			Allocator:         nil,
		},
		FramebufferWidth:  0,
		FramebufferHeight: 0,
		debug:             true,
	}
}

func (vr VulkanRenderer) Initialize(config *metadata.RendererBackendConfig) error {
	procAddr := glfw.GetVulkanGetInstanceProcAddress()
	if procAddr == nil {
		core.LogFatal("GetInstanceProcAddress is nil")
		return fmt.Errorf("GetInstanceProcAddress is nil")
	}
	vk.SetGetInstanceProcAddr(procAddr)

	if err := vk.Init(); err != nil {
		core.LogFatal("failed to initialize vk: %s", err)
		return err
	}

	// TODO: custom allocator.
	vr.context.Allocator = nil

	vr.FramebufferWidth = 0
	vr.FramebufferHeight = 0

	// Setup Vulkan instance.
	appInfo := &vk.ApplicationInfo{
		SType:              vk.StructureTypeApplicationInfo,
		ApiVersion:         uint32(vk.MakeVersion(1, 0, 0)),
		ApplicationVersion: uint32(vk.MakeVersion(1, 0, 0)),
		PApplicationName:   VulkanSafeString(""),
		PEngineName:        VulkanSafeString("Anima Engine"),
	}

	createInfo := vk.InstanceCreateInfo{
		SType:            vk.StructureTypeInstanceCreateInfo,
		PApplicationInfo: appInfo,
	}

	// Obtain a list of required extensions
	required_extensions := []string{"VK_KHR_surface"} // Generic surface extension
	en := vr.platform.GetRequiredExtensionNames()
	required_extensions = append(required_extensions, en...)

	if runtime.GOOS == "darwin" {
		required_extensions = append(required_extensions,
			"VK_KHR_portability_enumeration",
			"VK_KHR_get_physical_device_properties2",
		)
	}

	if vr.debug {
		required_extensions = append(required_extensions, vk.ExtDebugUtilsExtensionName, vk.ExtDebugReportExtensionName) // debug utilities
		core.LogInfo("Required extensions:")
		for i := 0; i < len(required_extensions); i++ {
			core.LogInfo(required_extensions[i])
		}
	}

	createInfo.EnabledExtensionCount = uint32(len(required_extensions))
	createInfo.PpEnabledExtensionNames = VulkanSafeStrings(required_extensions)

	// Validation layers.
	required_validation_layer_names := []string{}
	// var required_validation_layer_count uint32 = 0

	// If validation should be done, get a list of the required validation layert names
	// and make sure they exist. Validation layers should only be enabled on non-release builds.
	if vr.debug {
		core.LogInfo("Validation layers enabled. Enumerating...")

		// The list of validation layers required.
		required_validation_layer_names = []string{"VK_LAYER_KHRONOS_validation"}
		// required_validation_layer_count = uint32(len(required_validation_layer_names))

		if runtime.GOOS == "darwin" {
			createInfo.Flags |= 1
		}

		// Obtain a list of available validation layers
		var available_layer_count uint32
		if res := vk.EnumerateInstanceLayerProperties(&available_layer_count, nil); res != vk.Success {
			return nil
		}

		available_layers := make([]vk.LayerProperties, available_layer_count)
		if res := vk.EnumerateInstanceLayerProperties(&available_layer_count, available_layers); res != vk.Success {
			return nil
		}

		// Verify all required layers are available.
		for i := range required_validation_layer_names {
			core.LogInfo("Searching for layer: %s...", required_validation_layer_names[i])
			found := false
			for j := range available_layers {
				available_layers[j].Deref()
				core.LogInfo("Available Layer: `%s`", string(available_layers[j].LayerName[:]))
				end := FindFirstZeroInByteArray(available_layers[j].LayerName[:])
				if required_validation_layer_names[i] == vk.ToString(available_layers[j].LayerName[:end+1]) {
					found = true
					core.LogInfo("Found.")
					break
				}
			}

			if !found {
				core.LogFatal("Required validation layer is missing: %s", required_validation_layer_names[i])
				return nil
			}
		}
		core.LogInfo("All required validation layers are present.")
	}

	createInfo.EnabledLayerCount = uint32(len(required_validation_layer_names))
	createInfo.PpEnabledLayerNames = VulkanSafeStrings(required_validation_layer_names)

	if res := vk.CreateInstance(&createInfo, vr.context.Allocator, &vr.context.Instance); res != vk.Success {
		err := fmt.Errorf("failed in creating the Vulkan Instance with error `%s`", VulkanResultString(res, true))
		core.LogError(err.Error())
		return err
	}
	if err := vk.InitInstance(vr.context.Instance); err != nil {
		core.LogError(err.Error())
		return err
	}

	core.LogInfo("Vulkan Instance created.")

	// Debugger
	if vr.debug {
		core.LogDebug("Creating Vulkan debugger...")
		// log_severity := vk.DebugUtilsMessageSeverityErrorBit |
		// 	vk.DebugUtilsMessageSeverityWarningBit |
		// 	vk.DebugUtilsMessageSeverityInfoBit //|
		// 	//    VK_DEBUG_UTILS_MESSAGE_SEVERITY_VERBOSE_BIT_EXT;

		debugCreateInfo := vk.DebugReportCallbackCreateInfo{
			SType:       vk.StructureTypeDebugReportCallbackCreateInfo,
			Flags:       vk.DebugReportFlags(vk.DebugReportErrorBit | vk.DebugReportWarningBit | vk.DebugReportInformationBit),
			PfnCallback: dbgCallbackFunc,
			PNext:       nil,
		}

		var dbg vk.DebugReportCallback
		if err := vk.Error(vk.CreateDebugReportCallback(vr.context.Instance, &debugCreateInfo, nil, &dbg)); err != nil {
			core.LogError("vk.CreateDebugReportCallback failed with %s", err)
			return err
		}
		vr.context.debugMessenger = dbg

		core.LogDebug("Vulkan debugger created.")
	}

	// Surface
	core.LogDebug("Creating Vulkan surface...")
	surface := vr.createVulkanSurface()
	if surface == 0 {
		core.LogError("Failed to create platform surface!")
		return nil
	}
	vr.context.Surface = vk.SurfaceFromPointer(surface)
	core.LogDebug("Vulkan surface created.")

	// Device creation
	if err := DeviceCreate(vr.context); err != nil {
		core.LogError("Failed to create device!")
		return nil
	}

	// Swapchain
	sc, err := SwapchainCreate(vr.context, vr.context.FramebufferWidth, vr.context.FramebufferHeight)
	if err != nil {
		return nil
	}
	vr.context.Swapchain = sc

	rp, err := RenderpassCreate(
		vr.context,
		0, 0, float32(vr.context.FramebufferWidth), float32(vr.context.FramebufferHeight),
		0.0, 0.0, 0.2, 1.0,
		1.0,
		0)
	if err != nil {
		return nil
	}
	vr.context.MainRenderpass = rp

	// Swapchain framebuffers.
	vr.context.Swapchain.Framebuffers = make([]*VulkanFramebuffer, vr.context.Swapchain.ImageCount)
	if err := vr.regenerateFramebuffers(vr.context.Swapchain, vr.context.MainRenderpass); err != nil {
		return err
	}

	// Create command buffers.
	vr.createCommandBuffers()

	// Create sync objects.
	vr.context.ImageAvailableSemaphores = make([]vk.Semaphore, vr.context.Swapchain.MaxFramesInFlight)
	vr.context.QueueCompleteSemaphores = make([]vk.Semaphore, vr.context.Swapchain.MaxFramesInFlight)
	vr.context.InFlightFences = make([]*VulkanFence, vr.context.Swapchain.MaxFramesInFlight)

	for i := 0; i < int(vr.context.Swapchain.MaxFramesInFlight); i++ {
		semaphoreCreateInfo := vk.SemaphoreCreateInfo{
			SType: vk.StructureTypeSemaphoreCreateInfo,
		}
		// semaphoreCreateInfo.Deref()

		if res := vk.CreateSemaphore(vr.context.Device.LogicalDevice, &semaphoreCreateInfo, vr.context.Allocator, &vr.context.ImageAvailableSemaphores[i]); res != vk.Success {
			err := fmt.Errorf("failed to create semaphore on image available")
			core.LogError(err.Error())
			return err
		}

		if res := vk.CreateSemaphore(vr.context.Device.LogicalDevice, &semaphoreCreateInfo, vr.context.Allocator, &vr.context.QueueCompleteSemaphores[i]); res != vk.Success {
			err := fmt.Errorf("failed to create semaphore on queue complete")
			core.LogError(err.Error())
			return err
		}

		// Create the fence in a signaled state, indicating that the first frame has already been "rendered".
		// This will prevent the application from waiting indefinitely for the first frame to render since it
		// cannot be rendered until a frame is "rendered" before it.
		f, err := NewFence(vr.context, true)
		if err != nil {
			core.LogError(err.Error())
			return err
		}
		vr.context.InFlightFences[i] = f
	}

	// In flight fences should not yet exist at this point, so clear the list. These are stored in pointers
	// because the initial state should be 0, and will be 0 when not in use. Acutal fences are not owned
	// by this list.
	vr.context.ImagesInFlight = make([]*VulkanFence, vr.context.Swapchain.ImageCount)
	for i := 0; i < int(vr.context.Swapchain.ImageCount); i++ {
		vr.context.ImagesInFlight[i] = nil
	}

	core.LogInfo("Vulkan renderer initialized successfully.")

	return nil
}

func (vr VulkanRenderer) Shutdow() error {
	vk.DeviceWaitIdle(vr.context.Device.LogicalDevice)

	// Destroy in the opposite order of creation.

	// Sync objects
	for i := 0; i < int(vr.context.Swapchain.MaxFramesInFlight); i++ {
		if vr.context.ImageAvailableSemaphores[i] != vk.NullSemaphore {
			vk.DestroySemaphore(
				vr.context.Device.LogicalDevice,
				vr.context.ImageAvailableSemaphores[i],
				vr.context.Allocator)
			vr.context.ImageAvailableSemaphores[i] = vk.NullSemaphore
		}
		if vr.context.QueueCompleteSemaphores[i] != vk.NullSemaphore {
			vk.DestroySemaphore(
				vr.context.Device.LogicalDevice,
				vr.context.QueueCompleteSemaphores[i],
				vr.context.Allocator)
			vr.context.QueueCompleteSemaphores[i] = vk.NullSemaphore
		}
		vr.context.InFlightFences[i].FenceDestroy(vr.context)
	}

	vr.context.ImageAvailableSemaphores = nil
	vr.context.QueueCompleteSemaphores = nil
	vr.context.InFlightFences = nil
	vr.context.ImagesInFlight = nil

	// Command buffers
	for i := 0; i < int(vr.context.Swapchain.ImageCount); i++ {
		if vr.context.GraphicsCommandBuffers[i].Handle != nil {
			vr.context.GraphicsCommandBuffers[i].Free(vr.context, vr.context.Device.GraphicsCommandPool)
			vr.context.GraphicsCommandBuffers[i].Handle = nil
		}
	}
	vr.context.GraphicsCommandBuffers = nil

	// Destroy framebuffers
	for i := 0; i < int(vr.context.Swapchain.ImageCount); i++ {
		vr.context.Swapchain.Framebuffers[i].Destroy(vr.context)
	}

	// Renderpass
	vr.context.MainRenderpass.RenderpassDestroy(vr.context)

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
		}
	}

	core.LogDebug("Destroying Vulkan instance...")
	vk.DestroyInstance(vr.context.Instance, vr.context.Allocator)

	return nil
}

func (vr VulkanRenderer) Resized(width, height uint32) error {
	// Update the "framebuffer size generation", a counter which indicates when the
	// framebuffer size has been updated.
	vr.FramebufferWidth = width
	vr.FramebufferHeight = height
	vr.context.FramebufferSizeGeneration++

	core.LogInfo("Vulkan renderer backend.resized: w/h/gen: %d/%d/%d", width, height, vr.context.FramebufferSizeGeneration)
	return nil
}

func (vr VulkanRenderer) BeginFrame(deltaTime float64) error {
	device := vr.context.Device
	// Check if recreating swap chain and boot out.
	if vr.context.RecreatingSwapchain {
		result := vk.DeviceWaitIdle(device.LogicalDevice)
		if !VulkanResultIsSuccess(result) {
			err := fmt.Errorf("vulkan_renderer_backend_begin_frame vkDeviceWaitIdle (1) failed: '%s'", VulkanResultString(result, true))
			core.LogError(err.Error())
			return err
		}
		core.LogInfo("Recreating swapchain, booting.")
		return nil
	}

	// Check if the framebuffer has been resized. If so, a new swapchain must be created.
	if vr.context.FramebufferSizeGeneration != vr.context.FramebufferSizeLastGeneration {
		result := vk.DeviceWaitIdle(device.LogicalDevice)
		if !VulkanResultIsSuccess(result) {
			err := fmt.Errorf("vulkan_renderer_backend_begin_frame vkDeviceWaitIdle (2) failed: '%s'", VulkanResultString(result, true))
			core.LogError(err.Error())
			return err
		}

		// If the swapchain recreation failed (because, for example, the window was minimized),
		// boot out before unsetting the flag.
		if !vr.recreateSwapchain() {
			err := fmt.Errorf("failed to recreate the swapchain")
			core.LogError(err.Error())
			return err
		}

		core.LogInfo("Resized, booting.")
		return nil
	}

	// Wait for the execution of the current frame to complete. The fence being free will allow this one to move on.
	if !vr.context.InFlightFences[vr.context.CurrentFrame].FenceWait(vr.context, m.MaxUint32) {
		err := fmt.Errorf("in-flight fence wait failure")
		core.LogWarn(err.Error())
		return err
	}

	// Acquire the next image from the swap chain. Pass along the semaphore that should signaled when this completes.
	// This same semaphore will later be waited on by the queue submission to ensure this image is available.
	imageIndex, ok := vr.context.Swapchain.SwapchainAcquireNextImageIndex(vr.context, m.MaxUint64, vr.context.ImageAvailableSemaphores[vr.context.CurrentFrame], vk.NullFence)
	if !ok {
		err := fmt.Errorf("failed to swapchain aquire next image index")
		core.LogError(err.Error())
		return err
	}
	vr.context.ImageIndex = imageIndex

	// Begin recording commands.
	command_buffer := vr.context.GraphicsCommandBuffers[vr.context.ImageIndex]
	command_buffer.Reset()
	command_buffer.Begin(false, false, false)

	// Dynamic state
	viewport := vk.Viewport{
		X:        0.0,
		Y:        float32(vr.context.FramebufferHeight),
		Width:    float32(vr.context.FramebufferWidth),
		Height:   float32(vr.context.FramebufferHeight), // TODO: it was a negative value before
		MinDepth: 0.0,
		MaxDepth: 1.0,
	}

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

	vk.CmdSetViewport(command_buffer.Handle, 0, 1, []vk.Viewport{viewport})
	vk.CmdSetScissor(command_buffer.Handle, 0, 1, []vk.Rect2D{scissor})

	vr.context.MainRenderpass.W = float32(vr.context.FramebufferWidth)
	vr.context.MainRenderpass.H = float32(vr.context.FramebufferHeight)

	// Begin the render pass.
	vr.context.MainRenderpass.RenderpassBegin(command_buffer, vr.context.Swapchain.Framebuffers[vr.context.ImageIndex].Handle)

	return nil
}

func (vr VulkanRenderer) EndFrame(deltaTime float64) error {
	commandBuffer := vr.context.GraphicsCommandBuffers[vr.context.ImageIndex]

	// End renderpass
	vr.context.MainRenderpass.RenderpassEnd(commandBuffer)

	commandBuffer.End()

	// Make sure the previous frame is not using this image (i.e. its fence is being waited on)
	if vr.context.ImagesInFlight[vr.context.ImageIndex] != (*VulkanFence)(vk.NullHandle) { // was frame
		result := vk.WaitForFences(vr.context.Device.LogicalDevice, 1, []vk.Fence{vr.context.ImagesInFlight[vr.context.ImageIndex].Handle}, vk.True, m.MaxUint64)
		if !VulkanResultIsSuccess(result) {
			err := fmt.Errorf("vkWaitForFences error: %s", VulkanResultString(result, true))
			core.LogFatal(err.Error())
			return err
		}
	}

	// Mark the image fence as in-use by this frame.
	vr.context.ImagesInFlight[vr.context.ImageIndex] = vr.context.InFlightFences[vr.context.CurrentFrame]

	// Reset the fence for use on the next frame
	if res := vk.ResetFences(vr.context.Device.LogicalDevice, 1, []vk.Fence{vr.context.InFlightFences[vr.context.CurrentFrame].Handle}); res != vk.Success {
		err := fmt.Errorf("failed to reset fences")
		core.LogError(err.Error())
		return err
	}

	// Submit the queue and wait for the operation to complete.
	// Begin queue submission
	submit_info := vk.SubmitInfo{
		SType: vk.StructureTypeSubmitInfo,
	}

	// Command buffer(s) to be executed.
	submit_info.CommandBufferCount = 1
	submit_info.PCommandBuffers = []vk.CommandBuffer{commandBuffer.Handle}

	// The semaphore(s) to be signaled when the queue is complete.
	submit_info.SignalSemaphoreCount = 1
	submit_info.PSignalSemaphores = []vk.Semaphore{vr.context.QueueCompleteSemaphores[vr.context.CurrentFrame]}

	// Wait semaphore ensures that the operation cannot begin until the image is available.
	submit_info.WaitSemaphoreCount = 1
	submit_info.PWaitSemaphores = []vk.Semaphore{vr.context.ImageAvailableSemaphores[vr.context.CurrentFrame]}

	// Each semaphore waits on the corresponding pipeline stage to complete. 1:1 ratio.
	// VK_PIPELINE_STAGE_COLOR_ATTACHMENT_OUTPUT_BIT prevents subsequent colour attachment
	// writes from executing until the semaphore signals (i.e. one frame is presented at a time)
	flags := vk.PipelineStageFlags(vk.PipelineStageColorAttachmentOutputBit)
	submit_info.PWaitDstStageMask = []vk.PipelineStageFlags{flags}

	var fence vk.Fence
	if result := vk.QueueSubmit(vr.context.Device.GraphicsQueue, 1, []vk.SubmitInfo{submit_info}, fence); result != vk.Success {
		err := fmt.Errorf("vkQueueSubmit failed with result: %s", VulkanResultString(result, true))
		core.LogError(err.Error())
		return err
	}
	vr.context.InFlightFences[vr.context.CurrentFrame].Handle = fence

	commandBuffer.UpdateSubmitted()
	// End queue submission

	// Give the image back to the swapchain.
	vr.context.Swapchain.SwapchainPresent(
		vr.context,
		vr.context.Device.GraphicsQueue,
		vr.context.Device.PresentQueue,
		vr.context.QueueCompleteSemaphores[vr.context.CurrentFrame],
		vr.context.ImageIndex)

	return nil
}

func (vr VulkanRenderer) createCommandBuffers() error {
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

func (vr VulkanRenderer) regenerateFramebuffers(swapchain *VulkanSwapchain, renderpass *VulkanRenderpass) error {
	for i := 0; i < int(swapchain.ImageCount); i++ {
		// TODO: make this dynamic based on the currently configured attachments
		var attachment_count uint32 = 2
		attachments := []vk.ImageView{
			swapchain.Views[i],
			swapchain.DepthAttachment.View,
		}
		fb, err := FramebufferCreate(vr.context, renderpass, vr.context.FramebufferWidth, vr.context.FramebufferHeight, attachment_count, attachments)
		if err != nil {
			core.LogError("failed to execute framebuffer create function")
			return err
		}
		swapchain.Framebuffers[i] = fb
	}
	return nil
}

func (vr VulkanRenderer) recreateSwapchain() bool {
	// If already being recreated, do not try again.
	if vr.context.RecreatingSwapchain {
		core.LogDebug("recreate_swapchain called when already recreating. Booting.")
		return false
	}

	// Detect if the window is too small to be drawn to
	if vr.context.FramebufferWidth == 0 || vr.context.FramebufferHeight == 0 {
		core.LogDebug("recreate_swapchain called when window is < 1 in a dimension. Booting.")
		return false
	}

	// Mark as recreating if the dimensions are valid.
	vr.context.RecreatingSwapchain = true

	// Wait for any operations to complete.
	vk.DeviceWaitIdle(vr.context.Device.LogicalDevice)

	// Clear these out just in case.
	for i := 0; i < int(vr.context.Swapchain.ImageCount); i++ {
		vr.context.ImagesInFlight[i] = nil
	}

	// Requery support
	DeviceQuerySwapchainSupport(vr.context.Device.PhysicalDevice, vr.context.Surface, vr.context.Device.SwapchainSupport)
	DeviceDetectDepthFormat(vr.context.Device)

	sc, err := vr.context.Swapchain.SwapchainRecreate(vr.context, vr.FramebufferWidth, vr.FramebufferHeight)
	if err != nil {
		return false
	}
	vr.context.Swapchain = sc

	// Sync the framebuffer size with the cached sizes.
	vr.context.FramebufferWidth = vr.FramebufferWidth
	vr.context.FramebufferHeight = vr.FramebufferHeight
	vr.context.MainRenderpass.W = float32(vr.context.FramebufferWidth)
	vr.context.MainRenderpass.H = float32(vr.context.FramebufferHeight)
	vr.FramebufferWidth = 0
	vr.FramebufferHeight = 0

	// Update framebuffer size generation.
	vr.context.FramebufferSizeLastGeneration = vr.context.FramebufferSizeGeneration

	// cleanup swapchain
	for i := uint32(0); i < vr.context.Swapchain.ImageCount; i++ {
		vr.context.GraphicsCommandBuffers[i].Free(vr.context, vr.context.Device.GraphicsCommandPool)
	}

	// Framebuffers.
	// when SwapchainRecreate fuction is called, the Framebuffer is already destroyed and recreated empty
	// for i := uint32(0); i < vr.context.Swapchain.ImageCount; i++ {
	// 	vr.context.Swapchain.Framebuffers[i].Destroy(vr.context)
	// }

	vr.context.MainRenderpass.X = 0
	vr.context.MainRenderpass.Y = 0
	vr.context.MainRenderpass.W = float32(vr.context.FramebufferWidth)
	vr.context.MainRenderpass.H = float32(vr.context.FramebufferHeight)

	vr.regenerateFramebuffers(vr.context.Swapchain, vr.context.MainRenderpass)

	vr.createCommandBuffers()

	// Clear the recreating flag.
	vr.context.RecreatingSwapchain = false

	return true
}

func (vr VulkanRenderer) createVulkanSurface() uintptr {
	surface, err := vr.platform.Window.CreateWindowSurface(vr.context.Instance, nil)
	if err != nil {
		core.LogFatal("Vulkan surface creation failed.")
		return 0
	}
	return surface
}

func (vr VulkanRenderer) CreateGeometry(geometry *metadata.Geometry, vertex_size, vertexCount uint32, vertices interface{}, index_size uint32, indexCount uint32, indices []uint32) bool {
	if vertexCount == 0 || vertices == nil {
		core.LogError("vulkan_renderer_create_geometry requires vertex data, and none was supplied. VertexCount=%d, vertices=%p", vertexCount, vertices)
		return false
	}

	// Check if this is a re-upload. If it is, need to free old data afterward.
	isReupload := geometry.InternalID != loaders.InvalidID
	oldRange := &VulkanGeometryData{}

	var internal_data *VulkanGeometryData
	if isReupload {
		internal_data = &vr.context.Geometries[geometry.InternalID]

		// Take a copy of the old range.
		oldRange.IndexBufferOffset = internal_data.IndexBufferOffset
		oldRange.IndexCount = internal_data.IndexCount
		oldRange.IndexElementSize = internal_data.IndexElementSize
		oldRange.VertexBufferOffset = internal_data.VertexBufferOffset
		oldRange.VertexCount = internal_data.VertexCount
		oldRange.VertexElementSize = internal_data.VertexElementSize
	} else {
		for i := uint32(0); i < VULKAN_MAX_GEOMETRY_COUNT; i++ {
			if vr.context.Geometries[i].ID == loaders.InvalidID {
				// Found a free index.
				geometry.InternalID = i
				vr.context.Geometries[i].ID = i
				internal_data = &vr.context.Geometries[i]
				break
			}
		}
	}
	if internal_data == nil {
		core.LogFatal("vulkan_renderer_create_geometry failed to find a free index for a new geometry upload. Adjust config to allow for more.")
		return false
	}

	// Vertex data.
	internal_data.VertexCount = vertexCount
	internal_data.VertexElementSize = 0 //sizeof(vertex_3d);
	total_size := uint64(vertexCount * vertex_size)

	// Load the data.
	if !vr.RenderBufferLoadRange(&vr.context.ObjectVertexBuffer, internal_data.VertexBufferOffset, total_size, vertices) {
		core.LogError("vulkan_renderer_create_geometry failed to upload to the vertex buffer!")
		return false
	}

	// Index data, if applicable
	if indexCount > 0 && len(indices) > 0 {
		internal_data.IndexCount = indexCount
		internal_data.IndexElementSize = 0 //sizeof(u32)
		total_size = uint64(indexCount * index_size)

		if !vr.RenderBufferLoadRange(&vr.context.ObjectIndexBuffer, internal_data.IndexBufferOffset, total_size, indices) {
			core.LogError("vulkan_renderer_create_geometry failed to upload to the index buffer!")
			return false
		}
	}

	if internal_data.Generation == loaders.InvalidID {
		internal_data.Generation = 0
	} else {
		internal_data.Generation++
	}

	if isReupload {
		// Free vertex data
		if !vr.RenderBufferFree(&vr.context.ObjectVertexBuffer, uint64(oldRange.VertexElementSize*oldRange.VertexCount), oldRange.VertexBufferOffset) {
			core.LogError("vulkan_renderer_create_geometry free operation failed during reupload of vertex data.")
			return false
		}

		// Free index data, if applicable
		if oldRange.IndexElementSize > 0 {
			if !vr.RenderBufferFree(&vr.context.ObjectIndexBuffer, uint64(oldRange.IndexElementSize*oldRange.IndexCount), oldRange.IndexBufferOffset) {
				core.LogError("vulkan_renderer_create_geometry free operation failed during reupload of index data.")
				return false
			}
		}
	}

	return true
}

func (vr VulkanRenderer) TextureCreate(pixels []uint8, texture *metadata.Texture) {}

func (vr VulkanRenderer) TextureDestroy(texture *metadata.Texture) {}

func (vr VulkanRenderer) TextureCreateWriteable(texture *metadata.Texture) {}

func (vr VulkanRenderer) TextureResize(texture *metadata.Texture, new_width, new_height uint32) {}

func (vr VulkanRenderer) TextureWriteData(texture *metadata.Texture, offset, size uint32, pixels []uint8) {
}

func (vr VulkanRenderer) DestroyGeometry(geometry *metadata.Geometry) {}

func (vr VulkanRenderer) DrawGeometry(data *metadata.GeometryRenderData) {}

func (vr VulkanRenderer) RenderPassCreate(depth float32, stencil uint32, has_prev_pass, has_next_pass bool) (*metadata.RenderPass, error) {
	return nil, nil
}

func (vr VulkanRenderer) RenderpassDestroy(pass *metadata.RenderPass) {}

func (vr VulkanRenderer) RenderPassBegin(pass *metadata.RenderPass, target *metadata.RenderTarget) bool {
	return false
}

func (vr VulkanRenderer) RenderPassEnd(pass *metadata.RenderPass) bool { return false }

func (vr VulkanRenderer) RenderPassGet(name string) *metadata.RenderPass { return nil }

func (vr VulkanRenderer) ShaderCreate(shader *metadata.Shader, config *metadata.ShaderConfig, pass *metadata.RenderPass, stage_count uint8, stage_filenames []string, stages []metadata.ShaderStage) bool {
	return false
}

func (vr VulkanRenderer) ShaderDestroy(shader *metadata.Shader) {}

func (vr VulkanRenderer) ShaderInitialize(shader *metadata.Shader) bool { return false }

func (vr VulkanRenderer) ShaderUse(shader *metadata.Shader) bool { return false }

func (vr VulkanRenderer) ShaderBindGlobals(shader *metadata.Shader) bool { return false }

func (vr VulkanRenderer) ShaderBindInstance(shader *metadata.Shader, instance_id uint32) bool {
	return false
}

func (vr VulkanRenderer) ShaderApplyGlobals(shader *metadata.Shader) bool { return false }

func (vr VulkanRenderer) ShaderApplyInstance(shader *metadata.Shader, needs_update bool) bool {
	return false
}

func (vr VulkanRenderer) ShaderAcquireInstanceResources(shader *metadata.Shader, maps []*metadata.TextureMap) (out_instance_id uint32) {
	return 0
}

func (vr VulkanRenderer) ShaderReleaseInstanceResources(shader *metadata.Shader, instance_id uint32) bool {
	return false
}

func (vr VulkanRenderer) ShaderSetUniform(shader *metadata.Shader, uniform metadata.ShaderUniform, value interface{}) bool {
	return false
}

func (vr VulkanRenderer) TextureMapAcquireResources(texture_map *metadata.TextureMap) bool {
	return false
}

func (vr VulkanRenderer) TextureMapReleaseResources(texture_map *metadata.TextureMap) {}

func (vr VulkanRenderer) RenderTargetCreate(attachment_count uint8, attachments []*metadata.Texture, pass *metadata.RenderPass, width, height uint32) (out_target *metadata.RenderTarget) {
	return nil
}

func (vr VulkanRenderer) RenderTargetDestroy(target *metadata.RenderTarget, free_internal_memory bool) {
}

func (vr VulkanRenderer) IsMultithreaded() bool { return false }

func (vr VulkanRenderer) RenderBufferCreate(renderbufferType metadata.RenderBufferType, total_size uint64, use_freelist bool) *metadata.RenderBuffer {
	return nil
}

// vulkan_buffer_create_internal
func (vr VulkanRenderer) RenderBufferCreateInternal(buffer metadata.RenderBuffer) (*metadata.RenderBuffer, error) {
	return nil, nil
}

// vulkan_buffer_destroy_internal
func (vr VulkanRenderer) RenderBufferDestroyInternal(buffer *metadata.RenderBuffer) error {
	return nil
}

func (vr VulkanRenderer) RenderBufferDestroy(buffer *metadata.RenderBuffer) {}

func (vr VulkanRenderer) RenderBufferBind(buffer *metadata.RenderBuffer, offset uint64) bool {
	return false
}

func (vr VulkanRenderer) RenderBufferUnbind(buffer *metadata.RenderBuffer) bool { return false }

func (vr VulkanRenderer) RenderBufferMapMemory(buffer *metadata.RenderBuffer, offset, size uint64) interface{} {
	return nil
}

func (vr VulkanRenderer) RenderBufferUnmapMemory(buffer *metadata.RenderBuffer, offset, size uint64) {
}

func (vr VulkanRenderer) RenderBufferFlush(buffer *metadata.RenderBuffer, offset, size uint64) bool {
	return false
}

func (vr VulkanRenderer) RenderBufferRead(buffer *metadata.RenderBuffer, offset, size uint64) (out_memory []interface{}) {
	return nil
}

func (vr VulkanRenderer) RenderBufferResize(buffer *metadata.RenderBuffer, new_total_size uint64) bool {
	return false
}

func (vr VulkanRenderer) RenderBufferAllocate(buffer *metadata.RenderBuffer, size uint64) (out_offset uint64) {
	return 0
}

func (vr VulkanRenderer) RenderBufferFree(buffer *metadata.RenderBuffer, size, offset uint64) bool {
	return false
}

func (vr VulkanRenderer) RenderBufferLoadRange(buffer *metadata.RenderBuffer, offset, size uint64, data interface{}) bool {
	return false
}

func (vr VulkanRenderer) RenderBufferCopyRange(source *metadata.RenderBuffer, source_offset uint64, dest *metadata.RenderBuffer, dest_offset uint64, size uint64) bool {
	return false
}

func (vr VulkanRenderer) RenderBufferDraw(buffer *metadata.RenderBuffer, offset uint64, element_count uint32, bind_only bool) bool {
	return false
}

func (vr VulkanRenderer) WindowAttachmentGet(index uint8) *metadata.Texture {
	return nil
}

func (vr VulkanRenderer) WindowAttachmentIndexGet() uint64 {
	return 0
}

func (vr VulkanRenderer) DepthAttachmentGet() *metadata.Texture {
	return nil
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
