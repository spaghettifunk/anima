package vulkan

import (
	"fmt"

	vk "github.com/goki/vulkan"
	"github.com/spaghettifunk/anima/engine/core"
	"github.com/spaghettifunk/anima/engine/renderer/metadata"
)

/**
 * @brief Holds a Vulkan pipeline and its layout.
 */
type VulkanPipeline struct {
	/** @brief The internal pipeline handle. */
	Handle vk.Pipeline
	/** @brief The pipeline layout. */
	PipelineLayout vk.PipelineLayout
}

func NewGraphicsPipeline(
	context *VulkanContext,
	renderpass *VulkanRenderPass,
	stride uint32,
	attribute_count uint32,
	attributes []vk.VertexInputAttributeDescription,
	descriptor_set_layout_count uint32,
	descriptor_set_layouts []vk.DescriptorSetLayout,
	stage_count uint32,
	stages []vk.PipelineShaderStageCreateInfo,
	viewport vk.Viewport,
	scissor vk.Rect2D,
	cull_mode metadata.FaceCullMode,
	is_wireframe bool,
	depth_test_enabled bool,
	push_constant_range_count uint32,
	push_constant_ranges []*metadata.MemoryRange) (*VulkanPipeline, error) {

	out_pipeline := &VulkanPipeline{}

	// Viewport state
	viewport_state := vk.PipelineViewportStateCreateInfo{
		SType:         vk.StructureTypePipelineViewportStateCreateInfo,
		ViewportCount: 1,
		PViewports:    []vk.Viewport{viewport},
		ScissorCount:  1,
		PScissors:     []vk.Rect2D{scissor},
	}

	// Rasterizer
	rasterizer_create_info := vk.PipelineRasterizationStateCreateInfo{
		SType:                   vk.StructureTypePipelineRasterizationStateCreateInfo,
		DepthClampEnable:        vk.False,
		RasterizerDiscardEnable: vk.False,
		PolygonMode:             vk.PolygonModeLine,
		LineWidth:               1.0,
	}
	if !is_wireframe {
		rasterizer_create_info.PolygonMode = vk.PolygonModeFill
	}
	switch cull_mode {
	case metadata.FaceCullModeNone:
		rasterizer_create_info.CullMode = vk.CullModeFlags(vk.CullModeNone)
	case metadata.FaceCullModeFront:
		rasterizer_create_info.CullMode = vk.CullModeFlags(vk.CullModeFrontBit)
	case metadata.FaceCullModeFrontAndBack:
		rasterizer_create_info.CullMode = vk.CullModeFlags(vk.CullModeFrontAndBack)
	default:
		fallthrough
	case metadata.FaceCullModeBack:
		rasterizer_create_info.CullMode = vk.CullModeFlags(vk.CullModeBackBit)
	}
	rasterizer_create_info.FrontFace = vk.FrontFaceCounterClockwise
	rasterizer_create_info.DepthBiasEnable = vk.False
	rasterizer_create_info.DepthBiasConstantFactor = 0.0
	rasterizer_create_info.DepthBiasClamp = 0.0
	rasterizer_create_info.DepthBiasSlopeFactor = 0.0

	// Multisampling.
	multisampling_create_info := vk.PipelineMultisampleStateCreateInfo{
		SType:                 vk.StructureTypePipelineMultisampleStateCreateInfo,
		SampleShadingEnable:   vk.False,
		RasterizationSamples:  vk.SampleCount1Bit,
		MinSampleShading:      1.0,
		PSampleMask:           nil,
		AlphaToCoverageEnable: vk.False,
		AlphaToOneEnable:      vk.False,
	}

	// Depth and stencil testing.
	depth_stencil := vk.PipelineDepthStencilStateCreateInfo{
		SType: vk.StructureTypePipelineDepthStencilStateCreateInfo,
	}
	if depth_test_enabled {
		depth_stencil.DepthTestEnable = vk.True
		depth_stencil.DepthWriteEnable = vk.True
		depth_stencil.DepthCompareOp = vk.CompareOpLess
		depth_stencil.DepthBoundsTestEnable = vk.False
		depth_stencil.StencilTestEnable = vk.False
	}

	color_blend_attachment_state := vk.PipelineColorBlendAttachmentState{
		BlendEnable:         vk.True,
		SrcColorBlendFactor: vk.BlendFactorSrcAlpha,
		DstColorBlendFactor: vk.BlendFactorOneMinusSrcAlpha,
		ColorBlendOp:        vk.BlendOpAdd,
		SrcAlphaBlendFactor: vk.BlendFactorSrcAlpha,
		DstAlphaBlendFactor: vk.BlendFactorOneMinusSrcAlpha,
		AlphaBlendOp:        vk.BlendOpAdd,
		ColorWriteMask: vk.ColorComponentFlags(vk.ColorComponentRBit) | vk.ColorComponentFlags(vk.ColorComponentGBit) |
			vk.ColorComponentFlags(vk.ColorComponentBBit) | vk.ColorComponentFlags(vk.ColorComponentABit),
	}

	color_blend_state_create_info := vk.PipelineColorBlendStateCreateInfo{
		SType:           vk.StructureTypePipelineColorBlendStateCreateInfo,
		LogicOpEnable:   vk.False,
		LogicOp:         vk.LogicOpCopy,
		AttachmentCount: 1,
		PAttachments:    []vk.PipelineColorBlendAttachmentState{color_blend_attachment_state},
	}

	// Dynamic state
	dynamic_states := []vk.DynamicState{
		vk.DynamicStateViewport,
		vk.DynamicStateScissor,
		vk.DynamicStateLineWidth,
	}

	dynamic_state_create_info := vk.PipelineDynamicStateCreateInfo{
		SType:             vk.StructureTypePipelineDynamicStateCreateInfo,
		DynamicStateCount: uint32(len(dynamic_states)),
		PDynamicStates:    dynamic_states,
	}

	// Vertex input
	binding_description := vk.VertexInputBindingDescription{
		Binding:   0, // Binding index
		Stride:    stride,
		InputRate: vk.VertexInputRateVertex, // Move to next data entry for each vertex.
	}

	// Attributes
	vertex_input_info := vk.PipelineVertexInputStateCreateInfo{
		SType:                           vk.StructureTypePipelineVertexInputStateCreateInfo,
		VertexBindingDescriptionCount:   1,
		PVertexBindingDescriptions:      []vk.VertexInputBindingDescription{binding_description},
		VertexAttributeDescriptionCount: attribute_count,
		PVertexAttributeDescriptions:    attributes,
	}

	// Input assembly
	input_assembly := vk.PipelineInputAssemblyStateCreateInfo{
		SType:                  vk.StructureTypePipelineInputAssemblyStateCreateInfo,
		Topology:               vk.PrimitiveTopologyTriangleList,
		PrimitiveRestartEnable: vk.False,
	}

	// Pipeline layout
	pipeline_layout_create_info := vk.PipelineLayoutCreateInfo{
		SType:          vk.StructureTypePipelineLayoutCreateInfo,
		SetLayoutCount: descriptor_set_layout_count,
		PSetLayouts:    descriptor_set_layouts,
	}

	// Push constants
	if push_constant_range_count > 0 {
		if push_constant_range_count > 32 {
			err := fmt.Errorf("func NewGraphicsPipeline: cannot have more than 32 push constant ranges. Passed count: %d", push_constant_range_count)
			return nil, err
		}

		// NOTE: 32 is the max number of ranges we can ever have, since spec only guarantees 128 bytes with 4-byte alignment.
		ranges := make([]vk.PushConstantRange, 32)
		for i := uint32(0); i < push_constant_range_count; i++ {
			ranges[i].StageFlags = vk.ShaderStageFlags(vk.ShaderStageVertexBit) | vk.ShaderStageFlags(vk.ShaderStageFragmentBit)
			ranges[i].Offset = uint32(push_constant_ranges[i].Offset)
			ranges[i].Size = uint32(push_constant_ranges[i].Size)
		}
		pipeline_layout_create_info.PushConstantRangeCount = push_constant_range_count
		pipeline_layout_create_info.PPushConstantRanges = ranges
	} else {
		pipeline_layout_create_info.PushConstantRangeCount = 0
		pipeline_layout_create_info.PPushConstantRanges = nil
	}

	// Create the pipeline layout.
	var pPipelineLayout vk.PipelineLayout
	result := vk.CreatePipelineLayout(
		context.Device.LogicalDevice,
		&pipeline_layout_create_info,
		context.Allocator,
		&pPipelineLayout)
	if !VulkanResultIsSuccess(result) {
		err := fmt.Errorf("vkCreatePipelineLayout failed with %s", VulkanResultString(result, true))
		return nil, err
	}
	out_pipeline.PipelineLayout = pPipelineLayout

	// Pipeline create
	pipeline_create_info := vk.GraphicsPipelineCreateInfo{
		SType:               vk.StructureTypeGraphicsPipelineCreateInfo,
		StageCount:          stage_count,
		PStages:             stages,
		PVertexInputState:   &vertex_input_info,
		PInputAssemblyState: &input_assembly,
		PViewportState:      &viewport_state,
		PRasterizationState: &rasterizer_create_info,
		PMultisampleState:   &multisampling_create_info,
		PDepthStencilState:  &depth_stencil,
		PColorBlendState:    &color_blend_state_create_info,
		PDynamicState:       &dynamic_state_create_info,
		PTessellationState:  nil,
		Layout:              out_pipeline.PipelineLayout,
		RenderPass:          renderpass.Handle,
		Subpass:             0,
		BasePipelineHandle:  vk.NullPipeline,
		BasePipelineIndex:   -1,
	}

	if !depth_test_enabled {
		pipeline_create_info.PDepthStencilState = nil
	}

	result = vk.CreateGraphicsPipelines(
		context.Device.LogicalDevice,
		vk.NullPipelineCache,
		1,
		[]vk.GraphicsPipelineCreateInfo{pipeline_create_info},
		context.Allocator,
		[]vk.Pipeline{out_pipeline.Handle})

	if !VulkanResultIsSuccess(result) {
		err := fmt.Errorf("vkCreateGraphicsPipelines failed with %s", VulkanResultString(result, true))
		return nil, err
	}

	core.LogDebug("Graphics pipeline created!")
	return out_pipeline, nil
}

func (pipeline *VulkanPipeline) Destroy(context *VulkanContext) {
	// Destroy pipeline
	if pipeline.Handle != nil {
		vk.DestroyPipeline(context.Device.LogicalDevice, pipeline.Handle, context.Allocator)
		pipeline.Handle = nil
	}

	// Destroy layout
	if pipeline.PipelineLayout != nil {
		vk.DestroyPipelineLayout(context.Device.LogicalDevice, pipeline.PipelineLayout, context.Allocator)
		pipeline.PipelineLayout = nil
	}
}

func (pipeline *VulkanPipeline) Bind(command_buffer *VulkanCommandBuffer, bind_point vk.PipelineBindPoint) {
	vk.CmdBindPipeline(command_buffer.Handle, bind_point, pipeline.Handle)
}
