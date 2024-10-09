package vulkan

import (
	"fmt"

	vk "github.com/goki/vulkan"
	"github.com/spaghettifunk/alaska-engine/engine/core"
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

	var allocate_info vk.CommandBufferAllocateInfo = vk.CommandBufferAllocateInfo{
		SType:              vk.StructureTypeCommandBufferAllocateInfo,
		CommandPool:        pool,
		CommandBufferCount: 1,
		Level:              level,
		PNext:              nil,
	}

	pCommandBuffers := []vk.CommandBuffer{vCommandBuffer.Handle}
	if res := vk.AllocateCommandBuffers(context.Device.LogicalDevice, &allocate_info, pCommandBuffers); res != vk.Success {
		err := fmt.Errorf("failed to allocate command buffer")
		core.LogError(err.Error())
		return nil, err
	}
	vCommandBuffer.Handle = pCommandBuffers[0]
	vCommandBuffer.State = COMMAND_BUFFER_STATE_READY

	return vCommandBuffer, nil
}

func (v *VulkanCommandBuffer) Free(context *VulkanContext, pool vk.CommandPool) {
	vk.FreeCommandBuffers(context.Device.LogicalDevice, pool, 1, []vk.CommandBuffer{v.Handle})
	v.Handle = nil
	v.State = COMMAND_BUFFER_STATE_NOT_ALLOCATED
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

	if res := vk.BeginCommandBuffer(v.Handle, &vBeginInfo); res != vk.Success {
		err := fmt.Errorf("failed to begin command buffer")
		core.LogError(err.Error())
		return err
	}
	v.State = COMMAND_BUFFER_STATE_RECORDING

	return nil
}

func (v *VulkanCommandBuffer) End() error {
	if res := vk.EndCommandBuffer(v.Handle); res != vk.Success {
		err := fmt.Errorf("failed to end command buffer")
		core.LogError(err.Error())
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
func (v *VulkanCommandBuffer) EndSingleUse(context *VulkanContext, pool vk.CommandPool, queue vk.Queue) error {
	// End the command buffer.
	if err := v.End(); err != nil {
		return err
	}

	// Submit the queue
	submit_info := vk.SubmitInfo{
		SType:              vk.StructureTypeSubmitInfo,
		CommandBufferCount: 1,
		PCommandBuffers:    []vk.CommandBuffer{v.Handle},
	}

	if res := vk.QueueSubmit(queue, 1, []vk.SubmitInfo{submit_info}, nil); res != vk.Success {
		err := fmt.Errorf("failed submit info to queue")
		core.LogError(err.Error())
		return err
	}

	// Wait for it to finish
	if res := vk.QueueWaitIdle(queue); res != vk.Success {
		err := fmt.Errorf("queue failed to wait in idle mode")
		core.LogError(err.Error())
		return err
	}

	// Free the command buffer.
	v.Free(context, pool)

	return nil
}
