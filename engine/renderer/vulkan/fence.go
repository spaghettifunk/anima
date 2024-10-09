package vulkan

import (
	"fmt"

	vk "github.com/goki/vulkan"
	"github.com/spaghettifunk/anima/engine/core"
)

type VulkanFence struct {
	Handle     vk.Fence
	IsSignaled bool
}

func NewFence(context *VulkanContext, createSignaled bool) (*VulkanFence, error) {
	fence := &VulkanFence{
		// Make sure to signal the fence if required.
		IsSignaled: createSignaled,
	}

	fenceCreateInfo := vk.FenceCreateInfo{
		SType: vk.StructureTypeFenceCreateInfo,
	}
	if fence.IsSignaled {
		fenceCreateInfo.Flags = vk.FenceCreateFlags(vk.FenceCreateSignaledBit)
	}

	var pFence vk.Fence
	if res := vk.CreateFence(context.Device.LogicalDevice, &fenceCreateInfo, context.Allocator, &pFence); res != vk.Success {
		err := fmt.Errorf("failed to create fence")
		core.LogError(err.Error())
		return nil, err
	}
	fenceCreateInfo.Deref()
	fence.Handle = pFence
	return fence, nil
}

func (vf *VulkanFence) FenceDestroy(context *VulkanContext) {
	if vf.Handle != nil {
		vk.DestroyFence(context.Device.LogicalDevice, vf.Handle, context.Allocator)
		vf.Handle = nil
	}
	vf.IsSignaled = false
}

func (vf *VulkanFence) FenceWait(context *VulkanContext, timeoutNs uint64) bool {
	if !vf.IsSignaled {
		result := vk.WaitForFences(context.Device.LogicalDevice, 1, []vk.Fence{vf.Handle}, vk.True, timeoutNs)
		switch result {
		case vk.Success:
			vf.IsSignaled = true
			return true
		case vk.Timeout:
			core.LogWarn("vk_fence_wait - Timed out")
		case vk.ErrorDeviceLost:
			core.LogError("vk_fence_wait - VK_ERROR_DEVICE_LOST.")
		case vk.ErrorOutOfHostMemory:
			core.LogError("vk_fence_wait - VK_ERROR_OUT_OF_HOST_MEMORY.")
		case vk.ErrorOutOfDeviceMemory:
			core.LogError("vk_fence_wait - VK_ERROR_OUT_OF_DEVICE_MEMORY.")
		default:
			core.LogError("vk_fence_wait - An unknown error has occurred.")
		}
	} else {
		// If already signaled, do not wait.
		return true
	}
	return false
}

func (vf *VulkanFence) FenceReset(context *VulkanContext) error {
	if vf.IsSignaled {
		if res := vk.ResetFences(context.Device.LogicalDevice, 1, []vk.Fence{vf.Handle}); res != vk.Success {
			err := fmt.Errorf("failed to reset fence")
			core.LogError(err.Error())
			return err
		}
		vf.IsSignaled = false
	}
	return nil
}
