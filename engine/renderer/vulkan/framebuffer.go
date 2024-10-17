package vulkan

import (
	"fmt"

	vk "github.com/goki/vulkan"
	"github.com/spaghettifunk/anima/engine/core"
)

type VulkanFramebuffer struct {
	Handle          vk.Framebuffer
	AttachmentCount uint32
	Attachments     []vk.ImageView
	Renderpass      *VulkanRenderpass
}

func FramebufferCreate(context *VulkanContext, renderpass *VulkanRenderpass, width uint32, height uint32, attachment_count uint32, attachments []vk.ImageView) (*VulkanFramebuffer, error) {
	outFramebuffer := &VulkanFramebuffer{
		Attachments:     make([]vk.ImageView, attachment_count),
		Renderpass:      renderpass,
		AttachmentCount: attachment_count,
	}
	// Take a copy of the attachments, renderpass and attachment count
	// out_framebuffer->attachments = kallocate(sizeof(VkImageView) * attachment_count, MEMORY_TAG_RENDERER);
	for i := 0; i < int(attachment_count); i++ {
		outFramebuffer.Attachments[i] = attachments[i]
	}

	// Creation info
	framebuffer_create_info := vk.FramebufferCreateInfo{
		SType:           vk.StructureTypeFramebufferCreateInfo,
		RenderPass:      renderpass.Handle,
		AttachmentCount: attachment_count,
		PAttachments:    outFramebuffer.Attachments,
		Width:           width,
		Height:          height,
		Layers:          1,
	}

	var pFramebuffer vk.Framebuffer
	if res := vk.CreateFramebuffer(context.Device.LogicalDevice, &framebuffer_create_info, context.Allocator, &pFramebuffer); res != vk.Success {
		err := fmt.Errorf("failed to create famebuffer")
		core.LogError(err.Error())
		return nil, err
	}
	outFramebuffer.Handle = pFramebuffer
	return outFramebuffer, nil
}

func (vfb *VulkanFramebuffer) Destroy(context *VulkanContext) {
	vk.DestroyFramebuffer(context.Device.LogicalDevice, vfb.Handle, context.Allocator)
	if len(vfb.Attachments) > 0 {
		vfb.Attachments = nil
	}
	vfb.Handle = nil
	vfb.AttachmentCount = 0
	vfb.Renderpass = nil
}
