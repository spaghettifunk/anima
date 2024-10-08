package vulkan

import (
	"fmt"
	"runtime"

	vk "github.com/goki/vulkan"
	"github.com/spaghettifunk/alaska-engine/engine/core"
	"github.com/spaghettifunk/alaska-engine/engine/platform"
)

type VulkanDevice struct {
	PhysicalDevice     vk.PhysicalDevice
	LogicalDevice      vk.Device
	SwapchainSupport   *VulkanSwapchainSupportInfo
	GraphicsQueueIndex int32
	PresentQueueIndex  int32
	TransferQueueIndex int32

	GraphicsQueue vk.Queue
	PresentQueue  vk.Queue
	TransferQueue vk.Queue

	GraphicsCommandPool vk.CommandPool

	Properties vk.PhysicalDeviceProperties
	Features   vk.PhysicalDeviceFeatures
	Memory     vk.PhysicalDeviceMemoryProperties

	DepthFormat       vk.Format
	DepthChannelCount uint8
}

func CreateVulkanSurface(platform *platform.Platform, context *VulkanContext) bool {
	_, err := platform.Window.CreateWindowSurface(context.Instance, nil)
	if err != nil {
		core.LogFatal("Vulkan surface creation failed.")
		return false
	}
	return true
}

type VulkanPhysicalDeviceRequirements struct {
	Graphics             bool
	Present              bool
	Compute              bool
	Transfer             bool
	DeviceExtensionNames []string
	SamplerAnisotropy    bool
	DiscreteGPU          bool
}

type VulkanPhysicalDeviceQueueFamilyInfo struct {
	GraphicsFamilyIndex int32
	PresentFamilyIndex  int32
	ComputeFamilyIndex  int32
	TransferFamilyIndex int32
}

func DeviceCreate(context *VulkanContext) error {
	if !SelectPhysicalDevice(context) {
		return nil
	}

	core.LogInfo("Creating logical device...")

	// NOTE: Do not create additional queues for shared indices.
	presentSharesGraphicsQueue := context.Device.GraphicsQueueIndex == context.Device.PresentQueueIndex
	transferSharesGraphicsQueue := context.Device.GraphicsQueueIndex == context.Device.TransferQueueIndex
	indexCount := 1

	if !presentSharesGraphicsQueue {
		indexCount++
	}
	if !transferSharesGraphicsQueue {
		indexCount++
	}
	indices := make([]uint32, indexCount)
	index := 0
	indices[index] = uint32(context.Device.GraphicsQueueIndex)
	index += 1

	if !presentSharesGraphicsQueue {
		indices[index] = uint32(context.Device.PresentQueueIndex)
		index += 1
	}
	if !transferSharesGraphicsQueue {
		indices[index] = uint32(context.Device.TransferQueueIndex)
		index += 1
	}

	queueCreateInfos := make([]vk.DeviceQueueCreateInfo, indexCount)
	for i := 0; i < int(indexCount); i++ {
		queueCreateInfos[i].SType = vk.StructureTypeDeviceQueueCreateInfo
		queueCreateInfos[i].QueueFamilyIndex = indices[i]
		queueCreateInfos[i].QueueCount = 1

		// TODO: Enable this for a future enhancement.
		// if (indices[i] == context->device.graphics_queue_index) {
		//     queue_create_infos[i].queueCount = 2;
		// }
		queueCreateInfos[i].Flags = 0
		queueCreateInfos[i].PNext = nil
		var queuePriority float32 = 1.0
		queueCreateInfos[i].PQueuePriorities = []float32{queuePriority}
	}

	// Request device features.
	// TODO: should be config driven
	deviceFeatures := vk.PhysicalDeviceFeatures{
		SamplerAnisotropy: vk.True, // Request anistrophy
	}

	portabilityRequired := false
	var availableExtensionCount uint32 = 0
	var availableExtensions []vk.ExtensionProperties

	if res := vk.EnumerateDeviceExtensionProperties(context.Device.PhysicalDevice, "", &availableExtensionCount, nil); res != vk.Success {
		err := fmt.Errorf("error in EnumerateDeviceExtensionProperties")
		core.LogError(err.Error())
		return err
	}

	if availableExtensionCount != 0 {
		availableExtensions = make([]vk.ExtensionProperties, availableExtensionCount)
		if res := vk.EnumerateDeviceExtensionProperties(context.Device.PhysicalDevice, "", &availableExtensionCount, availableExtensions); res != vk.Success {
			err := fmt.Errorf("error in EnumerateDeviceExtensionProperties")
			core.LogError(err.Error())
			return err
		}

		for i := 0; i < int(availableExtensionCount); i++ {
			availableExtensions[i].Deref()
			core.LogInfo("Available Extension: `%s`", string(availableExtensions[i].ExtensionName[:]))
			end := FindFirstZeroInByteArray(availableExtensions[i].ExtensionName[:])
			if vk.ToString(availableExtensions[i].ExtensionName[:end+1]) == "VK_KHR_portability_subset" {
				core.LogInfo("Adding required extension 'VK_KHR_portability_subset'.")
				portabilityRequired = true
				break
			}
		}
	}

	// availableExtensions = nil

	extensionCount := 2
	if !portabilityRequired {
		extensionCount = 1
	}

	extensionNames := []string{}
	if portabilityRequired {
		extensionNames = append(extensionNames, vk.KhrSwapchainExtensionName, "VK_KHR_portability_subset")
	} else {
		extensionNames = append(extensionNames, vk.KhrSwapchainExtensionName)
	}

	deviceCreateInfo := vk.DeviceCreateInfo{
		SType:                   vk.StructureTypeDeviceCreateInfo,
		QueueCreateInfoCount:    uint32(indexCount),
		PQueueCreateInfos:       queueCreateInfos,
		PEnabledFeatures:        []vk.PhysicalDeviceFeatures{deviceFeatures},
		EnabledExtensionCount:   uint32(extensionCount),
		PpEnabledExtensionNames: VulkanSafeStrings(extensionNames),
	}

	// Create the device.
	var device vk.Device
	if res := vk.CreateDevice(context.Device.PhysicalDevice, &deviceCreateInfo, context.Allocator, &device); res != vk.Success {
		return nil
	}
	context.Device.LogicalDevice = device

	core.LogInfo("Logical device created.")

	// Get queues.
	var gQueue vk.Queue
	vk.GetDeviceQueue(context.Device.LogicalDevice, uint32(context.Device.GraphicsQueueIndex), 0, &gQueue)
	context.Device.GraphicsQueue = gQueue

	var pQueue vk.Queue
	vk.GetDeviceQueue(context.Device.LogicalDevice, uint32(context.Device.PresentQueueIndex), 0, &pQueue)
	context.Device.PresentQueue = pQueue

	var tQueue vk.Queue
	vk.GetDeviceQueue(context.Device.LogicalDevice, uint32(context.Device.TransferQueueIndex), 0, &tQueue)
	context.Device.TransferQueue = tQueue

	core.LogInfo("Queues obtained.")

	// Create command pool for graphics queue.
	poolCreateInfo := vk.CommandPoolCreateInfo{
		SType:            vk.StructureTypeCommandPoolCreateInfo,
		QueueFamilyIndex: uint32(context.Device.GraphicsQueueIndex),
		Flags:            vk.CommandPoolCreateFlags(vk.CommandPoolCreateResetCommandBufferBit),
	}

	var gcPool vk.CommandPool
	if res := vk.CreateCommandPool(context.Device.LogicalDevice, &poolCreateInfo, context.Allocator, &gcPool); res != vk.Success {
		return nil
	}
	context.Device.GraphicsCommandPool = gcPool
	core.LogInfo("Graphics command pool created.")

	return nil
}

func DeviceDestroy(context *VulkanContext) {
	// Unset queues
	context.Device.GraphicsQueue = nil
	context.Device.PresentQueue = nil
	context.Device.TransferQueue = nil

	core.LogInfo("Destroying command pools...")
	vk.DestroyCommandPool(
		context.Device.LogicalDevice,
		context.Device.GraphicsCommandPool,
		context.Allocator)

	// Destroy logical device
	core.LogInfo("Destroying logical device...")
	if context.Device.LogicalDevice != nil {
		vk.DestroyDevice(context.Device.LogicalDevice, context.Allocator)
		context.Device.LogicalDevice = nil
	}

	// Physical devices are not destroyed.
	core.LogInfo("Releasing physical device resources...")
	context.Device.PhysicalDevice = nil

	if context.Device.SwapchainSupport.Formats != nil {
		context.Device.SwapchainSupport.Formats = nil
		context.Device.SwapchainSupport.FormatCount = 0
	}

	if context.Device.SwapchainSupport.PresentModes != nil {
		context.Device.SwapchainSupport.PresentModes = nil
		context.Device.SwapchainSupport.PresentModeCount = 0
	}

	context.Device.SwapchainSupport.Capabilities = vk.SurfaceCapabilities{}

	context.Device.GraphicsQueueIndex = -1
	context.Device.PresentQueueIndex = -1
	context.Device.TransferQueueIndex = -1
}

func DeviceQuerySwapchainSupport(physicalDevice vk.PhysicalDevice, surface vk.Surface, supportInfo *VulkanSwapchainSupportInfo) error {
	// Surface capabilities
	var capabilities vk.SurfaceCapabilities
	if res := vk.GetPhysicalDeviceSurfaceCapabilities(physicalDevice, surface, &capabilities); res != vk.Success {
		return nil
	}
	capabilities.Deref()
	supportInfo.Capabilities = capabilities

	// Surface formats
	if res := vk.GetPhysicalDeviceSurfaceFormats(physicalDevice, surface, &supportInfo.FormatCount, nil); res != vk.Success {
		return nil
	}
	if supportInfo.FormatCount != 0 {
		if supportInfo.Formats != nil {
			supportInfo.Formats = make([]vk.SurfaceFormat, supportInfo.FormatCount)
			core.LogDebug("allocated memory for VkSurfaceFormatKHR")
		}
		supportInfo.Formats = make([]vk.SurfaceFormat, supportInfo.FormatCount)
		if res := vk.GetPhysicalDeviceSurfaceFormats(physicalDevice, surface, &supportInfo.FormatCount, supportInfo.Formats); res != vk.Success {
			err := fmt.Errorf("failed to get physical device surface formats")
			core.LogError(err.Error())
			return err
		}
		for i := range supportInfo.Formats {
			supportInfo.Formats[i].Deref()
		}
	}
	// Present modes
	if res := vk.GetPhysicalDeviceSurfacePresentModes(physicalDevice, surface, &supportInfo.PresentModeCount, nil); res != vk.Success {
		err := fmt.Errorf("failed to get physical device surface present modes")
		core.LogError(err.Error())
		return err
	}
	if supportInfo.PresentModeCount != 0 {
		if supportInfo.PresentModes != nil {
			supportInfo.PresentModes = make([]vk.PresentMode, supportInfo.PresentModeCount)
			core.LogDebug("allocated memory for VkPresentModeKHR")
		}
		supportInfo.PresentModes = make([]vk.PresentMode, supportInfo.PresentModeCount)
		if res := vk.GetPhysicalDeviceSurfacePresentModes(physicalDevice, surface, &supportInfo.PresentModeCount, supportInfo.PresentModes); res != vk.Success {
			err := fmt.Errorf("failed to get physical device surface present modes")
			core.LogError(err.Error())
			return err
		}
	}
	return nil
}

func DeviceDetectDepthFormat(device *VulkanDevice) bool {
	// Format candidates
	candidateCount := 3

	candidates := []vk.Format{
		vk.FormatD32Sfloat,
		vk.FormatD32SfloatS8Uint,
		vk.FormatD24UnormS8Uint,
	}

	sizes := []uint8{4, 4, 3}

	flags := vk.FormatFeatureDepthStencilAttachmentBit

	for i := 0; i < candidateCount; i++ {
		var properties vk.FormatProperties
		vk.GetPhysicalDeviceFormatProperties(device.PhysicalDevice, candidates[i], &properties)
		properties.Deref()
		if (uint32(properties.LinearTilingFeatures) & uint32(flags)) == uint32(flags) {
			device.DepthFormat = candidates[i]
			device.DepthChannelCount = sizes[i]
			return true
		} else if (uint32(properties.OptimalTilingFeatures) & uint32(flags)) == uint32(flags) {
			device.DepthFormat = candidates[i]
			device.DepthChannelCount = sizes[i]
			return true
		}
	}
	return false
}

func SelectPhysicalDevice(context *VulkanContext) bool {
	var physicalDeviceCount uint32 = 0
	if res := vk.EnumeratePhysicalDevices(context.Instance, &physicalDeviceCount, nil); res != vk.Success {
		return false
	}

	if physicalDeviceCount == 0 {
		core.LogFatal("No devices which support Vulkan were found.")
		return false
	}

	physicalDevices := make([]vk.PhysicalDevice, physicalDeviceCount)
	if res := vk.EnumeratePhysicalDevices(context.Instance, &physicalDeviceCount, physicalDevices); res != vk.Success {
		return false
	}

	for i := 0; i < int(physicalDeviceCount); i++ {
		var properties vk.PhysicalDeviceProperties
		vk.GetPhysicalDeviceProperties(physicalDevices[i], &properties)
		properties.Deref()

		var features vk.PhysicalDeviceFeatures
		vk.GetPhysicalDeviceFeatures(physicalDevices[i], &features)
		features.Deref()

		var memory vk.PhysicalDeviceMemoryProperties
		vk.GetPhysicalDeviceMemoryProperties(physicalDevices[i], &memory)
		memory.Deref()

		// TODO: These requirements should probably be driven by engine
		// configuration.
		requirements := VulkanPhysicalDeviceRequirements{
			Graphics:             true,
			Present:              true,
			Transfer:             true,
			SamplerAnisotropy:    true,
			DiscreteGPU:          true,
			DeviceExtensionNames: []string{VulkanSafeString(vk.KhrSwapchainExtensionName)},
		}

		if runtime.GOOS == "darwin" {
			requirements.DiscreteGPU = false
		}

		queueInfo, swapchainSupport, err := PhysicalDeviceMeetsRequirements(physicalDevices[i], context.Surface, &properties, &features, &requirements)
		if err != nil {
			core.LogError(err.Error())
			return false
		}

		context.Device = &VulkanDevice{
			SwapchainSupport: swapchainSupport,
		}

		core.LogInfo("Selected device: '%s'.", vk.ToString(properties.DeviceName[:]))

		// GPU type, etc.
		switch properties.DeviceType {
		default:
			fallthrough
		case vk.PhysicalDeviceTypeOther:
			core.LogInfo("GPU type is Unknown.")
		case vk.PhysicalDeviceTypeIntegratedGpu:
			core.LogInfo("GPU type is Integrated.")
		case vk.PhysicalDeviceTypeDiscreteGpu:
			core.LogInfo("GPU type is Descrete.")
		case vk.PhysicalDeviceTypeVirtualGpu:
			core.LogInfo("GPU type is Virtual.")
		case vk.PhysicalDeviceTypeCpu:
			core.LogInfo("GPU type is CPU.")
		}

		core.LogInfo(
			"GPU Driver version: %d.%d.%d",
			vk.Version.Major(vk.Version(properties.DriverVersion)),
			vk.Version.Minor(vk.Version(properties.DriverVersion)),
			vk.Version.Patch(vk.Version(properties.DriverVersion)),
		)

		// Vulkan API version.
		core.LogInfo(
			"Vulkan API version: %d.%d.%d",
			vk.Version.Major(vk.Version(properties.ApiVersion)),
			vk.Version.Minor(vk.Version(properties.ApiVersion)),
			vk.Version.Patch(vk.Version(properties.ApiVersion)),
		)

		// Memory information
		for j := 0; j < int(memory.MemoryHeapCount); j++ {
			memorySizeGib := ((memory.MemoryHeaps[j].Size) / 1024.0 / 1024.0 / 1024.0)
			// TODO: check the condition
			if uint32(memory.MemoryHeaps[j].Flags)&uint32(vk.MemoryHeapDeviceLocalBit) != 0 {
				core.LogInfo("Local GPU memory: %d GiB", memorySizeGib)
			} else {
				core.LogInfo("Shared System memory: %d GiB", memorySizeGib)
			}
		}

		context.Device.PhysicalDevice = physicalDevices[i]
		context.Device.GraphicsQueueIndex = queueInfo.GraphicsFamilyIndex
		context.Device.PresentQueueIndex = queueInfo.PresentFamilyIndex
		context.Device.TransferQueueIndex = queueInfo.TransferFamilyIndex
		// NOTE: set compute index here if needed.

		// Keep a copy of properties, features and memory info for later use.
		context.Device.Properties = properties
		context.Device.Features = features
		context.Device.Memory = memory
	}

	// Ensure a device was selected
	if context.Device.PhysicalDevice == nil {
		core.LogError("No physical devices were found which meet the requirements.")
		return false
	}
	core.LogInfo("Physical device selected.")
	return true
}

func PhysicalDeviceMeetsRequirements(device vk.PhysicalDevice, surface vk.Surface, properties *vk.PhysicalDeviceProperties,
	features *vk.PhysicalDeviceFeatures, requirements *VulkanPhysicalDeviceRequirements) (*VulkanPhysicalDeviceQueueFamilyInfo, *VulkanSwapchainSupportInfo, error) {
	// Evaluate device properties to determine if it meets the needs of our applcation.
	outQueueInfo := &VulkanPhysicalDeviceQueueFamilyInfo{
		GraphicsFamilyIndex: -1,
		PresentFamilyIndex:  -1,
		ComputeFamilyIndex:  -1,
		TransferFamilyIndex: -1,
	}

	// Discrete GPU?
	if requirements.DiscreteGPU {
		if properties.DeviceType != vk.PhysicalDeviceTypeDiscreteGpu {
			err := fmt.Errorf("device is not a discrete GPU, and one is required. Skipping")
			core.LogInfo(err.Error())
			return nil, nil, err
		}
	}

	var queueFamilyCount uint32 = 0
	vk.GetPhysicalDeviceQueueFamilyProperties(device, &queueFamilyCount, nil)
	queueFamilies := make([]vk.QueueFamilyProperties, queueFamilyCount)
	vk.GetPhysicalDeviceQueueFamilyProperties(device, &queueFamilyCount, queueFamilies)

	// Look at each queue and see what queues it supports
	core.LogInfo("Graphics | Present | Compute | Transfer | Name")
	minTransferScore := 255
	for i := 0; i < int(queueFamilyCount); i++ {
		currentTransferScore := 0
		// Graphics queue?
		if (uint32(queueFamilies[i].QueueFlags) & uint32(vk.QueueGraphicsBit)) != 0 {
			outQueueInfo.GraphicsFamilyIndex = int32(i)
			currentTransferScore++
		}

		// Compute queue?
		if (uint32(queueFamilies[i].QueueFlags) & uint32(vk.QueueComputeBit)) != 0 {
			outQueueInfo.ComputeFamilyIndex = int32(i)
			currentTransferScore++
		}

		// Transfer queue?
		if (uint32(queueFamilies[i].QueueFlags) & uint32(vk.QueueTransferBit)) != 0 {
			// Take the index if it is the current lowest. This increases the
			// liklihood that it is a dedicated transfer queue.
			if currentTransferScore <= minTransferScore {
				minTransferScore = currentTransferScore
				outQueueInfo.TransferFamilyIndex = int32(i)
			}
		}

		// Present queue?
		var supportsPresent vk.Bool32 = vk.False
		if res := vk.GetPhysicalDeviceSurfaceSupport(device, uint32(i), surface, &supportsPresent); res != vk.Success {
			err := fmt.Errorf("failed to get physical device surface support")
			core.LogDebug(err.Error())
			return nil, nil, err
		}
		if supportsPresent == vk.True {
			outQueueInfo.PresentFamilyIndex = int32(i)
		}
	}

	// Print out some info about the device
	core.LogInfo("       %t |       %t |       %t |        %t | %s",
		outQueueInfo.GraphicsFamilyIndex != 0,
		outQueueInfo.PresentFamilyIndex != 0,
		outQueueInfo.ComputeFamilyIndex != 0,
		outQueueInfo.TransferFamilyIndex != 0,
		vk.ToString(properties.DeviceName[:]))

	if (!requirements.Graphics || (requirements.Graphics && outQueueInfo.GraphicsFamilyIndex != 0)) &&
		(!requirements.Present || (requirements.Present && outQueueInfo.PresentFamilyIndex != 0)) &&
		(!requirements.Compute || (requirements.Compute && outQueueInfo.ComputeFamilyIndex != 0)) &&
		(!requirements.Transfer || (requirements.Transfer && outQueueInfo.TransferFamilyIndex != 0)) {
		core.LogInfo("Device meets queue requirements.")
		core.LogDebug("Graphics Family Index: %d", outQueueInfo.GraphicsFamilyIndex)
		core.LogDebug("Present Family Index:  %d", outQueueInfo.PresentFamilyIndex)
		core.LogDebug("Transfer Family Index: %d", outQueueInfo.TransferFamilyIndex)
		core.LogDebug("Compute Family Index:  %d", outQueueInfo.ComputeFamilyIndex)

		outSwapchainSupport := &VulkanSwapchainSupportInfo{}

		// Query swapchain support.
		DeviceQuerySwapchainSupport(device, surface, outSwapchainSupport)

		if outSwapchainSupport.FormatCount < 1 || outSwapchainSupport.PresentModeCount < 1 {
			if len(outSwapchainSupport.Formats) > 0 {
				// kfree(out_swapchain_support.Formats, sizeof(VkSurfaceFormatKHR) * out_swapchain_support.format_count, MEMORY_TAG_RENDERER);
			}
			if len(outSwapchainSupport.PresentModes) > 0 {
				// kfree(out_swapchain_support.present_modes, sizeof(VkPresentModeKHR) * out_swapchain_support.PresentModeCount, MEMORY_TAG_RENDERER);
			}
			err := fmt.Errorf("required swapchain support not present, skipping device")
			core.LogInfo(err.Error())
			return nil, nil, err
		}

		// Device extensions.
		if requirements.DeviceExtensionNames != nil {
			var availableExtensionCount uint32 = 0
			var availableExtensions []vk.ExtensionProperties

			if res := vk.EnumerateDeviceExtensionProperties(device, "", &availableExtensionCount, nil); res != vk.Success {
				err := fmt.Errorf("failed to enumerate device extension properties")
				core.LogInfo(err.Error())
				return nil, nil, err
			}

			if availableExtensionCount != 0 {
				availableExtensions = make([]vk.ExtensionProperties, availableExtensionCount)
				if res := vk.EnumerateDeviceExtensionProperties(device, "", &availableExtensionCount, availableExtensions); res != vk.Success {
					err := fmt.Errorf("failed to enumerate device extension properties")
					core.LogInfo(err.Error())
					return nil, nil, err
				}
				requiredExtensionCount := len(requirements.DeviceExtensionNames)
				for i := 0; i < requiredExtensionCount; i++ {
					found := false
					for j := 0; j < int(availableExtensionCount); j++ {
						availableExtensions[j].Deref()
						core.LogInfo("Available extension: `%s`", string(availableExtensions[j].ExtensionName[:]))
						end := FindFirstZeroInByteArray(availableExtensions[j].ExtensionName[:])
						if requirements.DeviceExtensionNames[i] == string(availableExtensions[j].ExtensionName[:end+1]) {
							found = true
							break
						}
					}
					if !found {
						err := fmt.Errorf("required extension not found: '%s', skipping device", requirements.DeviceExtensionNames[i])
						core.LogInfo("Required extension not found: '%s', skipping device.", requirements.DeviceExtensionNames[i])
						availableExtensions = nil
						return nil, nil, err
					}
				}
			}
			availableExtensions = nil
		}
		// Sampler anisotropy
		if requirements.SamplerAnisotropy && features.SamplerAnisotropy == vk.False {
			core.LogInfo("device does not support samplerAnisotropy, skipping")
		}

		// change values of queue otherwise conversion to uint32 will fail
		if outQueueInfo.GraphicsFamilyIndex == -1 {
			outQueueInfo.GraphicsFamilyIndex = 0
		}
		if outQueueInfo.PresentFamilyIndex == -1 {
			outQueueInfo.PresentFamilyIndex = 0
		}
		if outQueueInfo.TransferFamilyIndex == -1 {
			outQueueInfo.TransferFamilyIndex = 0
		}
		if outQueueInfo.ComputeFamilyIndex == -1 {
			outQueueInfo.ComputeFamilyIndex = 0
		}

		// Device meets all requirements.
		return outQueueInfo, outSwapchainSupport, nil
	}
	return nil, nil, fmt.Errorf("failed to get a device that meets the requirements")
}
