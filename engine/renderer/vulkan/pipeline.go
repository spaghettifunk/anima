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
	outPipeline := &VulkanPipeline{}

	// Viewport state
	viewportState := vk.PipelineViewportStateCreateInfo{
		SType:         vk.StructureTypePipelineViewportStateCreateInfo,
		ViewportCount: 1,
		PViewports:    []vk.Viewport{config.Viewport},
		ScissorCount:  1,
		PScissors:     []vk.Rect2D{config.Scissor},
	}
	viewportState.Deref()

	// Rasterizer
	rasterizerCreateInfo := vk.PipelineRasterizationStateCreateInfo{
		SType:                   vk.StructureTypePipelineRasterizationStateCreateInfo,
		DepthClampEnable:        vk.False,
		RasterizerDiscardEnable: vk.False,
		PolygonMode:             vk.PolygonModeLine,
		LineWidth:               1.0,
		FrontFace:               vk.FrontFaceCounterClockwise,
		DepthBiasEnable:         vk.False,
		DepthBiasConstantFactor: 0.0,
		DepthBiasClamp:          0.0,
		DepthBiasSlopeFactor:    0.0,
	}
	if !config.IsWireframe {
		rasterizerCreateInfo.PolygonMode = vk.PolygonModeFill
	}
	switch config.CullMode {
	case metadata.FaceCullModeNone:
		rasterizerCreateInfo.CullMode = vk.CullModeFlags(vk.CullModeNone)
	case metadata.FaceCullModeFront:
		rasterizerCreateInfo.CullMode = vk.CullModeFlags(vk.CullModeFrontBit)
	case metadata.FaceCullModeFrontAndBack:
		rasterizerCreateInfo.CullMode = vk.CullModeFlags(vk.CullModeFrontAndBack)
	default:
		fallthrough
	case metadata.FaceCullModeBack:
		rasterizerCreateInfo.CullMode = vk.CullModeFlags(vk.CullModeBackBit)
	}
	rasterizerCreateInfo.Deref()

	// Multisampling.
	multisamplingCreateInfo := vk.PipelineMultisampleStateCreateInfo{
		SType:                 vk.StructureTypePipelineMultisampleStateCreateInfo,
		SampleShadingEnable:   vk.False,
		RasterizationSamples:  vk.SampleCount1Bit,
		MinSampleShading:      1.0,
		PSampleMask:           nil,
		AlphaToCoverageEnable: vk.False,
		AlphaToOneEnable:      vk.False,
	}
	multisamplingCreateInfo.Deref()

	// Depth and stencil testing.
	depthStencil := vk.PipelineDepthStencilStateCreateInfo{
		SType:             vk.StructureTypePipelineDepthStencilStateCreateInfo,
		DepthTestEnable:   vk.False,
		DepthWriteEnable:  vk.False,
		StencilTestEnable: vk.False,
	}
	if (metadata.ShaderFlags(config.ShaderFlags) & metadata.SHADER_FLAG_DEPTH_TEST) != 0 {
		depthStencil.DepthTestEnable = vk.True
		depthStencil.DepthCompareOp = vk.CompareOpLess
		depthStencil.DepthBoundsTestEnable = vk.False
		depthStencil.StencilTestEnable = vk.False
	}
	if (metadata.ShaderFlags(config.ShaderFlags) & metadata.SHADER_FLAG_DEPTH_WRITE) != 0 {
		depthStencil.DepthWriteEnable = vk.True
	}
	depthStencil.Deref()

	colorBlendAttachmentState := vk.PipelineColorBlendAttachmentState{
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
	colorBlendAttachmentState.Deref()

	colorBlendStateCreateInfo := vk.PipelineColorBlendStateCreateInfo{
		SType:           vk.StructureTypePipelineColorBlendStateCreateInfo,
		LogicOpEnable:   vk.False,
		LogicOp:         vk.LogicOpCopy,
		AttachmentCount: 1,
		PAttachments:    []vk.PipelineColorBlendAttachmentState{colorBlendAttachmentState},
	}
	colorBlendStateCreateInfo.Deref()

	// Dynamic state
	dynamicStates := []vk.DynamicState{
		vk.DynamicStateViewport,
		vk.DynamicStateScissor,
		vk.DynamicStateLineWidth,
	}

	dynamicStateCreateInfo := vk.PipelineDynamicStateCreateInfo{
		SType:             vk.StructureTypePipelineDynamicStateCreateInfo,
		DynamicStateCount: uint32(len(dynamicStates)),
		PDynamicStates:    dynamicStates,
	}
	dynamicStateCreateInfo.Deref()

	// Vertex input
	bindingDescription := vk.VertexInputBindingDescription{
		Binding:   0, // Binding index
		Stride:    config.Stride,
		InputRate: vk.VertexInputRateVertex, // Move to next data entry for each vertex.
	}
	bindingDescription.Deref()

	// Attributes
	vertexInputInfo := vk.PipelineVertexInputStateCreateInfo{
		SType:                           vk.StructureTypePipelineVertexInputStateCreateInfo,
		VertexBindingDescriptionCount:   1,
		PVertexBindingDescriptions:      []vk.VertexInputBindingDescription{bindingDescription},
		VertexAttributeDescriptionCount: uint32(len(config.Attributes)),
		PVertexAttributeDescriptions:    config.Attributes,
	}
	vertexInputInfo.Deref()

	// Input assembly
	inputAssembly := vk.PipelineInputAssemblyStateCreateInfo{
		SType:                  vk.StructureTypePipelineInputAssemblyStateCreateInfo,
		Topology:               vk.PrimitiveTopologyTriangleList,
		PrimitiveRestartEnable: vk.False,
	}
	inputAssembly.Deref()

	// Pipeline layout
	pipelineLayoutCreateInfo := vk.PipelineLayoutCreateInfo{
		SType:                  vk.StructureTypePipelineLayoutCreateInfo,
		SetLayoutCount:         uint32(len(config.DescriptorSetLayouts)),
		PSetLayouts:            config.DescriptorSetLayouts,
		PushConstantRangeCount: 0,
		PPushConstantRanges:    nil,
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
		pipelineLayoutCreateInfo.PushConstantRangeCount = uint32(len(config.PushConstantRanges))
		pipelineLayoutCreateInfo.PPushConstantRanges = ranges
	}
	pipelineLayoutCreateInfo.Deref()

	// Create the pipeline layout.
	var pPipelineLayout vk.PipelineLayout

	if err := lockPool.SafeCall(PipelineManagement, func() error {
		result := vk.CreatePipelineLayout(
			context.Device.LogicalDevice,
			&pipelineLayoutCreateInfo,
			context.Allocator,
			&pPipelineLayout)
		if !VulkanResultIsSuccess(result) {
			err := fmt.Errorf("vkCreatePipelineLayout failed with %s", VulkanResultString(result, true))
			return err
		}
		outPipeline.PipelineLayout = pPipelineLayout
		return nil
	}); err != nil {
		return nil, err
	}

	// Pipeline create
	pipelineCreateInfo := vk.GraphicsPipelineCreateInfo{
		SType:               vk.StructureTypeGraphicsPipelineCreateInfo,
		StageCount:          uint32(len(config.Stages)),
		PStages:             config.Stages,
		PVertexInputState:   &vertexInputInfo,
		PInputAssemblyState: &inputAssembly,
		PViewportState:      &viewportState,
		PRasterizationState: &rasterizerCreateInfo,
		PMultisampleState:   &multisamplingCreateInfo,
		PDepthStencilState:  &depthStencil,
		PColorBlendState:    &colorBlendStateCreateInfo,
		PDynamicState:       &dynamicStateCreateInfo,
		PTessellationState:  nil,
		Layout:              outPipeline.PipelineLayout,
		RenderPass:          config.Renderpass.Handle,
		Subpass:             0,
		BasePipelineHandle:  vk.NullPipeline,
		BasePipelineIndex:   -1,
	}
	pipelineCreateInfo.Deref()

	pPipelines := []vk.Pipeline{outPipeline.Handle}

	if err := lockPool.SafeCall(PipelineManagement, func() error {
		result := vk.CreateGraphicsPipelines(
			context.Device.LogicalDevice,
			vk.NullPipelineCache,
			1,
			[]vk.GraphicsPipelineCreateInfo{pipelineCreateInfo},
			context.Allocator,
			pPipelines)

		if !VulkanResultIsSuccess(result) {
			err := fmt.Errorf("vkCreateGraphicsPipelines failed with %s", VulkanResultString(result, true))
			return err
		}
		return nil
	}); err != nil {
		return nil, err
	}

	if len(pPipelines) <= 0 {
		err := fmt.Errorf("vulkan pipeline handle is nil")
		return nil, err
	}

	outPipeline.Handle = pPipelines[0]

	core.LogDebug("Graphics pipeline created!")
	return outPipeline, nil
}

func (pipeline *VulkanPipeline) Destroy(context *VulkanContext) error {
	// Destroy pipeline
	if pipeline.Handle != nil {
		if err := lockPool.SafeCall(PipelineManagement, func() error {
			vk.DestroyPipeline(context.Device.LogicalDevice, pipeline.Handle, context.Allocator)
			pipeline.Handle = nil
			return nil
		}); err != nil {
			return err
		}
	}
	// Destroy layout
	if pipeline.PipelineLayout != nil {
		if err := lockPool.SafeCall(PipelineManagement, func() error {
			vk.DestroyPipelineLayout(context.Device.LogicalDevice, pipeline.PipelineLayout, context.Allocator)
			pipeline.PipelineLayout = nil
			return nil
		}); err != nil {
			return err
		}
	}
	return nil
}

func (pipeline *VulkanPipeline) Bind(command_buffer *VulkanCommandBuffer, bind_point vk.PipelineBindPoint) error {
	if err := lockPool.SafeCall(CommandBufferManagement, func() error {
		vk.CmdBindPipeline(command_buffer.Handle, bind_point, pipeline.Handle)
		return nil
	}); err != nil {
		return err
	}
	return nil
}
