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

type VulkanPipelineConfig struct {
	/** @brief A pointer to the renderpass to associate with the pipeline. */
	Renderpass *VulkanRenderPass
	/** @brief The stride of the vertex data to be used (ex: sizeof(vertex_3d)) */
	Stride uint32
	/** @brief An array of attributes. */
	Attributes []vk.VertexInputAttributeDescription
	/** @brief An array of descriptor set layouts. */
	DescriptorSetLayouts []vk.DescriptorSetLayout
	/** @brief An VK_ACCESS_DEPTH_STENCIL_ATTACHMENT_WRITE_BITarray of stages. */
	Stages []vk.PipelineShaderStageCreateInfo
	/** @brief The initial viewport configuration. */
	Viewport vk.Viewport
	/** @brief The initial scissor configuration. */
	Scissor vk.Rect2D
	/** @brief The face cull mode. */
	CullMode metadata.FaceCullMode
	/** @brief Indicates if this pipeline should use wireframe mode. */
	IsWireframe bool
	/** @brief The shader flags used for creating the pipeline. */
	ShaderFlags metadata.ShaderFlagBits
	/** @brief An array of push constant data ranges. */
	PushConstantRanges []*metadata.MemoryRange
}

func NewGraphicsPipeline(context *VulkanContext, config *VulkanPipelineConfig) (*VulkanPipeline, error) {

	out_pipeline := &VulkanPipeline{}

	// Viewport state
	viewport_state := vk.PipelineViewportStateCreateInfo{
		SType:         vk.StructureTypePipelineViewportStateCreateInfo,
		ViewportCount: 1,
		PViewports:    []vk.Viewport{config.Viewport},
		ScissorCount:  1,
		PScissors:     []vk.Rect2D{config.Scissor},
	}
	viewport_state.Deref()

	// Rasterizer
	rasterizer_create_info := vk.PipelineRasterizationStateCreateInfo{
		SType:                   vk.StructureTypePipelineRasterizationStateCreateInfo,
		DepthClampEnable:        vk.False,
		RasterizerDiscardEnable: vk.False,
		PolygonMode:             vk.PolygonModeLine,
		LineWidth:               1.0,
	}
	if !config.IsWireframe {
		rasterizer_create_info.PolygonMode = vk.PolygonModeFill
	}
	switch config.CullMode {
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
	rasterizer_create_info.Deref()

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
	multisampling_create_info.Deref()

	// Depth and stencil testing.
	depth_stencil := vk.PipelineDepthStencilStateCreateInfo{
		SType:             vk.StructureTypePipelineDepthStencilStateCreateInfo,
		DepthTestEnable:   vk.False,
		DepthWriteEnable:  vk.False,
		StencilTestEnable: vk.False,
	}
	if (metadata.ShaderFlags(config.ShaderFlags) & metadata.SHADER_FLAG_DEPTH_TEST) != 0 {
		depth_stencil.DepthTestEnable = vk.True
		depth_stencil.DepthCompareOp = vk.CompareOpLess
		depth_stencil.DepthBoundsTestEnable = vk.False
		depth_stencil.StencilTestEnable = vk.False
	}
	if (metadata.ShaderFlags(config.ShaderFlags) & metadata.SHADER_FLAG_DEPTH_WRITE) != 0 {
		depth_stencil.DepthWriteEnable = vk.True
	}
	depth_stencil.Deref()

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
	color_blend_attachment_state.Deref()

	color_blend_state_create_info := vk.PipelineColorBlendStateCreateInfo{
		SType:           vk.StructureTypePipelineColorBlendStateCreateInfo,
		LogicOpEnable:   vk.False,
		LogicOp:         vk.LogicOpCopy,
		AttachmentCount: 1,
		PAttachments:    []vk.PipelineColorBlendAttachmentState{color_blend_attachment_state},
	}
	color_blend_state_create_info.Deref()

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
	dynamic_state_create_info.Deref()

	// Vertex input
	binding_description := vk.VertexInputBindingDescription{
		Binding:   0, // Binding index
		Stride:    config.Stride,
		InputRate: vk.VertexInputRateVertex, // Move to next data entry for each vertex.
	}
	binding_description.Deref()

	// Attributes
	vertex_input_info := vk.PipelineVertexInputStateCreateInfo{
		SType:                           vk.StructureTypePipelineVertexInputStateCreateInfo,
		VertexBindingDescriptionCount:   1,
		PVertexBindingDescriptions:      []vk.VertexInputBindingDescription{binding_description},
		VertexAttributeDescriptionCount: uint32(len(config.Attributes)),
		PVertexAttributeDescriptions:    config.Attributes,
	}
	vertex_input_info.Deref()

	// Input assembly
	input_assembly := vk.PipelineInputAssemblyStateCreateInfo{
		SType:                  vk.StructureTypePipelineInputAssemblyStateCreateInfo,
		Topology:               vk.PrimitiveTopologyTriangleList,
		PrimitiveRestartEnable: vk.False,
	}
	input_assembly.Deref()

	// Pipeline layout
	pipeline_layout_create_info := vk.PipelineLayoutCreateInfo{
		SType:          vk.StructureTypePipelineLayoutCreateInfo,
		SetLayoutCount: uint32(len(config.DescriptorSetLayouts)),
		PSetLayouts:    config.DescriptorSetLayouts,
	}

	// Push constants
	if len(config.PushConstantRanges) > 0 {
		if len(config.PushConstantRanges) > 32 {
			err := fmt.Errorf("func NewGraphicsPipeline: cannot have more than 32 push constant ranges. Passed count: %d", len(config.PushConstantRanges))
			return nil, err
		}

		// NOTE: 32 is the max number of ranges we can ever have, since spec only guarantees 128 bytes with 4-byte alignment.
		ranges := make([]vk.PushConstantRange, 32)
		for i := 0; i < len(config.PushConstantRanges); i++ {
			ranges[i].StageFlags = vk.ShaderStageFlags(vk.ShaderStageVertexBit) | vk.ShaderStageFlags(vk.ShaderStageFragmentBit)
			ranges[i].Offset = uint32(config.PushConstantRanges[i].Offset)
			ranges[i].Size = uint32(config.PushConstantRanges[i].Size)
			ranges[i].Deref()
		}
		pipeline_layout_create_info.PushConstantRangeCount = uint32(len(config.PushConstantRanges))
		pipeline_layout_create_info.PPushConstantRanges = ranges
	} else {
		pipeline_layout_create_info.PushConstantRangeCount = 0
		pipeline_layout_create_info.PPushConstantRanges = nil
	}
	pipeline_layout_create_info.Deref()

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
		StageCount:          uint32(len(config.Stages)),
		PStages:             config.Stages,
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
		RenderPass:          config.Renderpass.Handle,
		Subpass:             0,
		BasePipelineHandle:  vk.NullPipeline,
		BasePipelineIndex:   -1,
	}
	pipeline_create_info.Deref()

	if (metadata.ShaderFlags(config.ShaderFlags) & metadata.SHADER_FLAG_DEPTH_TEST) == 0 {
		pipeline_create_info.PDepthStencilState = nil
	}

	pPipelines := []vk.Pipeline{out_pipeline.Handle}
	result = vk.CreateGraphicsPipelines(
		context.Device.LogicalDevice,
		vk.NullPipelineCache,
		1,
		[]vk.GraphicsPipelineCreateInfo{pipeline_create_info},
		context.Allocator,
		pPipelines)

	if !VulkanResultIsSuccess(result) {
		err := fmt.Errorf("vkCreateGraphicsPipelines failed with %s", VulkanResultString(result, true))
		return nil, err
	}

	if len(pPipelines) <= 0 {
		err := fmt.Errorf("vulkan pipeline handle is nil")
		return nil, err
	}

	out_pipeline.Handle = pPipelines[0]

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
