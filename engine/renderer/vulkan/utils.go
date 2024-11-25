package vulkan

import (
	vk "github.com/goki/vulkan"
)

func VulkanResultString(result vk.Result, getExtended bool) string {
	// From: https://www.khronos.org/registry/vulkan/specs/1.3-extensions/man/html/VkResult.html
	// Success Codes
	switch result {
	default:
		fallthrough
	case vk.Success:
		return ConditionalOperator(!getExtended, "VK_SUCCESS", "VK_SUCCESS Command successfully completed")
	case vk.NotReady:
		return ConditionalOperator(!getExtended, "VK_NOT_READY", "VK_NOT_READY A fence or query has not yet completed")
	case vk.Timeout:
		return ConditionalOperator(!getExtended, "VK_TIMEOUT", "VK_TIMEOUT A wait operation has not completed in the specified time")
	case vk.EventSet:
		return ConditionalOperator(!getExtended, "VK_EVENT_SET", "VK_EVENT_SET An event is signaled")
	case vk.EventReset:
		return ConditionalOperator(!getExtended, "VK_EVENT_RESET", "VK_EVENT_RESET An event is unsignaled")
	case vk.Incomplete:
		return ConditionalOperator(!getExtended, "VK_INCOMPLETE", "VK_INCOMPLETE A return array was too small for the result")
	case vk.Suboptimal:
		return ConditionalOperator(!getExtended, "VK_SUBOPTIMAL_KHR", "VK_SUBOPTIMAL_KHR A swapchain no longer matches the surface properties exactly, but can still be used to present to the surface successfully.")
	case vk.ThreadIdle:
		return ConditionalOperator(!getExtended, "VK_THREAD_IDLE_KHR", "VK_THREAD_IDLE_KHR A deferred operation is not complete but there is currently no work for this thread to do at the time of this call.")
	case vk.ThreadDone:
		return ConditionalOperator(!getExtended, "VK_THREAD_DONE_KHR", "VK_THREAD_DONE_KHR A deferred operation is not complete but there is no work remaining to assign to additional threads.")
	case vk.OperationDeferred:
		return ConditionalOperator(!getExtended, "VK_OPERATION_DEFERRED_KHR", "VK_OPERATION_DEFERRED_KHR A deferred operation was requested and at least some of the work was deferred.")
	case vk.OperationNotDeferred:
		return ConditionalOperator(!getExtended, "VK_OPERATION_NOT_DEFERRED_KHR", "VK_OPERATION_NOT_DEFERRED_KHR A deferred operation was requested and no operations were deferred.")
	case vk.PipelineCompileRequired:
		return ConditionalOperator(!getExtended, "VK_PIPELINE_COMPILE_REQUIRED_EXT", "VK_PIPELINE_COMPILE_REQUIRED_EXT A requested pipeline creation would have required compilation, but the application requested compilation to not be performed.")

	// Error codes
	case vk.ErrorOutOfHostMemory:
		return ConditionalOperator(!getExtended, "VK_ERROR_OUT_OF_HOST_MEMORY", "VK_ERROR_OUT_OF_HOST_MEMORY A host memory allocation has failed.")
	case vk.ErrorOutOfDeviceMemory:
		return ConditionalOperator(!getExtended, "VK_ERROR_OUT_OF_DEVICE_MEMORY", "VK_ERROR_OUT_OF_DEVICE_MEMORY A device memory allocation has failed.")
	case vk.ErrorInitializationFailed:
		return ConditionalOperator(!getExtended, "VK_ERROR_INITIALIZATION_FAILED", "VK_ERROR_INITIALIZATION_FAILED Initialization of an object could not be completed for implementation-specific reasons.")
	case vk.ErrorDeviceLost:
		return ConditionalOperator(!getExtended, "VK_ERROR_DEVICE_LOST", "VK_ERROR_DEVICE_LOST The logical or physical device has been lost. See Lost Device")
	case vk.ErrorMemoryMapFailed:
		return ConditionalOperator(!getExtended, "VK_ERROR_MEMORY_MAP_FAILED", "VK_ERROR_MEMORY_MAP_FAILED Mapping of a memory object has failed.")
	case vk.ErrorLayerNotPresent:
		return ConditionalOperator(!getExtended, "VK_ERROR_LAYER_NOT_PRESENT", "VK_ERROR_LAYER_NOT_PRESENT A requested layer is not present or could not be loaded.")
	case vk.ErrorExtensionNotPresent:
		return ConditionalOperator(!getExtended, "VK_ERROR_EXTENSION_NOT_PRESENT", "VK_ERROR_EXTENSION_NOT_PRESENT A requested extension is not supported.")
	case vk.ErrorFeatureNotPresent:
		return ConditionalOperator(!getExtended, "VK_ERROR_FEATURE_NOT_PRESENT", "VK_ERROR_FEATURE_NOT_PRESENT A requested feature is not supported.")
	case vk.ErrorIncompatibleDriver:
		return ConditionalOperator(!getExtended, "VK_ERROR_INCOMPATIBLE_DRIVER", "VK_ERROR_INCOMPATIBLE_DRIVER The requested version of Vulkan is not supported by the driver or is otherwise incompatible for implementation-specific reasons.")
	case vk.ErrorTooManyObjects:
		return ConditionalOperator(!getExtended, "VK_ERROR_TOO_MANY_OBJECTS", "VK_ERROR_TOO_MANY_OBJECTS Too many objects of the type have already been created.")
	case vk.ErrorFormatNotSupported:
		return ConditionalOperator(!getExtended, "VK_ERROR_FORMAT_NOT_SUPPORTED", "VK_ERROR_FORMAT_NOT_SUPPORTED A requested format is not supported on this device.")
	case vk.ErrorFragmentedPool:
		return ConditionalOperator(!getExtended, "VK_ERROR_FRAGMENTED_POOL", "VK_ERROR_FRAGMENTED_POOL A pool allocation has failed due to fragmentation of the pool’s memory. This must only be returned if no attempt to allocate host or device memory was made to accommodate the new allocation. This should be returned in preference to VK_ERROR_OUT_OF_POOL_MEMORY, but only if the implementation is certain that the pool allocation failure was due to fragmentation.")
	case vk.ErrorSurfaceLost:
		return ConditionalOperator(!getExtended, "VK_ERROR_SURFACE_LOST_KHR", "VK_ERROR_SURFACE_LOST_KHR A surface is no longer available.")
	case vk.ErrorNativeWindowInUse:
		return ConditionalOperator(!getExtended, "VK_ERROR_NATIVE_WINDOW_IN_USE_KHR", "VK_ERROR_NATIVE_WINDOW_IN_USE_KHR The requested window is already in use by Vulkan or another API in a manner which prevents it from being used again.")
	case vk.ErrorOutOfDate:
		return ConditionalOperator(!getExtended, "VK_ERROR_OUT_OF_DATE_KHR", "VK_ERROR_OUT_OF_DATE_KHR A surface has changed in such a way that it is no longer compatible with the swapchain, and further presentation requests using the swapchain will fail. Applications must query the new surface properties and recreate their swapchain if they wish to continue presenting to the surface.")
	case vk.ErrorIncompatibleDisplay:
		return ConditionalOperator(!getExtended, "VK_ERROR_INCOMPATIBLE_DISPLAY_KHR", "VK_ERROR_INCOMPATIBLE_DISPLAY_KHR The display used by a swapchain does not use the same presentable image layout, or is incompatible in a way that prevents sharing an image.")
	case vk.ErrorInvalidShaderNv:
		return ConditionalOperator(!getExtended, "VK_ERROR_INVALID_SHADER_NV", "VK_ERROR_INVALID_SHADER_NV One or more shaders failed to compile or link. More details are reported back to the application via VK_EXT_debug_report if enabled.")
	case vk.ErrorOutOfPoolMemory:
		return ConditionalOperator(!getExtended, "VK_ERROR_OUT_OF_POOL_MEMORY", "VK_ERROR_OUT_OF_POOL_MEMORY A pool memory allocation has failed. This must only be returned if no attempt to allocate host or device memory was made to accommodate the new allocation. If the failure was definitely due to fragmentation of the pool, VK_ERROR_FRAGMENTED_POOL should be returned instead.")
	case vk.ErrorInvalidExternalHandle:
		return ConditionalOperator(!getExtended, "VK_ERROR_INVALID_EXTERNAL_HANDLE", "VK_ERROR_INVALID_EXTERNAL_HANDLE An external handle is not a valid handle of the specified type.")
	case vk.ErrorFragmentation:
		return ConditionalOperator(!getExtended, "VK_ERROR_FRAGMENTATION", "VK_ERROR_FRAGMENTATION A descriptor pool creation has failed due to fragmentation.")
	case vk.ErrorInvalidDeviceAddress:
		return ConditionalOperator(!getExtended, "VK_ERROR_INVALID_DEVICE_ADDRESS_EXT", "VK_ERROR_INVALID_DEVICE_ADDRESS_EXT A buffer creation failed because the requested address is not available.")
	// NOTE: Same as above
	//case VK_ERROR_INVALID_OPAQUE_CAPTURE_ADDRESS:
	//    return conditionalOperator(!getExtended, "VK_ERROR_INVALID_OPAQUE_CAPTURE_ADDRESS" ,"VK_ERROR_INVALID_OPAQUE_CAPTURE_ADDRESS A buffer creation or memory allocation failed because the requested address is not available. A shader group handle assignment failed because the requested shader group handle information is no longer valid.")
	case vk.ErrorFullScreenExclusiveModeLost:
		return ConditionalOperator(!getExtended, "VK_ERROR_FULL_SCREEN_EXCLUSIVE_MODE_LOST_EXT", "VK_ERROR_FULL_SCREEN_EXCLUSIVE_MODE_LOST_EXT An operation on a swapchain created with VK_FULL_SCREEN_EXCLUSIVE_APPLICATION_CONTROLLED_EXT failed as it did not have exlusive full-screen access. This may occur due to implementation-dependent reasons, outside of the application’s control.")
	case vk.ErrorUnknown:
		return ConditionalOperator(!getExtended, "VK_ERROR_UNKNOWN", "VK_ERROR_UNKNOWN An unknown error has occurred; either the application has provided invalid input, or an implementation failure has occurred.")
	}
}

func VulkanResultIsSuccess(result vk.Result) bool {
	// From: https://www.khronos.org/registry/vulkan/specs/1.3-extensions/man/html/VkResult.html
	switch result {
	// Success Codes
	default:
		fallthrough
	case vk.Success, vk.NotReady, vk.Timeout, vk.EventSet, vk.EventReset,
		vk.Incomplete, vk.Suboptimal, vk.ThreadIdle, vk.ThreadDone,
		vk.OperationDeferred, vk.OperationNotDeferred, vk.PipelineCompileRequired:
		return true
	// Error codes
	case vk.ErrorOutOfHostMemory, vk.ErrorOutOfDeviceMemory, vk.ErrorInitializationFailed,
		vk.ErrorDeviceLost, vk.ErrorMemoryMapFailed, vk.ErrorLayerNotPresent,
		vk.ErrorExtensionNotPresent, vk.ErrorFeatureNotPresent, vk.ErrorIncompatibleDriver,
		vk.ErrorTooManyObjects, vk.ErrorFormatNotSupported, vk.ErrorFragmentedPool,
		vk.ErrorSurfaceLost, vk.ErrorNativeWindowInUse, vk.ErrorOutOfDate, vk.ErrorIncompatibleDisplay,
		vk.ErrorInvalidShaderNv, vk.ErrorOutOfPoolMemory, vk.ErrorInvalidExternalHandle,
		vk.ErrorFragmentation, vk.ErrorInvalidDeviceAddress, vk.ErrorFullScreenExclusiveModeLost,
		vk.ErrorUnknown:
		return false
	}
}

func ConditionalOperator(condition bool, res1, res2 string) string {
	if condition {
		return res1
	} else {
		return res2
	}
}

var end = "\x00"
var endChar byte = '\x00'

func VulkanSafeString(s string) string {
	if len(s) == 0 {
		return end
	}
	if s[len(s)-1] != endChar {
		return s + end
	}
	return s
}

func VulkanSafeStrings(list []string) []string {
	for i := range list {
		list[i] = VulkanSafeString(list[i])
	}
	return list
}

func FindFirstZeroInByteArray(arr []byte) int {
	end := 0
	for i, b := range arr {
		if b == 0 {
			end = i
			break
		}
	}
	return end
}
