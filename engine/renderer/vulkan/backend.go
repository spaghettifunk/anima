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

func New(p *platform.Platform, am *assets.AssetManager) *VulkanRenderer {
	defaultTextures := metadata.NewDefaultTexture()
	defaultTextures.CreateSkeletonTextures()

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

	vr.FramebufferWidth = 800
	vr.FramebufferHeight = 600

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
			createInfo.Flags |= vk.InstanceCreateFlags(vk.InstanceCreateEnumeratePortabilityBit)
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
	createInfo.Deref()

	var instance vk.Instance
	if res := vk.CreateInstance(&createInfo, vr.context.Allocator, &instance); res != vk.Success {
		err := fmt.Errorf("failed in creating the Vulkan Instance with error `%s`", VulkanResultString(res, true))
		core.LogError(err.Error())
		return err
	}
	vr.context.Instance = instance

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
		fence_create_info := vk.FenceCreateInfo{
			SType: vk.StructureTypeFenceCreateInfo,
			Flags: vk.FenceCreateFlags(vk.FenceCreateSignaledBit),
		}
		var pFence vk.Fence
		if res := vk.CreateFence(vr.context.Device.LogicalDevice, &fence_create_info, vr.context.Allocator, &pFence); !VulkanResultIsSuccess(res) {
			err := fmt.Errorf("failed to create fence with error %s", VulkanResultString(res, true))
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
	vr.context.ObjectVertexBuffer, err = vr.RenderBufferCreate(metadata.RENDERBUFFER_TYPE_VERTEX, uint64(vertex_buffer_size), true)
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
	vr.context.ObjectIndexBuffer, err = vr.RenderBufferCreate(metadata.RENDERBUFFER_TYPE_INDEX, uint64(index_buffer_size), true)
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
	vk.DeviceWaitIdle(vr.context.Device.LogicalDevice)

	// Destroy in the opposite order of creation.
	// Destroy buffers
	vr.RenderBufferDestroy(vr.context.ObjectVertexBuffer)
	vr.RenderBufferDestroy(vr.context.ObjectIndexBuffer)

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
		vk.DestroyFence(vr.context.Device.LogicalDevice, vr.context.InFlightFences[i], vr.context.Allocator)
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
		}
	}

	core.LogDebug("Destroying Vulkan instance...")
	vk.DestroyInstance(vr.context.Instance, vr.context.Allocator)

	// Destroy the allocator callbacks if set.
	if vr.context.Allocator != nil {
		vr.context.Allocator = nil
	}

	return nil
}

func (vr *VulkanRenderer) Resized(width, height uint32) error {
	// Update the "framebuffer size generation", a counter which indicates when the
	// framebuffer size has been updated.
	vr.FramebufferWidth = width
	vr.FramebufferHeight = height
	vr.context.FramebufferSizeGeneration++

	core.LogInfo("Vulkan renderer backend.resized: w/h/gen: %d/%d/%d", width, height, vr.context.FramebufferSizeGeneration)
	return nil
}

func (vr *VulkanRenderer) BeginFrame(deltaTime float64) error {
	vr.context.FrameDeltaTime = float32(deltaTime)
	device := vr.context.Device

	// Check if recreating swap chain and boot out.
	if vr.context.RecreatingSwapchain {
		result := vk.DeviceWaitIdle(device.LogicalDevice)
		if !VulkanResultIsSuccess(result) {
			err := fmt.Errorf("func BeginFrame vkDeviceWaitIdle (1) failed: '%s'", VulkanResultString(result, true))
			return err
		}
		core.LogInfo("recreating swapchain, booting")
		return nil
	}

	// Check if the framebuffer has been resized. If so, a new swapchain must be created.
	if vr.context.FramebufferSizeGeneration != vr.context.FramebufferSizeLastGeneration {
		result := vk.DeviceWaitIdle(device.LogicalDevice)
		if !VulkanResultIsSuccess(result) {
			err := fmt.Errorf("func BeginFrame vkDeviceWaitIdle (2) failed: '%s'", VulkanResultString(result, true))
			return err
		}

		// If the swapchain recreation failed (because, for example, the window was minimized),
		// boot out before unsetting the flag.
		if !vr.recreateSwapchain() {
			err := fmt.Errorf("func BeginFrame failed to recreate the swapchain")
			return err
		}

		core.LogInfo("resized, booting.")
		return nil
	}

	// Wait for the execution of the current frame to complete. The fence being free will allow this one to move on.
	f := vr.context.InFlightFences[vr.context.CurrentFrame]
	core.LogDebug("fence: %+v", f)

	inFlightsFences := []vk.Fence{f}
	result := vk.WaitForFences(vr.context.Device.LogicalDevice, 1, inFlightsFences, vk.True, vk.MaxUint64)
	if !VulkanResultIsSuccess(result) {
		err := fmt.Errorf("func BeginFram In-flight fence wait failure! error: %s", VulkanResultString(result, true))
		return err
	}

	// Acquire the next image from the swap chain. Pass along the semaphore that should signaled when this completes.
	// This same semaphore will later be waited on by the queue submission to ensure this image is available.
	imageIndex, ok := vr.context.Swapchain.SwapchainAcquireNextImageIndex(vr.context, vk.MaxUint64, vr.context.ImageAvailableSemaphores[vr.context.CurrentFrame], vk.NullFence)
	if !ok {
		err := fmt.Errorf("failed to swapchain aquire next image index")
		return err
	}
	vr.context.ImageIndex = imageIndex

	// Begin recording commands.
	command_buffer := vr.context.GraphicsCommandBuffers[vr.context.ImageIndex]
	command_buffer.Reset()
	command_buffer.Begin(false, false, false)

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
		result := vk.WaitForFences(vr.context.Device.LogicalDevice, 1, []vk.Fence{vr.context.ImagesInFlight[vr.context.ImageIndex]}, vk.True, m.MaxUint64)
		if !VulkanResultIsSuccess(result) {
			err := fmt.Errorf("func EndFrame vkWaitForFences error: %s", VulkanResultString(result, true))
			return err
		}
	}

	// Mark the image fence as in-use by this frame.
	vr.context.ImagesInFlight[vr.context.ImageIndex] = vr.context.InFlightFences[vr.context.CurrentFrame]

	// Reset the fence for use on the next frame
	if res := vk.ResetFences(vr.context.Device.LogicalDevice, 1, []vk.Fence{vr.context.InFlightFences[vr.context.CurrentFrame]}); res != vk.Success {
		err := fmt.Errorf("func EndFrame failed to reset fences")
		return err
	}

	// Submit the queue and wait for the operation to complete.
	// Begin queue submission
	submit_info := vk.SubmitInfo{
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
	submit_info.Deref()

	// Each semaphore waits on the corresponding pipeline stage to complete. 1:1 ratio.
	// VK_PIPELINE_STAGE_COLOR_ATTACHMENT_OUTPUT_BIT prevents subsequent colour attachment
	// writes from executing until the semaphore signals (i.e. one frame is presented at a time)
	flags := vk.PipelineStageFlags(vk.PipelineStageColorAttachmentOutputBit)
	submit_info.PWaitDstStageMask = []vk.PipelineStageFlags{flags}

	if result := vk.QueueSubmit(vr.context.Device.GraphicsQueue, 1, []vk.SubmitInfo{submit_info}, vr.context.InFlightFences[vr.context.CurrentFrame]); result != vk.Success {
		err := fmt.Errorf("vkQueueSubmit failed with result: %s", VulkanResultString(result, true))
		core.LogError(err.Error())
		return err
	}

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

func (vr *VulkanRenderer) SetViewport() {
	// Dynamic state
	viewport := vk.Viewport{
		X:        vr.context.ViewportRect.X,
		Y:        vr.context.ViewportRect.Y,
		Width:    vr.context.ViewportRect.Z,
		Height:   vr.context.ViewportRect.W,
		MinDepth: 0.0,
		MaxDepth: 1.0,
	}
	command_buffer := vr.context.GraphicsCommandBuffers[vr.context.ImageIndex]
	vk.CmdSetViewport(command_buffer.Handle, 0, 1, []vk.Viewport{viewport})
}

func (vr *VulkanRenderer) ResetViewport() {
	vr.SetViewport()
}

func (vr *VulkanRenderer) SetScissor() {
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
	command_buffer := vr.context.GraphicsCommandBuffers[vr.context.ImageIndex]
	vk.CmdSetScissor(command_buffer.Handle, 0, 1, []vk.Rect2D{scissor})
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

func (vr *VulkanRenderer) recreateSwapchain() bool {
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

	return true
}

func (vr *VulkanRenderer) createVulkanSurface() uintptr {
	surface, err := vr.platform.Window.CreateWindowSurface(vr.context.Instance, nil)
	if err != nil {
		core.LogFatal("Vulkan surface creation failed.")
		return 0
	}
	return surface
}

func (vr *VulkanRenderer) CreateGeometry(geometry *metadata.Geometry, vertex_size, vertexCount uint32, vertices interface{}, index_size uint32, indexCount uint32, indices []uint32) bool {
	if vertexCount == 0 || vertices == nil {
		core.LogError("vulkan_renderer_create_geometry requires vertex data, and none was supplied. VertexCount=%d, vertices=%p", vertexCount, vertices)
		return false
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
		core.LogFatal("vulkan_renderer_create_geometry failed to find a free index for a new geometry upload. Adjust config to allow for more.")
		return false
	}

	// Vertex data.
	internalData.VertexCount = vertexCount
	internalData.VertexElementSize = uint32(unsafe.Sizeof(math.Vertex3D{}))
	total_size := uint64(vertexCount * vertex_size)

	// Load the data.
	if !vr.RenderBufferLoadRange(vr.context.ObjectVertexBuffer, internalData.VertexBufferOffset, total_size, vertices) {
		core.LogError("vulkan_renderer_create_geometry failed to upload to the vertex buffer!")
		return false
	}

	// Index data, if applicable
	if indexCount > 0 && len(indices) > 0 {
		internalData.IndexCount = indexCount
		internalData.IndexElementSize = uint32(unsafe.Sizeof(uint32(1)))
		total_size = uint64(indexCount * index_size)

		if !vr.RenderBufferLoadRange(vr.context.ObjectIndexBuffer, internalData.IndexBufferOffset, total_size, indices) {
			core.LogError("vulkan_renderer_create_geometry failed to upload to the index buffer!")
			return false
		}
	}

	if internalData.Generation == metadata.InvalidID {
		internalData.Generation = 0
	} else {
		internalData.Generation++
	}

	return true
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
	image_format := vk.FormatR8g8b8a8Unorm

	// NOTE: Lots of assumptions here, different texture types will require
	// different options here.
	image, err := ImageCreate(
		vr.context,
		texture.TextureType,
		texture.Width,
		texture.Height,
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

	texture.InternalData = image

	// Load the data.
	vr.TextureWriteData(texture, 0, size, pixels)

	texture.Generation++

	return nil
}

func (vr *VulkanRenderer) TextureDestroy(texture *metadata.Texture) {
	vk.DeviceWaitIdle(vr.context.Device.LogicalDevice)
	if texture.InternalData != nil {
		image := texture.InternalData.(*VulkanImage)
		if image != nil {
			image.Destroy(vr.context)
			texture.InternalData = nil
		}
	}
	texture = nil
}

func (vr *VulkanRenderer) channel_count_to_format(channel_count uint8, default_format vk.Format) vk.Format {
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
	var image_format vk.Format
	if (metadata.TextureFlag(texture.Flags) & metadata.TextureFlagDepth) != 0 {
		usage = vk.ImageUsageDepthStencilAttachmentBit
		aspect = vk.ImageAspectDepthBit
		image_format = vr.context.Device.DepthFormat
	} else {
		usage = vk.ImageUsageTransferSrcBit | vk.ImageUsageTransferDstBit | vk.ImageUsageSampledBit | vk.ImageUsageColorAttachmentBit
		aspect = vk.ImageAspectColorBit
		image_format = vr.channel_count_to_format(texture.ChannelCount, vk.FormatR8g8b8a8Unorm)
	}
	image, err := ImageCreate(vr.context, texture.TextureType, texture.Width, texture.Height, image_format, vk.ImageTilingOptimal, vk.ImageUsageFlags(usage),
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

		image_format := vr.channel_count_to_format(texture.ChannelCount, vk.FormatR8g8b8a8Unorm)

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

	image_format := vr.channel_count_to_format(texture.ChannelCount, vk.FormatR8g8b8a8Unorm)

	// Create a staging buffer and load data into it.
	staging, err := vr.RenderBufferCreate(metadata.RENDERBUFFER_TYPE_STAGING, uint64(size), false)
	if err != nil {
		err := fmt.Errorf("failed to create staging buffer for texture write")
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
		image_format,
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
		image_format,
		vk.ImageLayoutTransferDstOptimal,
		vk.ImageLayoutShaderReadOnlyOptimal,
	); err != nil {
		return err
	}

	if err := tempBuffer.EndSingleUse(vr.context, pool, queue); err != nil {
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
		if !VulkanResultIsSuccess(vk.DeviceWaitIdle(vr.context.Device.LogicalDevice)) {
			err := fmt.Errorf("failed to wait for device")
			return err
		}
		internal_data := vr.context.Geometries[geometry.InternalID]

		// Free vertex data
		if !vr.RenderBufferFree(vr.context.ObjectVertexBuffer, uint64(internal_data.VertexElementSize*internal_data.VertexCount), internal_data.VertexBufferOffset) {
			err := fmt.Errorf("vulkan_renderer_destroy_geometry failed to free vertex buffer range")
			return err
		}

		// Free index data, if applicable
		if internal_data.IndexElementSize > 0 {
			if !vr.RenderBufferFree(vr.context.ObjectIndexBuffer, uint64(internal_data.IndexElementSize*internal_data.IndexCount), internal_data.IndexBufferOffset) {
				err := fmt.Errorf("vulkan_renderer_destroy_geometry failed to free index buffer range")
				return err
			}
		}

		// Clean up data.
		internal_data.ID = metadata.InvalidID
		internal_data.Generation = metadata.InvalidID
	}
	return nil
}

func (vr *VulkanRenderer) DrawGeometry(data *metadata.GeometryRenderData) error {
	// Ignore non-uploaded geometries.
	if data.Geometry != nil && data.Geometry.InternalID == metadata.InvalidID {
		return nil
	}

	buffer_data := vr.context.Geometries[data.Geometry.InternalID]
	includes_index_data := buffer_data.IndexCount > 0
	if !vr.RenderBufferDraw(vr.context.ObjectVertexBuffer, buffer_data.VertexBufferOffset, buffer_data.VertexCount, includes_index_data) {
		err := fmt.Errorf("vulkan_renderer_draw_geometry failed to draw vertex buffer")
		return err
	}

	if includes_index_data {
		if !vr.RenderBufferDraw(vr.context.ObjectIndexBuffer, buffer_data.IndexBufferOffset, buffer_data.IndexCount, !includes_index_data) {
			err := fmt.Errorf("vulkan_renderer_draw_geometry failed to draw index buffer")
			return err
		}
	}
	return nil
}

func (vr *VulkanRenderer) RenderPassCreate(config *metadata.RenderPassConfig) (*metadata.RenderPass, error) {
	outRenderpass := &metadata.RenderPass{
		InternalData: &VulkanRenderPass{
			Depth:   config.Depth,
			Stencil: config.Stencil,
		},
	}
	internalData := outRenderpass.InternalData.(*VulkanRenderPass)

	// Main subpass
	subpass := vk.SubpassDescription{
		PipelineBindPoint: vk.PipelineBindPointGraphics,
	}

	// Attachments.
	attachmentDescriptions := make([]vk.AttachmentDescription, 1)
	colourAttachmentDescs := make([]vk.AttachmentDescription, 1)
	depthAttachmentDescs := make([]vk.AttachmentDescription, 1)

	// Can always just look at the first target since they are all the same (one per frame).
	for i := 0; i < len(config.Target.Attachments); i++ {
		attachment_config := config.Target.Attachments[i]

		attachment_desc := vk.AttachmentDescription{}
		if attachment_config.RenderTargetAttachmentType == metadata.RENDER_TARGET_ATTACHMENT_TYPE_COLOUR {
			// Colour attachment.
			do_clear_colour := (outRenderpass.ClearFlags & uint8(metadata.RENDERPASS_CLEAR_COLOUR_BUFFER_FLAG)) != 0

			if attachment_config.Source == metadata.RENDER_TARGET_ATTACHMENT_SOURCE_DEFAULT {
				attachment_desc.Format = vr.context.Swapchain.ImageFormat.Format
			} else {
				// TODO: configurable format?
				attachment_desc.Format = vk.FormatR8g8b8a8Unorm
			}

			attachment_desc.Samples = vk.SampleCount1Bit

			// Determine which load operation to use.
			if attachment_config.LoadOperation == metadata.RENDER_TARGET_ATTACHMENT_LOAD_OPERATION_DONT_CARE {
				// If we don't care, the only other thing that needs checking is if the attachment is being cleared.
				attachment_desc.LoadOp = vk.AttachmentLoadOpClear
				if !do_clear_colour {
					attachment_desc.LoadOp = vk.AttachmentLoadOpDontCare
				}
			} else {
				// If we are loading, check if we are also clearing. This combination doesn't make sense, and should be warned about.
				if attachment_config.LoadOperation == metadata.RENDER_TARGET_ATTACHMENT_LOAD_OPERATION_LOAD {
					if do_clear_colour {
						core.LogWarn("colour attachment load operation set to load, but is also set to clear. This combination is invalid, and will err toward clearing. Verify attachment configuration")
						attachment_desc.LoadOp = vk.AttachmentLoadOpClear
					} else {
						attachment_desc.LoadOp = vk.AttachmentLoadOpLoad
					}
				} else {
					core.LogFatal("Invalid and unsupported combination of load operation (0x%x) and clear flags (0x%x) for colour attachment.", attachment_desc.LoadOp, outRenderpass.ClearFlags)
					return nil, nil
				}
			}

			// Determine which store operation to use.
			if attachment_config.StoreOperation == metadata.RENDER_TARGET_ATTACHMENT_STORE_OPERATION_DONT_CARE {
				attachment_desc.StoreOp = vk.AttachmentStoreOpDontCare
			} else if attachment_config.StoreOperation == metadata.RENDER_TARGET_ATTACHMENT_STORE_OPERATION_STORE {
				attachment_desc.StoreOp = vk.AttachmentStoreOpStore
			} else {
				core.LogFatal("invalid store operation (0x%d) set for depth attachment. Check configuration", attachment_config.StoreOperation)
				return nil, nil
			}

			// NOTE: these will never be used on a colour attachment.
			attachment_desc.StencilLoadOp = vk.AttachmentLoadOpDontCare
			attachment_desc.StencilStoreOp = vk.AttachmentStoreOpDontCare
			// If loading, that means coming from another pass, meaning the format should be VK_IMAGE_LAYOUT_COLOR_ATTACHMENT_OPTIMAL. Otherwise it is undefined.
			attachment_desc.InitialLayout = vk.ImageLayoutColorAttachmentOptimal
			if attachment_config.LoadOperation != metadata.RENDER_TARGET_ATTACHMENT_LOAD_OPERATION_LOAD {
				attachment_desc.InitialLayout = vk.ImageLayoutUndefined
			}

			// If this is the last pass writing to this attachment, present after should be set to true.
			attachment_desc.FinalLayout = vk.ImageLayoutPresentSrc
			if !attachment_config.PresentAfter {
				attachment_desc.FinalLayout = vk.ImageLayoutColorAttachmentOptimal // Transitioned to after the render pass
			}
			attachment_desc.Flags = 0

			// Push to colour attachments array.
			colourAttachmentDescs = append(colourAttachmentDescs, attachment_desc)
		} else if attachment_config.RenderTargetAttachmentType == metadata.RENDER_TARGET_ATTACHMENT_TYPE_DEPTH {
			// Depth attachment.
			do_clear_depth := (outRenderpass.ClearFlags & uint8(metadata.RENDERPASS_CLEAR_DEPTH_BUFFER_FLAG)) != 0

			if attachment_config.Source == metadata.RENDER_TARGET_ATTACHMENT_SOURCE_DEFAULT {
				attachment_desc.Format = vr.context.Device.DepthFormat
			} else {
				// TODO: There may be a more optimal format to use when not the default depth target.
				attachment_desc.Format = vr.context.Device.DepthFormat
			}

			attachment_desc.Samples = vk.SampleCount1Bit
			// Determine which load operation to use.
			if attachment_config.LoadOperation == metadata.RENDER_TARGET_ATTACHMENT_LOAD_OPERATION_DONT_CARE {
				// If we don't care, the only other thing that needs checking is if the attachment is being cleared.
				attachment_desc.LoadOp = vk.AttachmentLoadOpClear
				if !do_clear_depth {
					attachment_desc.LoadOp = vk.AttachmentLoadOpDontCare
				}
			} else {
				// If we are loading, check if we are also clearing. This combination doesn't make sense, and should be warned about.
				if attachment_config.LoadOperation == metadata.RENDER_TARGET_ATTACHMENT_LOAD_OPERATION_LOAD {
					if do_clear_depth {
						core.LogWarn("depth attachment load operation set to load, but is also set to clear. This combination is invalid, and will err toward clearing. Verify attachment configuration")
						attachment_desc.LoadOp = vk.AttachmentLoadOpClear
					} else {
						attachment_desc.LoadOp = vk.AttachmentLoadOpLoad
					}
				} else {
					core.LogFatal("invalid and unsupported combination of load operation (0x%d) and clear flags (0x%d) for depth attachment.", attachment_desc.LoadOp, outRenderpass.ClearFlags)
					return nil, nil
				}
			}

			// Determine which store operation to use.
			if attachment_config.StoreOperation == metadata.RENDER_TARGET_ATTACHMENT_STORE_OPERATION_DONT_CARE {
				attachment_desc.StoreOp = vk.AttachmentStoreOpDontCare
			} else if attachment_config.StoreOperation == metadata.RENDER_TARGET_ATTACHMENT_STORE_OPERATION_STORE {
				attachment_desc.StoreOp = vk.AttachmentStoreOpStore
			} else {
				core.LogFatal("invalid store operation (0x%d) set for depth attachment. Check configuration", attachment_config.StoreOperation)
				return nil, nil
			}

			// TODO: Configurability for stencil attachments.
			attachment_desc.StencilLoadOp = vk.AttachmentLoadOpDontCare
			attachment_desc.StencilStoreOp = vk.AttachmentStoreOpDontCare
			// If coming from a previous pass, should already be VK_IMAGE_LAYOUT_DEPTH_STENCIL_ATTACHMENT_OPTIMAL. Otherwise undefined.
			attachment_desc.InitialLayout = vk.ImageLayoutDepthStencilAttachmentOptimal
			if attachment_config.LoadOperation != metadata.RENDER_TARGET_ATTACHMENT_LOAD_OPERATION_LOAD {
				attachment_desc.InitialLayout = vk.ImageLayoutUndefined
			}
			// Final layout for depth stencil attachments is always this.
			attachment_desc.FinalLayout = vk.ImageLayoutDepthStencilAttachmentOptimal

			// Push to colour attachments array.
			depthAttachmentDescs = append(depthAttachmentDescs, attachment_desc)
		}
		// Push to general array.
		attachmentDescriptions = append(attachmentDescriptions, attachment_desc)
	}

	// Setup the attachment references.
	attachments_added := uint32(0)

	// Colour attachment reference.
	colour_attachment_references := make([]vk.AttachmentReference, 0)
	colour_attachment_count := len(colourAttachmentDescs)
	if colour_attachment_count > 0 {
		colour_attachment_references = make([]vk.AttachmentReference, colour_attachment_count)
		for i := 0; i < colour_attachment_count; i++ {
			colour_attachment_references[i].Attachment = attachments_added // Attachment description array index
			colour_attachment_references[i].Layout = vk.ImageLayoutColorAttachmentOptimal
			attachments_added++
		}
		subpass.ColorAttachmentCount = uint32(colour_attachment_count)
		subpass.PColorAttachments = colour_attachment_references
	} else {
		subpass.ColorAttachmentCount = 0
		subpass.PColorAttachments = nil
	}

	// Depth attachment reference.
	depth_attachment_references := make([]vk.AttachmentReference, 0)
	depth_attachment_count := len(depthAttachmentDescs)
	if depth_attachment_count > 0 {
		if depth_attachment_count > 1 {
			core.LogFatal("multiple depth attachments not supported")
			return nil, nil
		}
		// Depth attachment reference
		depth_attachment_reference := vk.AttachmentReference{
			Attachment: 1,
			Layout:     vk.ImageLayoutDepthStencilAttachmentOptimal,
		}
		// Depth stencil data.
		subpass.PDepthStencilAttachment = &depth_attachment_reference
	} else {
		subpass.PDepthStencilAttachment = nil
	}

	// Input from a shader
	subpass.InputAttachmentCount = 0
	subpass.PInputAttachments = nil

	// Attachments used for multisampling colour attachments
	subpass.PResolveAttachments = nil

	// Attachments not used in this subpass, but must be preserved for the next.
	subpass.PreserveAttachmentCount = 0
	subpass.PPreserveAttachments = nil

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

	// Render pass create.
	render_pass_create_info := vk.RenderPassCreateInfo{
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
	render_pass_create_info.Deref()

	result := vk.CreateRenderPass(vr.context.Device.LogicalDevice, &render_pass_create_info, vr.context.Allocator, &internalData.Handle)
	if !VulkanResultIsSuccess(result) {
		err := fmt.Errorf("%s", VulkanResultString(result, true))
		return nil, err
	}

	// Cleanup
	if len(attachmentDescriptions) > 0 {
		attachmentDescriptions = nil
	}

	if len(colourAttachmentDescs) > 0 {
		colourAttachmentDescs = nil
	}
	if len(colour_attachment_references) > 0 {
		colour_attachment_references = nil
	}

	if len(depthAttachmentDescs) > 0 {
		depthAttachmentDescs = nil
	}
	if len(depth_attachment_references) > 0 {
		depth_attachment_references = nil
	}

	return nil, nil
}

func (vr *VulkanRenderer) RenderPassDestroy(pass *metadata.RenderPass) {
	// Destroy its rendertargets.
	for i := 0; i < int(pass.RenderTargetCount); i++ {
		target := pass.Targets[i]
		if target != nil && target.InternalFramebuffer != nil {
			buff := target.InternalFramebuffer.(vk.Framebuffer)
			vk.DestroyFramebuffer(vr.context.Device.LogicalDevice, buff, vr.context.Allocator)
			target.InternalFramebuffer = nil
			target.Attachments = nil
			target.AttachmentCount = 0
		}
	}
}

func (vr *VulkanRenderer) RenderPassBegin(pass *metadata.RenderPass, target *metadata.RenderTarget) bool {
	command_buffer := vr.context.GraphicsCommandBuffers[vr.context.ImageIndex]

	// Begin the render pass.
	internal_data := pass.InternalData.(*VulkanRenderPass)

	begin_info := vk.RenderPassBeginInfo{
		SType:       vk.StructureTypeRenderPassBeginInfo,
		RenderPass:  internal_data.Handle,
		Framebuffer: target.InternalFramebuffer.(vk.Framebuffer),
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

	clear_values := make([]vk.ClearValue, 2)

	do_clear_colour := (pass.ClearFlags & uint8(metadata.RENDERPASS_CLEAR_COLOUR_BUFFER_FLAG)) != 0
	if do_clear_colour {
		clear_values[begin_info.ClearValueCount].SetColor([]float32{pass.ClearColour.X, pass.ClearColour.Y, pass.ClearColour.Z, pass.ClearColour.W})
		begin_info.ClearValueCount++
	} else {
		// Still add it anyway, but don't bother copying data since it will be ignored.
		begin_info.ClearValueCount++
	}

	do_clear_depth := (pass.ClearFlags & uint8(metadata.RENDERPASS_CLEAR_DEPTH_BUFFER_FLAG)) != 0
	if do_clear_depth {
		clear_values[begin_info.ClearValueCount].SetColor([]float32{pass.ClearColour.X, pass.ClearColour.Y, pass.ClearColour.Z, pass.ClearColour.W})
		do_clear_stencil := (pass.ClearFlags & uint8(metadata.RENDERPASS_CLEAR_STENCIL_BUFFER_FLAG)) != 0
		if do_clear_stencil {
			clear_values[begin_info.ClearValueCount].SetDepthStencil(internal_data.Depth, internal_data.Stencil)
		} else {
			clear_values[begin_info.ClearValueCount].SetDepthStencil(internal_data.Depth, 0)
		}
		begin_info.ClearValueCount++
	} else {
		for i := 0; i < len(target.Attachments); i++ {
			if target.Attachments[i].RenderTargetAttachmentType == metadata.RENDER_TARGET_ATTACHMENT_TYPE_DEPTH {
				// If there is a depth attachment, make sure to add the clear count, but don't bother copying the data.
				begin_info.ClearValueCount++
			}
		}
	}

	if begin_info.ClearValueCount > 0 {
		begin_info.PClearValues = clear_values
	}
	begin_info.Deref()

	vk.CmdBeginRenderPass(command_buffer.Handle, &begin_info, vk.SubpassContentsInline)

	command_buffer.State = COMMAND_BUFFER_STATE_IN_RENDER_PASS

	return true
}

func (vr *VulkanRenderer) RenderPassEnd(pass *metadata.RenderPass) bool {
	command_buffer := vr.context.GraphicsCommandBuffers[vr.context.ImageIndex]
	// End the renderpass.
	vk.CmdEndRenderPass(command_buffer.Handle)
	command_buffer.State = COMMAND_BUFFER_STATE_RECORDING

	return true
}

func (vr *VulkanRenderer) TextureReadData(texture *metadata.Texture, offset, size uint32) (interface{}, error) {
	image := texture.InternalData.(*VulkanImage)
	image_format := vr.channel_count_to_format(texture.ChannelCount, vk.FormatR8g8b8a8Unorm)
	// Create a staging buffer and load data into it.
	staging, err := vr.RenderBufferCreate(metadata.RENDERBUFFER_TYPE_READ, uint64(size), false)
	if err != nil {
		core.LogError("failed to create staging buffer for texture read")
		return nil, err
	}

	vr.RenderBufferBind(staging, 0)

	pool := vr.context.Device.GraphicsCommandPool
	queue := vr.context.Device.GraphicsQueue

	temp_buffer, err := AllocateAndBeginSingleUse(vr.context, pool)
	if err != nil {
		return nil, err
	}

	// NOTE: transition to VK_IMAGE_LAYOUT_TRANSFER_SRC_OPTIMAL
	// Transition the layout from whatever it is currently to optimal for handing out data.
	if err := image.ImageTransitionLayout(
		vr.context,
		texture.TextureType,
		temp_buffer,
		image_format,
		vk.ImageLayoutUndefined,
		vk.ImageLayoutTransferSrcOptimal); err != nil {
		return nil, err
	}
	// Copy the data to the buffer.
	buff := staging.InternalData.(*VulkanBuffer)
	if err := image.ImageCopyFromBuffer(vr.context, texture.TextureType, buff.Handle, temp_buffer); err != nil {
		return nil, err
	}
	// Transition from optimal for data reading to shader-read-only optimal layout.
	if err := image.ImageTransitionLayout(
		vr.context,
		texture.TextureType,
		temp_buffer,
		image_format,
		vk.ImageLayoutTransferSrcOptimal,
		vk.ImageLayoutShaderReadOnlyOptimal); err != nil {
		return nil, err
	}

	if err := temp_buffer.EndSingleUse(vr.context, pool, queue); err != nil {
		return nil, err
	}
	out_memory, err := vr.RenderBufferRead(staging, uint64(offset), uint64(size))
	if err != nil {
		core.LogError("vulkan_buffer_read failed.")
		return nil, err
	}
	if !vr.RenderBufferUnbind(staging) {
		err := fmt.Errorf("failed to unbind renderbuffer")
		return nil, err
	}

	vr.RenderBufferDestroy(staging)

	return out_memory, nil
}

func (vr *VulkanRenderer) TextureReadPixel(texture *metadata.Texture, x, y uint32) ([]uint8, error) {
	image := texture.InternalData.(*VulkanImage)
	image_format := vr.channel_count_to_format(texture.ChannelCount, vk.FormatR8g8b8a8Unorm)
	// TODO: creating a buffer every time isn't great. Could optimize this by creating a buffer once
	// and just reusing it.
	// Create a staging buffer and load data into it.
	staging, err := vr.RenderBufferCreate(metadata.RENDERBUFFER_TYPE_READ, uint64(unsafe.Sizeof(uint8(1))*4), false)
	if err != nil {
		core.LogError("failed to create staging buffer for texture pixel read")
		return nil, err
	}

	vr.RenderBufferBind(staging, 0)

	pool := vr.context.Device.GraphicsCommandPool
	queue := vr.context.Device.GraphicsQueue

	temp_buffer, err := AllocateAndBeginSingleUse(vr.context, pool)
	if err != nil {
		return nil, err
	}

	// NOTE: transition to VK_IMAGE_LAYOUT_TRANSFER_SRC_OPTIMAL
	// Transition the layout from whatever it is currently to optimal for handing out data.
	if err := image.ImageTransitionLayout(
		vr.context,
		texture.TextureType,
		temp_buffer,
		image_format,
		vk.ImageLayoutUndefined,
		vk.ImageLayoutTransferSrcOptimal); err != nil {
		return nil, err
	}

	// Copy the data to the buffer.
	buff := staging.InternalData.(*VulkanBuffer)
	image.ImageCopyPixelToBuffer(vr.context, texture.TextureType, buff.Handle, x, y, temp_buffer)

	// Transition from optimal for data reading to shader-read-only optimal layout.
	if err := image.ImageTransitionLayout(
		vr.context,
		texture.TextureType,
		temp_buffer,
		image_format,
		vk.ImageLayoutTransferSrcOptimal,
		vk.ImageLayoutShaderReadOnlyOptimal); err != nil {
		return nil, err
	}

	if err := temp_buffer.EndSingleUse(vr.context, pool, queue); err != nil {
		return nil, err
	}

	out_rgba, err := vr.RenderBufferRead(staging, 0, uint64(unsafe.Sizeof(uint8(1))*4))
	if err != nil {
		core.LogError("vulkan_buffer_read failed.")
		return nil, err
	}
	if !vr.RenderBufferUnbind(staging) {
		err := fmt.Errorf("failed to unbind renderbuffer")
		return nil, err
	}

	vr.RenderBufferDestroy(staging)

	return out_rgba.([]uint8), nil
}

func (vr *VulkanRenderer) ShaderCreate(shader *metadata.Shader, config *metadata.ShaderConfig, pass *metadata.RenderPass, stageCount uint8, stageFilenames []string, stages []metadata.ShaderStage) bool {
	shader.InternalData = &VulkanShader{
		Config: &VulkanShaderConfig{
			PoolSizes:      make([]vk.DescriptorPoolSize, 2),
			DescriptorSets: make([]*VulkanDescriptorSetConfig, 2),
			Attributes:     make([]vk.VertexInputAttributeDescription, len(config.Attributes)),
			Stages:         make([]VulkanShaderStageConfig, len(config.Stages)),
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
		internalShader.Config.Stages[i].Stage = stageFlag
		internalShader.Config.Stages[i].FileName = stageFilenames[i]
	}

	// Zero out arrays and counts.
	internalShader.Config.DescriptorSets[0].SamplerBindingIndex = metadata.InvalidIDUint8
	internalShader.Config.DescriptorSets[1].SamplerBindingIndex = metadata.InvalidIDUint8

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
			setConfig.Bindings[setConfig.BindingCount] = vk.DescriptorSetLayoutBinding{
				Binding:         uint32(setConfig.BindingCount),
				DescriptorCount: 1,
				DescriptorType:  vk.DescriptorTypeUniformBuffer,
				StageFlags:      vk.ShaderStageFlags(vk.ShaderStageVertexBit) | vk.ShaderStageFlags(vk.ShaderStageFragmentBit),
			}
			setConfig.Bindings[setConfig.BindingCount].Deref()
			setConfig.BindingCount++
		}
		// Add a binding for Samplers if used.
		if internalShader.GlobalUniformSamplerCount > 0 {
			setConfig.Bindings[setConfig.BindingCount] = vk.DescriptorSetLayoutBinding{
				Binding:         uint32(setConfig.BindingCount),
				DescriptorCount: uint32(internalShader.GlobalUniformSamplerCount), // One descriptor per sampler.
				DescriptorType:  vk.DescriptorTypeCombinedImageSampler,
				StageFlags:      vk.ShaderStageFlags(vk.ShaderStageVertexBit) | vk.ShaderStageFlags(vk.ShaderStageFragmentBit),
			}
			setConfig.Bindings[setConfig.BindingCount].Deref()
			setConfig.SamplerBindingIndex = setConfig.BindingCount
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
			setConfig.Bindings = make([]vk.DescriptorSetLayoutBinding, 1)
		}

		if internalShader.InstanceUniformCount > 0 {
			setConfig.Bindings[setConfig.BindingCount] = vk.DescriptorSetLayoutBinding{
				Binding:         uint32(setConfig.BindingCount),
				DescriptorCount: 1,
				DescriptorType:  vk.DescriptorTypeUniformBuffer,
				StageFlags:      vk.ShaderStageFlags(vk.ShaderStageVertexBit) | vk.ShaderStageFlags(vk.ShaderStageFragmentBit),
			}
			setConfig.Bindings[setConfig.BindingCount].Deref()
			setConfig.BindingCount++
		}
		// Add a binding for Samplers if used.
		if internalShader.InstanceUniformSamplerCount > 0 {
			setConfig.Bindings[setConfig.BindingCount] = vk.DescriptorSetLayoutBinding{
				Binding:         uint32(setConfig.BindingCount),
				DescriptorCount: uint32(internalShader.InstanceUniformSamplerCount), // One descriptor per sampler.
				DescriptorType:  vk.DescriptorTypeCombinedImageSampler,
				StageFlags:      vk.ShaderStageFlags(vk.ShaderStageVertexBit) | vk.ShaderStageFlags(vk.ShaderStageFragmentBit),
			}
			setConfig.Bindings[setConfig.BindingCount].Deref()
			setConfig.SamplerBindingIndex = setConfig.BindingCount
			setConfig.BindingCount++
		}
		// Increment the set counter.
		descriptorSetCount++
	}

	// Invalidate all instance states.
	// TODO: dynamic
	for i := 0; i < 1024; i++ {
		if internalShader.InstanceStates[i] == nil {
			internalShader.InstanceStates[i] = &VulkanShaderInstanceState{
				ID: metadata.InvalidID,
			}
			continue
		}
		internalShader.InstanceStates[i].ID = metadata.InvalidID
	}

	// Keep a copy of the cull mode.
	internalShader.Config.CullMode = config.CullMode

	return true
}

func (vr *VulkanRenderer) ShaderDestroy(s *metadata.Shader) {
	if s != nil && s.InternalData != nil {
		shader := s.InternalData.(*VulkanShader)
		if shader != nil {
			core.LogError("vulkan_renderer_shader_destroy requires a valid pointer to a shader.")
			return
		}

		logical_device := vr.context.Device.LogicalDevice
		vk_allocator := vr.context.Allocator

		// Descriptor set layouts.
		for i := 0; i < len(shader.Config.DescriptorSets); i++ {
			if shader.DescriptorSetLayouts[i] != vk.NullDescriptorSetLayout {
				vk.DestroyDescriptorSetLayout(logical_device, shader.DescriptorSetLayouts[i], vk_allocator)
				shader.DescriptorSetLayouts[i] = nil
			}
		}

		// Descriptor pool
		if shader.DescriptorPool != nil {
			vk.DestroyDescriptorPool(logical_device, shader.DescriptorPool, vk_allocator)
		}

		// Uniform buffer.
		vr.RenderBufferUnmapMemory(shader.UniformBuffer, 0, vk.WholeSize)
		shader.MappedUniformBufferBlock = 0
		vr.RenderBufferDestroy(shader.UniformBuffer)

		// Pipeline
		shader.Pipeline.Destroy(vr.context)

		// Shader modules
		for i := 0; i < len(shader.Config.Stages); i++ {
			vk.DestroyShaderModule(vr.context.Device.LogicalDevice, shader.Stages[i].Handle, vr.context.Allocator)
		}

		// Destroy the configuration.
		shader.Config = nil

		// Free the internal data memory.
		s.InternalData = nil
	}
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
		if err := vr.createModule(internalShader, internalShader.Config.Stages[i], internalShader.Stages[i]); err != nil {
			core.LogError("Unable to create %s shader module for '%s'. Shader will be destroyed", internalShader.Config.Stages[i].FileName, shader.Name)
			return err
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
	pool_info := vk.DescriptorPoolCreateInfo{
		SType:         vk.StructureTypeDescriptorPoolCreateInfo,
		PoolSizeCount: 2,
		PPoolSizes:    internalShader.Config.PoolSizes,
		MaxSets:       uint32(internalShader.Config.MaxDescriptorSetCount),
		Flags:         vk.DescriptorPoolCreateFlags(vk.DescriptorPoolCreateFreeDescriptorSetBit),
	}
	pool_info.Deref()

	// Create descriptor pool.
	var pDescriptorPool vk.DescriptorPool
	result := vk.CreateDescriptorPool(logicalDevice, &pool_info, vkAllocator, &pDescriptorPool)
	if !VulkanResultIsSuccess(result) {
		err := fmt.Errorf("vulkan_shader_initialize failed creating descriptor pool: '%s'", VulkanResultString(result, true))
		return err
	}
	internalShader.DescriptorPool = pDescriptorPool

	// Create descriptor set layouts.
	internalShader.DescriptorSetLayouts = make([]vk.DescriptorSetLayout, 2)
	for i := 0; i < len(internalShader.Config.DescriptorSets); i++ {
		layout_info := vk.DescriptorSetLayoutCreateInfo{
			SType:        vk.StructureTypeDescriptorSetLayoutCreateInfo,
			BindingCount: uint32(internalShader.Config.DescriptorSets[i].BindingCount),
			PBindings:    internalShader.Config.DescriptorSets[i].Bindings,
		}
		layout_info.Deref()

		var pSetLayout vk.DescriptorSetLayout
		result = vk.CreateDescriptorSetLayout(logicalDevice, &layout_info, vkAllocator, &pSetLayout)
		if !VulkanResultIsSuccess(result) {
			err := fmt.Errorf("vulkan_shader_initialize failed creating descriptor pool: '%s'", VulkanResultString(result, true))
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

	stage_create_infos := make([]vk.PipelineShaderStageCreateInfo, len(internalShader.Config.Stages))
	for i := 0; i < len(internalShader.Config.Stages); i++ {
		stage_create_infos[i] = internalShader.Stages[i].ShaderStageCreateInfo
		stage_create_infos[i].Deref()
	}

	pipConfig := &VulkanPipelineConfig{
		Renderpass:           internalShader.Renderpass,
		Stride:               uint32(shader.AttributeStride),
		Attributes:           internalShader.Config.Attributes,
		DescriptorSetLayouts: internalShader.DescriptorSetLayouts,
		Stages:               stage_create_infos,
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
	total_buffer_size := shader.GlobalUboStride + (shader.UboStride * uint64(VULKAN_MAX_MATERIAL_COUNT)) // global + (locals)
	internalShader.UniformBuffer, err = vr.RenderBufferCreate(metadata.RENDERBUFFER_TYPE_UNIFORM, total_buffer_size, true)
	if err != nil {
		core.LogError("Vulkan buffer creation failed for object shader.")
		return err
	}
	vr.RenderBufferBind(internalShader.UniformBuffer, 0)

	// Map the entire buffer's memory.
	internalShader.MappedUniformBufferBlock = vr.RenderBufferMapMemory(internalShader.UniformBuffer, 0, vk.WholeSize)

	// Allocate global descriptor sets, one per frame. Global is always the first set.
	global_layouts := []vk.DescriptorSetLayout{
		internalShader.DescriptorSetLayouts[DESC_SET_INDEX_GLOBAL],
		internalShader.DescriptorSetLayouts[DESC_SET_INDEX_GLOBAL],
		internalShader.DescriptorSetLayouts[DESC_SET_INDEX_GLOBAL],
	}

	alloc_info := vk.DescriptorSetAllocateInfo{
		SType:              vk.StructureTypeDescriptorSetAllocateInfo,
		DescriptorPool:     internalShader.DescriptorPool,
		DescriptorSetCount: 3,
		PSetLayouts:        global_layouts,
	}
	alloc_info.Deref()

	internalShader.GlobalDescriptorSets = make([]vk.DescriptorSet, 3)
	for i := 0; i < len(internalShader.GlobalDescriptorSets); i++ {
		gds := internalShader.GlobalDescriptorSets[i]
		result = vk.AllocateDescriptorSets(vr.context.Device.LogicalDevice, &alloc_info, &gds)
		if !VulkanResultIsSuccess(result) {
			err := fmt.Errorf("%s", VulkanResultString(result, true))
			return err
		}
		internalShader.GlobalDescriptorSets[i] = gds // not necessary in theory but hey...
	}

	return nil
}

func (vr *VulkanRenderer) createModule(shader *VulkanShader, config VulkanShaderStageConfig, shader_stage *VulkanShaderStage) error {
	// Read the resource.
	binary_resource, err := vr.assetManager.LoadAsset(config.FileName, metadata.ResourceTypeBinary, nil)
	if err != nil {
		return err
	}

	shader_stage.CreateInfo = vk.ShaderModuleCreateInfo{
		SType:    vk.StructureTypeShaderModuleCreateInfo,
		CodeSize: binary_resource.DataSize * 4,
		PCode:    binary_resource.Data.([]uint32),
	}

	var shaderModule vk.ShaderModule
	result := vk.CreateShaderModule(vr.context.Device.LogicalDevice, &shader_stage.CreateInfo, vr.context.Allocator, &shaderModule)
	if !VulkanResultIsSuccess(result) {
		err := fmt.Errorf("%s", VulkanResultString(result, true))
		return err
	}
	shader_stage.Handle = shaderModule

	// Release the resource.
	vr.assetManager.UnloadAsset(binary_resource)

	// Shader stage info
	shader_stage.ShaderStageCreateInfo = vk.PipelineShaderStageCreateInfo{
		SType:               vk.StructureTypePipelineShaderStageCreateInfo,
		Stage:               config.Stage,
		Module:              shader_stage.Handle,
		PName:               VulkanSafeString("main"),
		PSpecializationInfo: nil,
		PNext:               vk.NullHandle,
		Flags:               0,
	}

	return nil
}

func (vr *VulkanRenderer) ShaderUse(shader *metadata.Shader) bool {
	s := shader.InternalData.(*VulkanShader)
	s.Pipeline.Bind(vr.context.GraphicsCommandBuffers[vr.context.ImageIndex], vk.PipelineBindPointGraphics)
	return true
}

func (vr *VulkanRenderer) ShaderBindGlobals(shader *metadata.Shader) bool {
	if shader == nil {
		return false
	}
	shader.BoundUboOffset = uint32(shader.GlobalUboOffset)
	return true
}

func (vr *VulkanRenderer) ShaderBindInstance(shader *metadata.Shader, instance_id uint32) bool {
	if shader == nil {
		return false
	}

	internal := shader.InternalData.(*VulkanShader)

	shader.BoundInstanceID = instance_id
	state := internal.InstanceStates[instance_id]
	shader.BoundUboOffset = uint32(state.Offset)

	return true
}

func (vr *VulkanRenderer) ShaderApplyGlobals(shader *metadata.Shader) bool {
	image_index := vr.context.ImageIndex
	internal := shader.InternalData.(*VulkanShader)
	command_buffer := vr.context.GraphicsCommandBuffers[image_index].Handle
	global_descriptor := internal.GlobalDescriptorSets[image_index]

	// Apply UBO first
	bufferInfo := vk.DescriptorBufferInfo{
		Buffer: (internal.UniformBuffer.InternalData.(*VulkanBuffer)).Handle,
		Offset: vk.DeviceSize(shader.GlobalUboOffset),
		Range:  vk.DeviceSize(shader.GlobalUboStride),
	}
	bufferInfo.Deref()

	// Update descriptor sets.
	ubo_write := vk.WriteDescriptorSet{
		SType:           vk.StructureTypeWriteDescriptorSet,
		DstSet:          internal.GlobalDescriptorSets[image_index],
		DstBinding:      0,
		DstArrayElement: 0,
		DescriptorType:  vk.DescriptorTypeUniformBuffer,
		DescriptorCount: 1,
		PBufferInfo:     []vk.DescriptorBufferInfo{bufferInfo},
	}
	ubo_write.Deref()

	descriptor_writes := make([]vk.WriteDescriptorSet, 1)
	descriptor_writes[0] = ubo_write

	global_set_binding_count := uint32(internal.Config.DescriptorSets[DESC_SET_INDEX_GLOBAL].BindingCount)
	if global_set_binding_count > 1 {
		// TODO: There are samplers to be written. Support this.
		global_set_binding_count = 1
		core.LogError("Global image samplers are not yet supported.")

		// VkWriteDescriptorSet sampler_write = {VK_STRUCTURE_TYPE_WRITE_DESCRIPTOR_SET};
		// descriptor_writes[1] = ...
	}

	// var pDescriptorCopies []vk.CopyDescriptorSet
	vk.UpdateDescriptorSets(vr.context.Device.LogicalDevice, global_set_binding_count, descriptor_writes, 0, nil)

	// Bind the global descriptor set to be updated.
	vk.CmdBindDescriptorSets(command_buffer, vk.PipelineBindPointGraphics, internal.Pipeline.PipelineLayout, 0, 1, []vk.DescriptorSet{global_descriptor}, 0, nil)
	return true
}

func (vr *VulkanRenderer) ShaderApplyInstance(shader *metadata.Shader, needs_update bool) bool {
	internal := shader.InternalData.(*VulkanShader)
	if internal.InstanceUniformCount < 1 && internal.InstanceUniformSamplerCount < 1 {
		core.LogError("This shader does not use instances.")
		return false
	}
	image_index := vr.context.ImageIndex
	command_buffer := vr.context.GraphicsCommandBuffers[image_index].Handle

	// Obtain instance data.
	object_state := internal.InstanceStates[shader.BoundInstanceID]
	object_descriptor_set := object_state.DescriptorSetState.DescriptorSets[image_index]

	if needs_update {
		descriptor_writes := make([]vk.WriteDescriptorSet, 2) // Always a max of 2 descriptor sets.

		descriptor_count := uint32(0)
		descriptor_index := uint32(0)

		buffer_info := vk.DescriptorBufferInfo{}

		// Descriptor 0 - Uniform buffer
		if internal.InstanceUniformCount > 0 {
			// Only do this if the descriptor has not yet been updated.
			// instance_ubo_generation := object_state.DescriptorSetState.DescriptorSets[descriptor_index] //.generations[image_index]
			// TODO: determine if update is required.
			// if *instance_ubo_generation == metadata.InvalidIDUint8 {
			buffer_info.Buffer = (internal.UniformBuffer.InternalData.(*VulkanBuffer)).Handle
			buffer_info.Offset = vk.DeviceSize(object_state.Offset)
			buffer_info.Range = vk.DeviceSize(shader.UboStride)

			ubo_descriptor := vk.WriteDescriptorSet{
				SType:           vk.StructureTypeWriteDescriptorSet,
				DstSet:          object_descriptor_set,
				DstBinding:      descriptor_index,
				DescriptorType:  vk.DescriptorTypeUniformBuffer,
				DescriptorCount: 1,
				PBufferInfo:     []vk.DescriptorBufferInfo{buffer_info},
			}

			descriptor_writes[descriptor_count] = ubo_descriptor
			descriptor_count++

			// Update the frame generation. In this case it is only needed once since this is a buffer.
			// *instance_ubo_generation = 1 // material.generation; TODO: some generation from... somewhere
			// }
			descriptor_index++
		}

		// Iterate samplers.
		if internal.InstanceUniformSamplerCount > 0 {
			sampler_binding_index := internal.Config.DescriptorSets[DESC_SET_INDEX_INSTANCE].SamplerBindingIndex
			total_sampler_count := internal.Config.DescriptorSets[DESC_SET_INDEX_INSTANCE].Bindings[sampler_binding_index].DescriptorCount
			update_sampler_count := uint32(0)
			image_infos := make([]vk.DescriptorImageInfo, VULKAN_SHADER_MAX_GLOBAL_TEXTURES)

			for i := uint32(0); i < total_sampler_count; i++ {
				// TODO: only update in the list if actually needing an update.
				texture_map := internal.InstanceStates[shader.BoundInstanceID].InstanceTextureMaps[i]
				texture := texture_map.Texture

				// Ensure the texture is valid.
				if texture.Generation == metadata.InvalidID {
					switch texture_map.Use {
					case metadata.TextureUseMapDiffuse:
						texture = vr.defaultTexture.DefaultDiffuseTexture
					case metadata.TextureUseMapSpecular:
						texture = vr.defaultTexture.DefaultSpecularTexture
					case metadata.TextureUseMapNormal:
						texture = vr.defaultTexture.DefaultNormalTexture
					default:
						core.LogWarn("Undefined texture use %d", texture_map.Use)
						texture = vr.defaultTexture.DefaultTexture
					}
				}

				image := texture.InternalData.(*VulkanImage)
				image_infos[i].ImageLayout = vk.ImageLayoutShaderReadOnlyOptimal
				image_infos[i].ImageView = image.View
				image_infos[i].Sampler = texture_map.InternalData.(vk.Sampler)

				// TODO: change up descriptor state to handle this properly.
				// Sync frame generation if not using a default texture.
				// if (t.generation != INVALID_ID) {
				//     *descriptor_generation = t.generation;
				//     *descriptor_id = t.id;
				// }

				update_sampler_count++
			}

			sampler_descriptor := vk.WriteDescriptorSet{
				SType:           vk.StructureTypeWriteDescriptorSet,
				DstSet:          object_descriptor_set,
				DstBinding:      descriptor_index,
				DescriptorType:  vk.DescriptorTypeCombinedImageSampler,
				DescriptorCount: update_sampler_count,
				PImageInfo:      image_infos,
			}
			sampler_descriptor.Deref()

			descriptor_writes[descriptor_count] = sampler_descriptor
			descriptor_count++
		}

		if descriptor_count > 0 {
			vk.UpdateDescriptorSets(vr.context.Device.LogicalDevice, descriptor_count, descriptor_writes, 0, nil)
		}
	}

	// Bind the descriptor set to be updated, or in case the shader changed.
	vk.CmdBindDescriptorSets(command_buffer, vk.PipelineBindPointGraphics, internal.Pipeline.PipelineLayout, 1, 1, []vk.DescriptorSet{object_descriptor_set}, 0, nil)

	return true
}

func (vr *VulkanRenderer) ShaderAcquireInstanceResources(shader *metadata.Shader, maps []*metadata.TextureMap) (uint32, error) {
	internal := shader.InternalData.(*VulkanShader)
	// TODO: dynamic
	out_instance_id := metadata.InvalidID
	for i := uint32(0); i < 1024; i++ {
		if internal.InstanceStates[i].ID == metadata.InvalidID {
			internal.InstanceStates[i].ID = i
			out_instance_id = i
			break
		}
	}
	if out_instance_id == metadata.InvalidID {
		err := fmt.Errorf("vulkan_shader_acquire_instance_resources failed to acquire new id")
		return 0, err
	}

	instance_state := internal.InstanceStates[out_instance_id]
	sampler_binding_index := internal.Config.DescriptorSets[DESC_SET_INDEX_INSTANCE].SamplerBindingIndex
	instance_texture_count := internal.Config.DescriptorSets[DESC_SET_INDEX_INSTANCE].Bindings[sampler_binding_index].DescriptorCount

	// Only setup if the shader actually requires it.
	if shader.InstanceTextureCount > 0 {
		instance_state.InstanceTextureMaps = make([]metadata.TextureMap, shader.InstanceTextureCount)
		for i := uint32(0); i < instance_texture_count; i++ {
			if maps[i].Texture != nil {
				instance_state.InstanceTextureMaps[i].Texture = vr.defaultTexture.DefaultTexture
				instance_state.InstanceTextureMaps[i].InternalData = *new(vk.Sampler)
			}
		}
	}

	// Allocate some space in the UBO - by the stride, not the size.
	internal.UniformBuffer.Buffer = make([]interface{}, uint32(m.Max(1, float64(shader.UboStride))))
	set_state := instance_state.DescriptorSetState

	// Each descriptor binding in the set
	binding_count := internal.Config.DescriptorSets[DESC_SET_INDEX_INSTANCE].BindingCount
	for i := uint32(0); i < uint32(binding_count); i++ {
		if len(set_state.DescriptorStates) == 0 {
			set_state.DescriptorStates = make([]*VulkanDescriptorState, 3)
		}
		if set_state.DescriptorStates[i] == nil {
			set_state.DescriptorStates[i] = &VulkanDescriptorState{
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

	alloc_info := vk.DescriptorSetAllocateInfo{
		SType:              vk.StructureTypeDescriptorSetAllocateInfo,
		DescriptorPool:     internal.DescriptorPool,
		DescriptorSetCount: uint32(len(layouts)),
		PSetLayouts:        layouts,
	}
	alloc_info.Deref()

	for i := 0; i < len(instance_state.DescriptorSetState.DescriptorSets); i++ {
		var pDescriptorSets vk.DescriptorSet
		result := vk.AllocateDescriptorSets(vr.context.Device.LogicalDevice, &alloc_info, &pDescriptorSets)
		if !VulkanResultIsSuccess(result) {
			err := fmt.Errorf("error allocating instance descriptor sets in shader: '%s'", VulkanResultString(result, true))
			return 0, err
		}
		instance_state.DescriptorSetState.DescriptorSets[i] = pDescriptorSets
	}

	return out_instance_id, nil
}

func (vr *VulkanRenderer) ShaderReleaseInstanceResources(shader *metadata.Shader, instance_id uint32) bool {
	internal := shader.InternalData.(*VulkanShader)
	instance_state := internal.InstanceStates[instance_id]

	// Wait for any pending operations using the descriptor set to finish.
	vk.DeviceWaitIdle(vr.context.Device.LogicalDevice)

	// Free 3 descriptor sets (one per frame)
	for _, ds := range instance_state.DescriptorSetState.DescriptorSets {
		result := vk.FreeDescriptorSets(vr.context.Device.LogicalDevice, internal.DescriptorPool, 1, &ds)
		if !VulkanResultIsSuccess(result) {
			core.LogError("Error freeing object shader descriptor sets!")
		}
	}

	// Destroy descriptor states.
	instance_state.DescriptorSetState.DescriptorStates = nil
	instance_state.InstanceTextureMaps = nil

	if !vr.RenderBufferFree(internal.UniformBuffer, shader.UboStride, instance_state.Offset) {
		core.LogError("vulkan_renderer_shader_release_instance_resources failed to free range from renderbuffer.")
	}
	instance_state.Offset = metadata.InvalidIDUint64
	instance_state.ID = metadata.InvalidID

	return true
}

func (vr *VulkanRenderer) SetUniform(shader *metadata.Shader, uniform metadata.ShaderUniform, value interface{}) bool {
	internal := shader.InternalData.(*VulkanShader)
	if uniform.ShaderUniformType == metadata.ShaderUniformTypeSampler {
		if uniform.Scope == metadata.ShaderScopeGlobal {
			shader.GlobalTextureMaps[uniform.Location] = value.(*metadata.TextureMap)
		} else {
			internal.InstanceStates[shader.BoundInstanceID].InstanceTextureMaps[uniform.Location] = value.(metadata.TextureMap)
		}
	} else {
		if uniform.Scope == metadata.ShaderScopeLocal {
			// Is local, using push constants. Do this immediately.
			command_buffer := vr.context.GraphicsCommandBuffers[vr.context.ImageIndex].Handle
			vk.CmdPushConstants(command_buffer, internal.Pipeline.PipelineLayout, vk.ShaderStageFlags(vk.ShaderStageVertexBit)|vk.ShaderStageFlags(vk.ShaderStageFragmentBit),
				uint32(uniform.Offset), uint32(uniform.Size), unsafe.Pointer(&value),
			)
		} else {
			// Map the appropriate memory location and copy the data over.
			addr := internal.MappedUniformBufferBlock.(uint64)
			addr += uint64(shader.BoundUboOffset) + uniform.Offset
		}
	}
	return true
}

func (vr *VulkanRenderer) TextureMapAcquireResources(texture_map *metadata.TextureMap) bool {
	// Create a sampler for the texture
	sampler_info := vk.SamplerCreateInfo{
		SType: vk.StructureTypeSamplerCreateInfo,

		MinFilter: vr.convertFilterType("min", texture_map.FilterMinify),
		MagFilter: vr.convertFilterType("mag", texture_map.FilterMagnify),

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
	sampler_info.Deref()

	var pSampler vk.Sampler
	result := vk.CreateSampler(vr.context.Device.LogicalDevice, &sampler_info, vr.context.Allocator, &pSampler)
	if !VulkanResultIsSuccess(result) {
		core.LogError("Error creating texture sampler: %s", VulkanResultString(result, true))
		return false
	}
	texture_map.InternalData = pSampler

	return true
}

func (vr *VulkanRenderer) TextureMapReleaseResources(texture_map *metadata.TextureMap) {
	if texture_map != nil {
		// Make sure there's no way this is in use.
		vk.DeviceWaitIdle(vr.context.Device.LogicalDevice)
		vk.DestroySampler(vr.context.Device.LogicalDevice, texture_map.InternalData.(vk.Sampler), vr.context.Allocator)
		texture_map.InternalData = 0
	}
}

func (vr *VulkanRenderer) RenderTargetCreate(attachment_count uint8, attachments []*metadata.RenderTargetAttachment, pass *metadata.RenderPass, width, height uint32) (*metadata.RenderTarget, error) {
	// Max number of attachments
	attachment_views := make([]vk.ImageView, 32)
	for i := uint32(0); i < uint32(attachment_count); i++ {
		attachment_views[i] = (attachments[i].Texture.InternalData.(*VulkanImage)).View
	}

	// Take a copy of the attachments and count.
	out_target := &metadata.RenderTarget{
		AttachmentCount: attachment_count,
		Attachments:     attachments,
	}

	framebuffer_create_info := vk.FramebufferCreateInfo{
		SType:           vk.StructureTypeFramebufferCreateInfo,
		RenderPass:      (pass.InternalData.(*VulkanRenderPass)).Handle,
		AttachmentCount: uint32(attachment_count),
		PAttachments:    attachment_views,
		Width:           width,
		Height:          height,
		Layers:          1,
	}

	// fb := out_target.InternalFramebuffer
	var fb vk.Framebuffer
	result := vk.CreateFramebuffer(vr.context.Device.LogicalDevice, &framebuffer_create_info, vr.context.Allocator, &fb)
	if !VulkanResultIsSuccess(result) {
		err := fmt.Errorf("%s", VulkanResultString(result, true))
		return nil, err
	}
	out_target.InternalFramebuffer = fb

	return out_target, nil
}

func (vr *VulkanRenderer) RenderTargetDestroy(target *metadata.RenderTarget, free_internal_memory bool) {
	if target != nil && target.InternalFramebuffer != nil {
		fb := target.InternalFramebuffer.(*vk.Framebuffer)
		vk.DestroyFramebuffer(vr.context.Device.LogicalDevice, *fb, vr.context.Allocator)
		target.InternalFramebuffer = nil
		if free_internal_memory {
			target.Attachments = nil
			target.AttachmentCount = 0
		}
	}
}

func (vr *VulkanRenderer) IsMultithreaded() bool {
	return vr.context.MultithreadingEnabled
}

func (vr *VulkanRenderer) RenderBufferCreate(renderbufferType metadata.RenderBufferType, total_size uint64, use_freelist bool) (*metadata.RenderBuffer, error) {
	out_buffer := &metadata.RenderBuffer{
		RenderBufferType: renderbufferType,
		TotalSize:        total_size,
	}

	internal_buffer := &VulkanBuffer{}

	switch out_buffer.RenderBufferType {
	case metadata.RENDERBUFFER_TYPE_VERTEX:
		internal_buffer.Usage = vk.BufferUsageFlags(vk.BufferUsageVertexBufferBit) | vk.BufferUsageFlags(vk.BufferUsageTransferDstBit) | vk.BufferUsageFlags(vk.BufferUsageTransferSrcBit)
		internal_buffer.MemoryPropertyFlags = uint32(vk.MemoryPropertyDeviceLocalBit)
	case metadata.RENDERBUFFER_TYPE_INDEX:
		internal_buffer.Usage = vk.BufferUsageFlags(vk.BufferUsageIndexBufferBit) | vk.BufferUsageFlags(vk.BufferUsageTransferDstBit) | vk.BufferUsageFlags(vk.BufferUsageTransferSrcBit)
		internal_buffer.MemoryPropertyFlags = uint32(vk.MemoryPropertyDeviceLocalBit)
	case metadata.RENDERBUFFER_TYPE_UNIFORM:
		device_local_bits := uint32(vk.MemoryPropertyDeviceLocalBit)
		if vr.context.Device.SupportsDeviceLocalHostVisible {
			device_local_bits = 0
		}
		internal_buffer.Usage = vk.BufferUsageFlags(vk.BufferUsageUniformBufferBit) | vk.BufferUsageFlags(vk.BufferUsageTransferDstBit)
		internal_buffer.MemoryPropertyFlags = uint32(vk.MemoryPropertyHostVisibleBit) | uint32(vk.MemoryPropertyHostCoherentBit) | device_local_bits
	case metadata.RENDERBUFFER_TYPE_STAGING:
		internal_buffer.Usage = vk.BufferUsageFlags(vk.BufferUsageTransferSrcBit)
		internal_buffer.MemoryPropertyFlags = uint32(vk.MemoryPropertyHostVisibleBit) | uint32(vk.MemoryPropertyHostCoherentBit)
	case metadata.RENDERBUFFER_TYPE_READ:
		internal_buffer.Usage = vk.BufferUsageFlags(vk.BufferUsageTransferDstBit)
		internal_buffer.MemoryPropertyFlags = uint32(vk.MemoryPropertyHostVisibleBit) | uint32(vk.MemoryPropertyHostCoherentBit)
	case metadata.RENDERBUFFER_TYPE_STORAGE:
		err := fmt.Errorf("storage buffer not yet supported")
		return nil, err
	default:
		err := fmt.Errorf("unsupported buffer type: %d", out_buffer.RenderBufferType)
		return nil, err
	}

	buffer_info := vk.BufferCreateInfo{
		SType:       vk.StructureTypeBufferCreateInfo,
		Size:        vk.DeviceSize(out_buffer.TotalSize),
		Usage:       internal_buffer.Usage,
		SharingMode: vk.SharingModeExclusive, // NOTE: Only used in one queue.
	}
	buffer_info.Deref()

	var pBuffer vk.Buffer
	result := vk.CreateBuffer(vr.context.Device.LogicalDevice, &buffer_info, vr.context.Allocator, &pBuffer)
	if !VulkanResultIsSuccess(result) {
		err := fmt.Errorf("%s", VulkanResultString(result, true))
		return nil, err
	}
	internal_buffer.Handle = pBuffer

	// Gather memory requirements.
	var pMemoryRequirements vk.MemoryRequirements
	vk.GetBufferMemoryRequirements(vr.context.Device.LogicalDevice, internal_buffer.Handle, &pMemoryRequirements)
	pMemoryRequirements.Deref()
	internal_buffer.MemoryRequirements = pMemoryRequirements

	internal_buffer.MemoryIndex = vr.context.FindMemoryIndex(internal_buffer.MemoryRequirements.MemoryTypeBits, internal_buffer.MemoryPropertyFlags)
	if internal_buffer.MemoryIndex == -1 {
		err := fmt.Errorf("unable to create vulkan buffer because the required memory type index was not found")
		return nil, err
	}

	// Allocate memory info
	allocate_info := vk.MemoryAllocateInfo{
		SType:           vk.StructureTypeMemoryAllocateInfo,
		AllocationSize:  internal_buffer.MemoryRequirements.Size,
		MemoryTypeIndex: uint32(internal_buffer.MemoryIndex),
	}
	allocate_info.Deref()

	// Allocate the memory.
	var mem vk.DeviceMemory
	result = vk.AllocateMemory(vr.context.Device.LogicalDevice, &allocate_info, vr.context.Allocator, &mem)
	if !VulkanResultIsSuccess(result) {
		err := fmt.Errorf("%s", VulkanResultString(result, true))
		return nil, err
	}
	internal_buffer.Memory = mem

	// Allocate the internal state block of memory at the end once we are sure everything was created successfully.
	out_buffer.InternalData = internal_buffer

	return out_buffer, nil
}

// vulkan_buffer_create_internal
func (vr *VulkanRenderer) RenderBufferCreateInternal(buffer metadata.RenderBuffer) (*metadata.RenderBuffer, error) {
	return nil, nil
}

// vulkan_buffer_destroy_internal
func (vr *VulkanRenderer) RenderBufferDestroyInternal(buffer *metadata.RenderBuffer) error {
	return nil
}

func (vr *VulkanRenderer) RenderBufferDestroy(buffer *metadata.RenderBuffer) {
	if buffer != nil {
		buffer.Buffer = nil
		// Free up the backend resources.
		vr.RenderBufferDestroyInternal(buffer)
		buffer.InternalData = nil
	}
}

func (vr *VulkanRenderer) RenderBufferBind(buffer *metadata.RenderBuffer, offset uint64) error {
	if buffer == nil {
		err := fmt.Errorf("renderer_renderbuffer_bind requires a valid pointer to a buffer")
		return err
	}
	internal_buffer := buffer.InternalData.(*VulkanBuffer)

	result := vk.BindBufferMemory(vr.context.Device.LogicalDevice, internal_buffer.Handle, internal_buffer.Memory, vk.DeviceSize(offset))
	if !VulkanResultIsSuccess(result) {
		err := fmt.Errorf("%s", VulkanResultString(result, true))
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

func (vr *VulkanRenderer) RenderBufferMapMemory(buffer *metadata.RenderBuffer, offset, size uint64) interface{} {
	if buffer == nil || buffer.InternalData == nil {
		core.LogError("vulkan_buffer_map_memory requires a valid pointer to a buffer.")
		return nil
	}
	internal_buffer := buffer.InternalData.(*VulkanBuffer)

	var dataPtr unsafe.Pointer
	result := vk.MapMemory(vr.context.Device.LogicalDevice, internal_buffer.Memory, vk.DeviceSize(offset), vk.DeviceSize(size), 0, &dataPtr)
	if !VulkanResultIsSuccess(result) {
		core.LogError("%s", VulkanResultString(result, true))
		return nil
	}
	return uint64(uintptr(dataPtr))
}

func (vr *VulkanRenderer) RenderBufferUnmapMemory(buffer *metadata.RenderBuffer, offset, size uint64) {
	if buffer == nil || buffer.InternalData == nil {
		core.LogError("vulkan_buffer_unmap_memory requires a valid pointer to a buffer.")
		return
	}
	internal_buffer := buffer.InternalData.(*VulkanBuffer)
	vk.UnmapMemory(vr.context.Device.LogicalDevice, internal_buffer.Memory)
}

func (vr *VulkanRenderer) RenderBufferFlush(buffer *metadata.RenderBuffer, offset, size uint64) bool {
	if buffer == nil || buffer.InternalData == nil {
		core.LogError("vulkan_buffer_flush requires a valid pointer to a buffer.")
		return false
	}
	// NOTE: If not host-coherent, flush the mapped memory range.
	internal_buffer := buffer.InternalData.(*VulkanBuffer)
	if !vr.vulkanBufferIsHostCoherent(internal_buffer) {
		mrange := vk.MappedMemoryRange{
			SType:  vk.StructureTypeMappedMemoryRange,
			Memory: internal_buffer.Memory,
			Offset: vk.DeviceSize(offset),
			Size:   vk.DeviceSize(size),
		}
		result := vk.FlushMappedMemoryRanges(vr.context.Device.LogicalDevice, 1, []vk.MappedMemoryRange{mrange})
		if !VulkanResultIsSuccess(result) {
			core.LogError("%s", VulkanResultString(result, true))
			return false
		}
	}

	return true
}

func (vr *VulkanRenderer) RenderBufferRead(buffer *metadata.RenderBuffer, offset, size uint64) (interface{}, error) {
	if buffer == nil || buffer.InternalData == nil {
		err := fmt.Errorf("vulkan_buffer_read requires a valid pointer to a buffer and out_memory, and the size must be nonzero.")
		return nil, err
	}

	out_memory := make([]interface{}, 1)

	internal_buffer := buffer.InternalData.(*VulkanBuffer)
	if vr.vulkanBufferIsDeviceLocal(internal_buffer) && !vr.vulkanBufferIsHostVisible(internal_buffer) {
		// NOTE: If a read buffer is needed (i.e.) the target buffer's memory is not host visible but is device-local,
		// create the read buffer, copy data to it, then read from that buffer.

		// Create a host-visible staging buffer to copy to. Mark it as the destination of the transfer.
		read, err := vr.RenderBufferCreate(metadata.RENDERBUFFER_TYPE_READ, size, false)
		if err != nil {
			core.LogError("vulkan_buffer_read() - Failed to create read buffer.")
			return nil, err
		}
		vr.RenderBufferBind(read, 0)
		read_internal := read.InternalData.(*VulkanBuffer)

		// Perform the copy from device local to the read buffer.
		vr.RenderBufferCopyRange(buffer, offset, read, 0, size)

		// Map/copy/unmap
		var mapped_data interface{}
		md := unsafe.Pointer(&mapped_data)
		result := vk.MapMemory(vr.context.Device.LogicalDevice, read_internal.Memory, 0, vk.DeviceSize(size), 0, &md)
		if !VulkanResultIsSuccess(result) {
			err := fmt.Errorf("%s", VulkanResultString(result, true))
			return nil, err
		}

		// kcopy_memory(*out_memory, mapped_data, size);
		vk.UnmapMemory(vr.context.Device.LogicalDevice, read_internal.Memory)

		// Clean up the read buffer.
		vr.RenderBufferUnbind(read)
		vr.RenderBufferDestroy(read)
	} else {
		// If no staging buffer is needed, map/copy/unmap.
		var data_ptr interface{}
		dp := unsafe.Pointer(&data_ptr)
		result := vk.MapMemory(vr.context.Device.LogicalDevice, internal_buffer.Memory, vk.DeviceSize(offset), vk.DeviceSize(size), 0, &dp)
		if !VulkanResultIsSuccess(result) {
			err := fmt.Errorf("%s", VulkanResultString(result, true))
			return nil, err
		}
		// kcopy_memory(*out_memory, data_ptr, size);
		vk.UnmapMemory(vr.context.Device.LogicalDevice, internal_buffer.Memory)
	}

	return out_memory, nil
}

func (vr *VulkanRenderer) RenderBufferResize(buffer *metadata.RenderBuffer, new_total_size uint64) bool {
	if buffer == nil || buffer.InternalData == nil {
		return false
	}

	internal_buffer := buffer.InternalData.(*VulkanBuffer)

	// Create new buffer.
	buffer_info := vk.BufferCreateInfo{
		SType:       vk.StructureTypeBufferCreateInfo,
		Size:        vk.DeviceSize(new_total_size),
		Usage:       internal_buffer.Usage,
		SharingMode: vk.SharingModeExclusive, // NOTE: Only used in one queue.
	}

	var new_buffer vk.Buffer
	result := vk.CreateBuffer(vr.context.Device.LogicalDevice, &buffer_info, vr.context.Allocator, &new_buffer)

	// Gather memory requirements.
	requirements := vk.MemoryRequirements{}
	vk.GetBufferMemoryRequirements(vr.context.Device.LogicalDevice, new_buffer, &requirements)

	// Allocate memory info
	allocate_info := vk.MemoryAllocateInfo{
		SType:           vk.StructureTypeMemoryAllocateInfo,
		AllocationSize:  requirements.Size,
		MemoryTypeIndex: uint32(internal_buffer.MemoryIndex),
	}

	// Allocate the memory.
	var new_memory vk.DeviceMemory
	result = vk.AllocateMemory(vr.context.Device.LogicalDevice, &allocate_info, vr.context.Allocator, &new_memory)
	if !VulkanResultIsSuccess(result) {
		core.LogError("Unable to resize vulkan buffer because the required memory allocation failed. Error: %s", VulkanResultString(result, true))
		return false
	}

	// Bind the new buffer's memory
	result = vk.BindBufferMemory(vr.context.Device.LogicalDevice, new_buffer, vk.DeviceMemory(new_memory), vk.DeviceSize(0))
	if !VulkanResultIsSuccess(result) {
		core.LogError("%s", VulkanResultString(result, true))
		return false
	}

	// Copy over the data.
	vr.vulkan_buffer_copy_range_internal(internal_buffer.Handle, 0, new_buffer, 0, buffer.TotalSize)

	// Make sure anything potentially using these is finished.
	// NOTE: We could use vkQueueWaitIdle here if we knew what queue this buffer would be used with...
	vk.DeviceWaitIdle(vr.context.Device.LogicalDevice)

	// Destroy the old
	if internal_buffer.Memory != nil {
		vk.FreeMemory(vr.context.Device.LogicalDevice, internal_buffer.Memory, vr.context.Allocator)
		internal_buffer.Memory = nil
	}
	if internal_buffer.Handle != nil {
		vk.DestroyBuffer(vr.context.Device.LogicalDevice, internal_buffer.Handle, vr.context.Allocator)
		internal_buffer.Handle = nil
	}

	// Report free of the old, allocate of the new.
	is_device_memory := (internal_buffer.MemoryPropertyFlags & uint32(vk.MemoryPropertyDeviceLocalBit)) == uint32(vk.MemoryPropertyDeviceLocalBit)

	internal_buffer.MemoryRequirements = requirements
	internal_buffer.MemoryRequirements.Size = 1 //MEMORY_TAG_GPU_LOCAL
	if !is_device_memory {
		internal_buffer.MemoryRequirements.Size = 2 //MEMORY_TAG_VULKAN
	}

	// Set new properties
	internal_buffer.Memory = new_memory
	internal_buffer.Handle = new_buffer

	return true
}

func (vr *VulkanRenderer) RenderBufferFree(buffer *metadata.RenderBuffer, size, offset uint64) bool {
	if buffer != nil {
		buffer.Buffer = make([]interface{}, size)
		buffer.InternalData = nil
		buffer.TotalSize = 0
	}
	return true
}

func (vr *VulkanRenderer) RenderBufferLoadRange(buffer *metadata.RenderBuffer, offset, size uint64, data interface{}) bool {
	if buffer == nil || buffer.InternalData == nil || size == 0 || data == nil {
		core.LogError("vulkan_buffer_load_range requires a valid pointer to a buffer, a nonzero size and a valid pointer to data")
		return false
	}

	internal_buffer := buffer.InternalData.(*VulkanBuffer)
	if vr.vulkanBufferIsDeviceLocal(internal_buffer) && !vr.vulkanBufferIsHostVisible(internal_buffer) {
		// NOTE: If a staging buffer is needed (i.e.) the target buffer's memory is not host visible but is device-local,
		// create a staging buffer to load the data into first. Then copy from it to the target buffer.

		// Create a host-visible staging buffer to upload to. Mark it as the source of the transfer.
		staging, err := vr.RenderBufferCreate(metadata.RENDERBUFFER_TYPE_STAGING, size, false)
		if err != nil {
			core.LogError("vulkan_buffer_load_range() - Failed to create staging buffer")
			return false
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
		var data_ptr unsafe.Pointer
		if result := vk.MapMemory(vr.context.Device.LogicalDevice, internal_buffer.Memory, vk.DeviceSize(offset), vk.DeviceSize(size), 0, &data_ptr); !VulkanResultIsSuccess(result) {
			core.LogError("%s", VulkanResultString(result, true))
			return false
		}

		data = data_ptr

		vk.UnmapMemory(vr.context.Device.LogicalDevice, internal_buffer.Memory)
	}

	return true
}

func (vr *VulkanRenderer) RenderBufferCopyRange(source *metadata.RenderBuffer, source_offset uint64, dest *metadata.RenderBuffer, dest_offset uint64, size uint64) bool {
	if source == nil || source.InternalData == nil || dest == nil || dest.InternalData == nil || size == 0 {
		core.LogError("vulkan_buffer_copy_range requires a valid pointers to source and destination buffers as well as a nonzero size")
		return false
	}

	return vr.vulkan_buffer_copy_range_internal(
		(source.InternalData.(*VulkanBuffer)).Handle,
		source_offset,
		(dest.InternalData.(*VulkanBuffer)).Handle,
		dest_offset,
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

func (vr *VulkanRenderer) vulkan_buffer_copy_range_internal(source vk.Buffer, source_offset uint64, dest vk.Buffer, dest_offset, size uint64) bool {
	// TODO: Assuming queue and pool usage here. Might want dedicated queue.
	queue := vr.context.Device.GraphicsQueue
	vk.QueueWaitIdle(queue)
	// Create a one-time-use command buffer.
	temp_command_buffer, err := AllocateAndBeginSingleUse(vr.context, vr.context.Device.GraphicsCommandPool)
	if err != nil {
		core.LogError(err.Error())
		return false
	}

	// Prepare the copy command and add it to the command buffer.
	copy_region := vk.BufferCopy{
		SrcOffset: vk.DeviceSize(source_offset),
		DstOffset: vk.DeviceSize(dest_offset),
		Size:      vk.DeviceSize(size),
	}
	vk.CmdCopyBuffer(temp_command_buffer.Handle, source, dest, 1, []vk.BufferCopy{copy_region})

	// Submit the buffer for execution and wait for it to complete.
	temp_command_buffer.EndSingleUse(vr.context, vr.context.Device.GraphicsCommandPool, queue)

	return true
}

func (vr *VulkanRenderer) RenderBufferDraw(buffer *metadata.RenderBuffer, offset uint64, element_count uint32, bind_only bool) bool {
	command_buffer := vr.context.GraphicsCommandBuffers[vr.context.ImageIndex]

	if buffer.RenderBufferType == metadata.RENDERBUFFER_TYPE_VERTEX {
		// Bind vertex buffer at offset.
		offsets := []vk.DeviceSize{vk.DeviceSize(offset)}
		vk.CmdBindVertexBuffers(command_buffer.Handle, 0, 1, []vk.Buffer{buffer.InternalData.(*VulkanBuffer).Handle}, offsets)
		if !bind_only {
			vk.CmdDraw(command_buffer.Handle, element_count, 1, 0, 0)
		}
		return true
	} else if buffer.RenderBufferType == metadata.RENDERBUFFER_TYPE_INDEX {
		// Bind index buffer at offset.
		vk.CmdBindIndexBuffer(command_buffer.Handle, (buffer.InternalData.(*VulkanBuffer)).Handle, vk.DeviceSize(offset), vk.IndexTypeUint32)
		if !bind_only {
			vk.CmdDrawIndexed(command_buffer.Handle, element_count, 1, 0, 0, 0)
		}
		return true
	} else {
		core.LogError("Cannot draw buffer of type: %d", buffer.RenderBufferType)
		return false
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
