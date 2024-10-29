package vulkan

import (
	"fmt"

	vk "github.com/goki/vulkan"
	"github.com/spaghettifunk/anima/engine/renderer/metadata"
)

type VulkanRenderPassState int

const (
	READY VulkanRenderPassState = iota
	RECORDING
	IN_RENDER_PASS
	RECORDING_ENDED
	SUBMITTED
	NOT_ALLOCATED
)

type VulkanRenderPass struct {
	Handle vk.RenderPass
	/** @brief Indicates if there is a previous renderpass. */
	HasPrevPass bool
	/** @brief Indicates if there is a next renderpass. */
	HasNextPass bool
	Depth       float32
	Stencil     uint32
	State       VulkanRenderPassState
}

func RenderpassCreate(context *VulkanContext, renderPass *metadata.RenderPass, depth float32, stencil uint32, has_prev_pass, has_next_pass bool) (*metadata.RenderPass, error) {
	internal_data := renderPass.InternalData.(*VulkanRenderPass)
	internal_data.HasPrevPass = has_prev_pass
	internal_data.HasNextPass = has_next_pass
	internal_data.Depth = depth
	internal_data.Stencil = stencil

	// Main subpass
	subpass := vk.SubpassDescription{
		PipelineBindPoint: vk.PipelineBindPointGraphics,
	}

	// Attachments TODO: make this configurable.
	attachment_description_count := uint32(0)
	attachment_descriptions := make([]vk.AttachmentDescription, 2)

	// Color attachment
	do_clear_colour := (renderPass.ClearFlags & uint8(metadata.RENDERPASS_CLEAR_COLOUR_BUFFER_FLAG)) != 0
	color_attachment := vk.AttachmentDescription{
		Format:         context.Swapchain.ImageFormat.Format, // TODO: configurable
		Samples:        vk.SampleCount1Bit,
		LoadOp:         vk.AttachmentLoadOpClear,
		StoreOp:        vk.AttachmentStoreOpStore,
		StencilLoadOp:  vk.AttachmentLoadOpDontCare,
		StencilStoreOp: vk.AttachmentStoreOpDontCare,
		// If coming from a previous pass, should already be VK_IMAGE_LAYOUT_COLOR_ATTACHMENT_OPTIMAL. Otherwise undefined.
		InitialLayout: vk.ImageLayoutColorAttachmentOptimal,
		// If going to another pass, use VK_IMAGE_LAYOUT_COLOR_ATTACHMENT_OPTIMAL. Otherwise VK_IMAGE_LAYOUT_PRESENT_SRC_KHR.
		FinalLayout: vk.ImageLayoutColorAttachmentOptimal, // Transitioned to after the render pass
		Flags:       0,
	}

	if do_clear_colour {
		color_attachment.LoadOp = vk.AttachmentLoadOpLoad
	}

	if !has_prev_pass {
		color_attachment.InitialLayout = vk.ImageLayoutUndefined
	}

	if !has_next_pass {
		color_attachment.FinalLayout = vk.ImageLayoutPresentSrc
	}

	attachment_descriptions[attachment_description_count] = color_attachment
	attachment_description_count++

	color_attachment_reference := []vk.AttachmentReference{
		{
			Attachment: 0, // Attachment description array index
			Layout:     vk.ImageLayoutColorAttachmentOptimal,
		},
	}

	subpass.ColorAttachmentCount = 1
	subpass.PColorAttachments = color_attachment_reference

	// Depth attachment, if there is one
	do_clear_depth := (renderPass.ClearFlags & uint8(metadata.RENDERPASS_CLEAR_DEPTH_BUFFER_FLAG)) != 0
	if do_clear_depth {
		depth_attachment := vk.AttachmentDescription{
			Format:         context.Device.DepthFormat,
			Samples:        vk.SampleCount1Bit,
			StoreOp:        vk.AttachmentStoreOpDontCare,
			StencilLoadOp:  vk.AttachmentLoadOpDontCare,
			StencilStoreOp: vk.AttachmentStoreOpDontCare,
			InitialLayout:  vk.ImageLayoutUndefined,
			FinalLayout:    vk.ImageLayoutDepthStencilAttachmentOptimal,
		}

		if has_prev_pass {
			depth_attachment.LoadOp = vk.AttachmentLoadOpClear
			if do_clear_depth {
				depth_attachment.LoadOp = vk.AttachmentLoadOpLoad
			}
		} else {
			depth_attachment.LoadOp = vk.AttachmentLoadOpDontCare
		}

		attachment_descriptions[attachment_description_count] = depth_attachment
		attachment_description_count++

		// Depth attachment reference
		depth_attachment_reference := vk.AttachmentReference{
			Attachment: 1,
			Layout:     vk.ImageLayoutDepthStencilAttachmentOptimal,
		}

		// TODO: other attachment types (input, resolve, preserve)

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
		AttachmentCount: attachment_description_count,
		PAttachments:    attachment_descriptions,
		SubpassCount:    1,
		PSubpasses:      []vk.SubpassDescription{subpass},
		DependencyCount: 1,
		PDependencies:   []vk.SubpassDependency{dependency},
		PNext:           nil,
		Flags:           0,
	}

	if res := vk.CreateRenderPass(context.Device.LogicalDevice, &render_pass_create_info, context.Allocator, &internal_data.Handle); res != vk.Success {
		err := fmt.Errorf("failed to create renderpass")
		return nil, err
	}

	return renderPass, nil
}

func (vr *VulkanRenderPass) RenderpassDestroy(context *VulkanContext) {
	if vr.Handle != nil {
		vk.DestroyRenderPass(context.Device.LogicalDevice, vr.Handle, context.Allocator)
		vr.Handle = nil
	}
}

func (vr *VulkanRenderPass) RenderpassBegin(commandBuffer *VulkanCommandBuffer, frameBuffer vk.Framebuffer) {
	// beginInfo := vk.RenderPassBeginInfo{
	// 	SType:       vk.StructureTypeRenderPassBeginInfo,
	// 	RenderPass:  vr.Handle,
	// 	Framebuffer: frameBuffer,
	// 	RenderArea: vk.Rect2D{
	// 		Offset: vk.Offset2D{
	// 			X: int32(vr.X),
	// 			Y: int32(vr.Y),
	// 		},
	// 		Extent: vk.Extent2D{
	// 			Width:  uint32(vr.W),
	// 			Height: uint32(vr.H),
	// 		},
	// 	},
	// }
	// beginInfo.Deref()

	// clearValues := make([]vk.ClearValue, 2)

	// color := []float32{vr.R, vr.G, vr.B, vr.A}
	// clearValues[0].SetColor(color)
	// clearValues[1].SetDepthStencil(vr.Depth, vr.Stencil)

	// beginInfo.ClearValueCount = 2
	// beginInfo.PClearValues = clearValues

	// vk.CmdBeginRenderPass(commandBuffer.Handle, &beginInfo, vk.SubpassContentsInline)
	// commandBuffer.State = COMMAND_BUFFER_STATE_IN_RENDER_PASS
}

func (vr *VulkanRenderPass) RenderpassEnd(commandBuffer *VulkanCommandBuffer) {
	vk.CmdEndRenderPass(commandBuffer.Handle)
	commandBuffer.State = COMMAND_BUFFER_STATE_RECORDING
}
