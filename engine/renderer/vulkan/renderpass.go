package vulkan

import (
	vk "github.com/goki/vulkan"
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

type VulkanRenderpass struct {
	Handle     vk.RenderPass
	X, Y, W, H float32
	R, G, B, A float32

	Depth   float32
	Stencil uint32

	State VulkanRenderPassState
}

func RenderpassCreate(
	context *VulkanContext,
	x, y, w, h,
	r, g, b, a,
	depth float32,
	stencil uint32) (*VulkanRenderpass, error) {

	outRenderpass := &VulkanRenderpass{
		X:       x,
		Y:       y,
		W:       w,
		H:       h,
		R:       r,
		G:       g,
		B:       b,
		A:       a,
		Depth:   depth,
		Stencil: stencil,
	}

	// Main subpass
	subpass := vk.SubpassDescription{
		PipelineBindPoint: vk.PipelineBindPointGraphics,
	}

	// Attachments TODO: make this configurable.
	attachmentDescriptionCount := 2
	attachmentDescriptions := make([]vk.AttachmentDescription, attachmentDescriptionCount)

	// Color attachment
	colorAttachment := vk.AttachmentDescription{
		Format:         context.Swapchain.ImageFormat.Format, // TODO: configurable
		Samples:        vk.SampleCount1Bit,
		LoadOp:         vk.AttachmentLoadOpClear,
		StoreOp:        vk.AttachmentStoreOpStore,
		StencilLoadOp:  vk.AttachmentLoadOpDontCare,
		StencilStoreOp: vk.AttachmentStoreOpDontCare,
		InitialLayout:  vk.ImageLayoutUndefined,  // Do not expect any particular layout before render pass starts.
		FinalLayout:    vk.ImageLayoutPresentSrc, // Transitioned to after the render pass
		Flags:          0,
	}

	attachmentDescriptions[0] = colorAttachment

	colorAttachmentReference := []vk.AttachmentReference{
		{
			Attachment: 0, // Attachment description array index
			Layout:     vk.ImageLayoutColorAttachmentOptimal,
		},
	}

	subpass.ColorAttachmentCount = 1
	subpass.PColorAttachments = colorAttachmentReference

	// Depth attachment, if there is one
	depthAttachment := vk.AttachmentDescription{
		Format:         context.Device.DepthFormat,
		Samples:        vk.SampleCount1Bit,
		LoadOp:         vk.AttachmentLoadOpClear,
		StoreOp:        vk.AttachmentStoreOpDontCare,
		StencilLoadOp:  vk.AttachmentLoadOpDontCare,
		StencilStoreOp: vk.AttachmentStoreOpDontCare,
		InitialLayout:  vk.ImageLayoutUndefined,
		FinalLayout:    vk.ImageLayoutDepthStencilAttachmentOptimal,
	}

	attachmentDescriptions[1] = depthAttachment

	// Depth attachment reference
	depthAttachmentReference := vk.AttachmentReference{
		Attachment: 1,
		Layout:     vk.ImageLayoutDepthStencilAttachmentOptimal,
	}

	// TODO: other attachment types (input, resolve, preserve)

	// Depth stencil data.
	subpass.PDepthStencilAttachment = &depthAttachmentReference

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
	renderpassCreateInfo := vk.RenderPassCreateInfo{
		SType:           vk.StructureTypeRenderPassCreateInfo,
		AttachmentCount: uint32(attachmentDescriptionCount),
		PAttachments:    attachmentDescriptions,
		SubpassCount:    1,
		PSubpasses:      []vk.SubpassDescription{subpass},
		DependencyCount: 1,
		PNext:           nil,
		Flags:           0,
		PDependencies:   []vk.SubpassDependency{dependency},
	}

	if res := vk.CreateRenderPass(context.Device.LogicalDevice, &renderpassCreateInfo, context.Allocator, &outRenderpass.Handle); res != vk.Success {
		return nil, nil
	}
	return outRenderpass, nil
}

func (vr *VulkanRenderpass) RenderpassDestroy(context *VulkanContext) {
	if vr.Handle != nil {
		vk.DestroyRenderPass(context.Device.LogicalDevice, vr.Handle, context.Allocator)
		vr.Handle = nil
	}
}

func (vr *VulkanRenderpass) RenderpassBegin(command_buffer *VulkanCommandBuffer, frame_buffer vk.Framebuffer) {
	begin_info := vk.RenderPassBeginInfo{
		SType:       vk.StructureTypeRenderPassBeginInfo,
		RenderPass:  vr.Handle,
		Framebuffer: frame_buffer,
		RenderArea: vk.Rect2D{
			Offset: vk.Offset2D{
				X: int32(vr.X),
				Y: int32(vr.Y),
			},
			Extent: vk.Extent2D{
				Width:  uint32(vr.W),
				Height: uint32(vr.H),
			},
		},
	}

	clear_values := make([]vk.ClearValue, 2)

	color := []float32{vr.R, vr.G, vr.B, vr.A}
	clear_values[0].SetColor(color)
	clear_values[1].SetDepthStencil(vr.Depth, vr.Stencil)

	begin_info.ClearValueCount = 2
	begin_info.PClearValues = clear_values

	vk.CmdBeginRenderPass(command_buffer.Handle, &begin_info, vk.SubpassContentsInline)
	command_buffer.State = COMMAND_BUFFER_STATE_IN_RENDER_PASS
}

func (vr *VulkanRenderpass) RenderpassEnd(command_buffer *VulkanCommandBuffer) {
	vk.CmdEndRenderPass(command_buffer.Handle)
	command_buffer.State = COMMAND_BUFFER_STATE_RECORDING
}
