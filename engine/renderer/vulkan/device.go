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
	SwapchainSupport   VulkanSwapchainSupportInfo
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

	DepthFormat vk.Format
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
	GraphicsFamilyIndex uint32
	PresentFamilyIndex  uint32
	ComputeFamilyIndex  uint32
	TransferFamilyIndex uint32
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
	deviceFeatures := vk.PhysicalDeviceFeatures{}
	deviceFeatures.SamplerAnisotropy = vk.True // Request anistrophy

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
		// availableExtensions = kallocate(sizeof(VkExtensionProperties)*availableExtensionCount, MEMORYTAGRENDERER)

		if res := vk.EnumerateDeviceExtensionProperties(context.Device.PhysicalDevice, "", &availableExtensionCount, availableExtensions); res != vk.Success {
			err := fmt.Errorf("error in EnumerateDeviceExtensionProperties")
			core.LogError(err.Error())
			return err
		}

		for i := 0; i < int(availableExtensionCount); i++ {
			if string(availableExtensions[i].ExtensionName[:]) == "VK_KHR_portability_subset" {
				core.LogInfo("Adding required extension 'VK_KHR_portability_subset'.")
				portabilityRequired = true
				break
			}
		}
	}
	// kfree(available_extensions, sizeof(VkExtensionProperties)*available_extension_count, MEMORY_TAG_RENDERER)
	availableExtensions = nil

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
		PpEnabledExtensionNames: extensionNames,
		// Deprecated and ignored, so pass nothing.
		EnabledLayerCount:   0,
		PpEnabledLayerNames: nil,
	}

	// Create the device.
	if res := vk.CreateDevice(
		context.Device.PhysicalDevice,
		&deviceCreateInfo,
		context.Allocator,
		&context.Device.LogicalDevice); res != vk.Success {
		return nil
	}

	core.LogInfo("Logical device created.")

	// Get queues.
	vk.GetDeviceQueue(
		context.Device.LogicalDevice,
		uint32(context.Device.GraphicsQueueIndex),
		0,
		&context.Device.GraphicsQueue)

	vk.GetDeviceQueue(
		context.Device.LogicalDevice,
		uint32(context.Device.PresentQueueIndex),
		0,
		&context.Device.PresentQueue)

	vk.GetDeviceQueue(
		context.Device.LogicalDevice,
		uint32(context.Device.TransferQueueIndex),
		0,
		&context.Device.TransferQueue)
	core.LogInfo("Queues obtained.")

	// Create command pool for graphics queue.
	poolCreateInfo := vk.CommandPoolCreateInfo{
		SType:            vk.StructureTypeCommandPoolCreateInfo,
		QueueFamilyIndex: uint32(context.Device.GraphicsQueueIndex),
		Flags:            vk.CommandPoolCreateFlags(vk.CommandPoolCreateResetCommandBufferBit),
	}
	if res := vk.CreateCommandPool(
		context.Device.LogicalDevice,
		&poolCreateInfo,
		context.Allocator,
		&context.Device.GraphicsCommandPool); res != vk.Success {
		return nil
	}
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
	if res := vk.GetPhysicalDeviceSurfaceCapabilities(physicalDevice, surface, &supportInfo.Capabilities); res != vk.Success {
		return nil
	}
	// Surface formats
	if res := vk.GetPhysicalDeviceSurfaceFormats(physicalDevice, surface, &supportInfo.FormatCount, nil); res != vk.Success {
		return nil
	}
	if supportInfo.FormatCount != 0 {
		if supportInfo.Formats != nil {
			supportInfo.Formats = make([]vk.SurfaceFormat, supportInfo.FormatCount)
			core.LogDebug("allocated memory for VkSurfaceFormatKHR")
		}
		if res := vk.GetPhysicalDeviceSurfaceFormats(physicalDevice, surface, &supportInfo.FormatCount, supportInfo.Formats); res != vk.Success {
			return nil
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
	flags := vk.FormatFeatureDepthStencilAttachmentBit
	for i := 0; i < candidateCount; i++ {
		var properties vk.FormatProperties = vk.FormatProperties{}
		vk.GetPhysicalDeviceFormatProperties(device.PhysicalDevice, candidates[i], &properties)
		if (vk.FormatFeatureFlagBits(properties.LinearTilingFeatures) & flags) == flags {
			device.DepthFormat = candidates[i]
			return true
		} else if (vk.FormatFeatureFlagBits(properties.OptimalTilingFeatures) & flags) == flags {
			device.DepthFormat = candidates[i]
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
		properties := vk.PhysicalDeviceProperties{}
		vk.GetPhysicalDeviceProperties(physicalDevices[i], &properties)

		features := vk.PhysicalDeviceFeatures{}
		vk.GetPhysicalDeviceFeatures(physicalDevices[i], &features)

		memory := vk.PhysicalDeviceMemoryProperties{}
		vk.GetPhysicalDeviceMemoryProperties(physicalDevices[i], &memory)

		// TODO: These requirements should probably be driven by engine
		// configuration.
		requirements := VulkanPhysicalDeviceRequirements{
			Graphics:             true,
			Present:              true,
			Transfer:             true,
			SamplerAnisotropy:    true,
			DiscreteGPU:          true,
			DeviceExtensionNames: []string{vk.KhrSwapchainExtensionName},
			// NOTE: Enable this if compute will be required.
			// Compute: true,
		}

		if runtime.GOOS == "darwin" {
			requirements.DiscreteGPU = false
		}

		queueInfo := VulkanPhysicalDeviceQueueFamilyInfo{}
		result := PhysicalDeviceMeetsRequirements(
			physicalDevices[i],
			context.Surface,
			&properties,
			&features,
			&requirements,
			&queueInfo,
			&context.Device.SwapchainSupport)

		if result {
			core.LogInfo("Selected device: '%s'.", properties.DeviceName)
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
				if vk.MemoryHeapFlagBits(memory.MemoryHeaps[j].Flags)&vk.MemoryHeapDeviceLocalBit > 0 {
					core.LogInfo("Local GPU memory: %d GiB", memorySizeGib)
				} else {
					core.LogInfo("Shared System memory: %d GiB", memorySizeGib)
				}
			}

			context.Device.PhysicalDevice = physicalDevices[i]
			context.Device.GraphicsQueueIndex = int32(queueInfo.GraphicsFamilyIndex)
			context.Device.PresentQueueIndex = int32(queueInfo.PresentFamilyIndex)
			context.Device.TransferQueueIndex = int32(queueInfo.TransferFamilyIndex)
			// NOTE: set compute index here if needed.

			// Keep a copy of properties, features and memory info for later use.
			context.Device.Properties = properties
			context.Device.Features = features
			context.Device.Memory = memory
			break
		}
	}

	// Ensure a device was selected
	if context.Device.PhysicalDevice != nil {
		core.LogError("No physical devices were found which meet the requirements.")
		return false
	}

	core.LogInfo("Physical device selected.")
	return true
}

func PhysicalDeviceMeetsRequirements(device vk.PhysicalDevice, surface vk.Surface, properties *vk.PhysicalDeviceProperties, features *vk.PhysicalDeviceFeatures, requirements *VulkanPhysicalDeviceRequirements, outQueueInfo *VulkanPhysicalDeviceQueueFamilyInfo, outSwapchainSupport *VulkanSwapchainSupportInfo) bool {
	// Evaluate device properties to determine if it meets the needs of our applcation.
	outQueueInfo.GraphicsFamilyIndex = 0
	outQueueInfo.PresentFamilyIndex = 0
	outQueueInfo.ComputeFamilyIndex = 0
	outQueueInfo.TransferFamilyIndex = 0

	// Discrete GPU?
	if requirements.DiscreteGPU {
		if properties.DeviceType != vk.PhysicalDeviceTypeDiscreteGpu {
			core.LogInfo("Device is not a discrete GPU, and one is required. Skipping.")
			return false
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
		if vk.QueueFlagBits(queueFamilies[i].QueueFlags)&vk.QueueGraphicsBit > 0 {
			outQueueInfo.GraphicsFamilyIndex = uint32(i)
			currentTransferScore++
		}

		// Compute queue?
		if queueFamilies[i].QueueFlags&vk.QueueFlags(vk.QueueComputeBit) > 0 {
			outQueueInfo.ComputeFamilyIndex = uint32(i)
			currentTransferScore++
		}

		// Transfer queue?
		if vk.QueueFlagBits(queueFamilies[i].QueueFlags)&vk.QueueTransferBit > 0 {
			// Take the index if it is the current lowest. This increases the
			// liklihood that it is a dedicated transfer queue.
			if currentTransferScore <= minTransferScore {
				minTransferScore = currentTransferScore
				outQueueInfo.TransferFamilyIndex = uint32(i)
			}
		}

		// Present queue?
		var supportsPresent vk.Bool32 = vk.False
		if res := vk.GetPhysicalDeviceSurfaceSupport(device, uint32(i), surface, &supportsPresent); res != vk.Success {
			return false
		}
		if supportsPresent == vk.True {
			outQueueInfo.PresentFamilyIndex = uint32(i)
		}
	}

	// Print out some info about the device
	core.LogInfo("       %t |       %t |       %t |        %t | %s",
		outQueueInfo.GraphicsFamilyIndex != 0,
		outQueueInfo.PresentFamilyIndex != 0,
		outQueueInfo.ComputeFamilyIndex != 0,
		outQueueInfo.TransferFamilyIndex != 0,
		properties.DeviceName)

	if (!requirements.Graphics || (requirements.Graphics && outQueueInfo.GraphicsFamilyIndex != 0)) &&
		(!requirements.Present || (requirements.Present && outQueueInfo.PresentFamilyIndex != 0)) &&
		(!requirements.Compute || (requirements.Compute && outQueueInfo.ComputeFamilyIndex != 0)) &&
		(!requirements.Transfer || (requirements.Transfer && outQueueInfo.TransferFamilyIndex != 0)) {
		core.LogInfo("Device meets queue requirements.")
		core.LogDebug("Graphics Family Index: %d", outQueueInfo.GraphicsFamilyIndex)
		core.LogDebug("Present Family Index:  %d", outQueueInfo.PresentFamilyIndex)
		core.LogDebug("Transfer Family Index: %d", outQueueInfo.TransferFamilyIndex)
		core.LogDebug("Compute Family Index:  %d", outQueueInfo.ComputeFamilyIndex)

		// Query swapchain support.
		DeviceQuerySwapchainSupport(device, surface, outSwapchainSupport)

		if outSwapchainSupport.FormatCount < 1 || outSwapchainSupport.PresentModeCount < 1 {
			if len(outSwapchainSupport.Formats) > 0 {
				// kfree(out_swapchain_support.Formats, sizeof(VkSurfaceFormatKHR) * out_swapchain_support.format_count, MEMORY_TAG_RENDERER);
			}
			if len(outSwapchainSupport.PresentModes) > 0 {
				// kfree(out_swapchain_support.present_modes, sizeof(VkPresentModeKHR) * out_swapchain_support.PresentModeCount, MEMORY_TAG_RENDERER);
			}
			core.LogInfo("Required swapchain support not present, skipping device.")
			return false
		}

		// Device extensions.
		if requirements.DeviceExtensionNames != nil {
			var availableExtensionCount uint32 = 0
			var availableExtensions []vk.ExtensionProperties

			if res := vk.EnumerateDeviceExtensionProperties(device, "", &availableExtensionCount, nil); res != vk.Success {
				return false
			}

			if availableExtensionCount != 0 {
				availableExtensions = make([]vk.ExtensionProperties, availableExtensionCount)
				if res := vk.EnumerateDeviceExtensionProperties(device, "", &availableExtensionCount, availableExtensions); res != vk.Success {
					return false
				}
				requiredExtensionCount := len(requirements.DeviceExtensionNames)
				for i := 0; i < requiredExtensionCount; i++ {
					found := false
					for j := 0; j < int(availableExtensionCount); j++ {
						if requirements.DeviceExtensionNames[i] == string(availableExtensions[j].ExtensionName[:]) {
							found = true
							break
						}
					}
					if !found {
						core.LogInfo("Required extension not found: '%s', skipping device.", requirements.DeviceExtensionNames[i])
						availableExtensions = nil
						return false
					}
				}
			}
			availableExtensions = nil
		}
		// Sampler anisotropy
		if requirements.SamplerAnisotropy && features.SamplerAnisotropy == vk.False {
			core.LogInfo("Device does not support samplerAnisotropy, skipping.")
			return false
		}
		// Device meets all requirements.
		return true
	}
	return false
}
