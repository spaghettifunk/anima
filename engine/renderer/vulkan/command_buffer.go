package vulkan

import (
	"fmt"

	vk "github.com/goki/vulkan"
)

type VulkanCommandBufferState int

const (
	COMMAND_BUFFER_STATE_READY VulkanCommandBufferState = iota
	COMMAND_BUFFER_STATE_RECORDING
	COMMAND_BUFFER_STATE_IN_RENDER_PASS
	COMMAND_BUFFER_STATE_RECORDING_ENDED
	COMMAND_BUFFER_STATE_SUBMITTED
	COMMAND_BUFFER_STATE_NOT_ALLOCATED
)

type VulkanCommandBuffer struct {
	Handle vk.CommandBuffer
	// Command buffer state.
	State VulkanCommandBufferState
}

func NewVulkanCommandBuffer(context *VulkanContext, pool vk.CommandPool, isPrimary bool) (*VulkanCommandBuffer, error) {
	vCommandBuffer := &VulkanCommandBuffer{
		State: COMMAND_BUFFER_STATE_NOT_ALLOCATED,
	}

	level := vk.CommandBufferLevelPrimary
	if !isPrimary {
		level = vk.CommandBufferLevelSecondary
	}

	allocate_info := vk.CommandBufferAllocateInfo{
		SType:              vk.StructureTypeCommandBufferAllocateInfo,
		CommandPool:        pool,
		CommandBufferCount: 1,
		Level:              level,
		PNext:              nil,
	}
	allocate_info.Deref()

	pCommandBuffers := make([]vk.CommandBuffer, 1)
	if err := lockPool.SafeCall(CommandBufferManagement, func() error {
		if res := vk.AllocateCommandBuffers(context.Device.LogicalDevice, &allocate_info, pCommandBuffers); !VulkanResultIsSuccess(res) {
			err := fmt.Errorf("failed to allocate command buffer with error %s", VulkanResultString(res, true))
			return err
		}
		return nil
	}); err != nil {
		return nil, err
	}
	vCommandBuffer.Handle = pCommandBuffers[0]
	vCommandBuffer.State = COMMAND_BUFFER_STATE_READY

	return vCommandBuffer, nil
}

func (v *VulkanCommandBuffer) Free(context *VulkanContext, pool vk.CommandPool) error {
	if err := lockPool.SafeCall(CommandBufferManagement, func() error {
		vk.FreeCommandBuffers(context.Device.LogicalDevice, pool, 1, []vk.CommandBuffer{v.Handle})
		return nil
	}); err != nil {
		return err
	}
	v.Handle = nil
	v.State = COMMAND_BUFFER_STATE_NOT_ALLOCATED
	return nil
}

func (v *VulkanCommandBuffer) Begin(isSingleUse, isRenderpassContinue, isSimultaneousUse bool) error {
	vBeginInfo := vk.CommandBufferBeginInfo{
		SType: vk.StructureTypeCommandBufferBeginInfo,
		Flags: 0,
	}

	if isSingleUse {
		vBeginInfo.Flags |= vk.CommandBufferUsageFlags(vk.CommandBufferUsageOneTimeSubmitBit)
	}
	if isRenderpassContinue {
		vBeginInfo.Flags |= vk.CommandBufferUsageFlags(vk.CommandBufferUsageRenderPassContinueBit)
	}
	if isSimultaneousUse {
		vBeginInfo.Flags |= vk.CommandBufferUsageFlags(vk.CommandBufferUsageSimultaneousUseBit)
	}
	vBeginInfo.Deref()

	if err := lockPool.SafeCall(CommandBufferManagement, func() error {
		if res := vk.BeginCommandBuffer(v.Handle, &vBeginInfo); !VulkanResultIsSuccess(res) {
			err := fmt.Errorf("failed to begin command buffer with error %s", VulkanResultString(res, true))
			return err
		}
		return nil
	}); err != nil {
		return err
	}
	v.State = COMMAND_BUFFER_STATE_RECORDING
	return nil
}

func (v *VulkanCommandBuffer) End() error {
	if err := lockPool.SafeCall(CommandBufferManagement, func() error {
		if res := vk.EndCommandBuffer(v.Handle); !VulkanResultIsSuccess(res) {
			err := fmt.Errorf("failed to end command buffer with error %s", VulkanResultString(res, true))
			return err
		}
		return nil
	}); err != nil {
		return err
	}
	v.State = COMMAND_BUFFER_STATE_RECORDING_ENDED
	return nil
}

func (v *VulkanCommandBuffer) UpdateSubmitted() {
	v.State = COMMAND_BUFFER_STATE_SUBMITTED
}

func (v *VulkanCommandBuffer) Reset() {
	v.State = COMMAND_BUFFER_STATE_READY
}

/**
 * Allocates and begins recording to out_command_buffer.
 */
func AllocateAndBeginSingleUse(context *VulkanContext, pool vk.CommandPool) (*VulkanCommandBuffer, error) {
	cb, err := NewVulkanCommandBuffer(context, pool, true)
	if err != nil {
		return nil, err
	}

	if err := cb.Begin(true, false, false); err != nil {
		return nil, err
	}
	return cb, nil
}

/**
 * Ends recording, submits to and waits for queue operation and frees the provided command buffer.
 */
func (v *VulkanCommandBuffer) EndSingleUse(context *VulkanContext, pool vk.CommandPool, queue vk.Queue, queueIndex uint32) error {
	// End the command buffer.
	if err := v.End(); err != nil {
		return err
	}

	// Submit the queue
	submitInfo := vk.SubmitInfo{
		SType:              vk.StructureTypeSubmitInfo,
		CommandBufferCount: 1,
		PCommandBuffers:    []vk.CommandBuffer{v.Handle},
	}
	submitInfo.Deref()

	if err := lockPool.SafeQueueCall(queueIndex, func() error {
		if res := vk.QueueSubmit(queue, 1, []vk.SubmitInfo{submitInfo}, nil); !VulkanResultIsSuccess(res) {
			err := fmt.Errorf("%s", VulkanResultString(res, true))
			return err
		}
		return nil
	}); err != nil {
		return err
	}

	// Wait for it to finish
	if err := lockPool.SafeQueueCall(queueIndex, func() error {
		if res := vk.QueueWaitIdle(queue); !VulkanResultIsSuccess(res) {
			err := fmt.Errorf("%s", VulkanResultString(res, true))
			return err
		}
		return nil
	}); err != nil {
		return err
	}

	// Free the command buffer.
	v.Free(context, pool)

	return nil
}
