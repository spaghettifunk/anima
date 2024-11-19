package vulkan

import (
	vk "github.com/goki/vulkan"
	"github.com/spaghettifunk/anima/engine/renderer/metadata"
)

/**
 * @brief Put some hard limits in place for the count of supported textures,
 * attributes, uniforms, etc. This is to maintain memory locality and avoid
 * dynamic allocations.
 */
/** @brief The maximum number of stages (such as vertex, fragment, compute, etc.) allowed. */
const (
	VULKAN_SHADER_MAX_STAGES uint32 = 8
	/** @brief The maximum number of textures allowed at the global level. */
	VULKAN_SHADER_MAX_GLOBAL_TEXTURES uint32 = 31
	/** @brief The maximum number of textures allowed at the instance level. */
	VULKAN_SHADER_MAX_INSTANCE_TEXTURES uint32 = 31
	/** @brief The maximum number of vertex input attributes allowed. */
	VULKAN_SHADER_MAX_ATTRIBUTES uint32 = 16
	/**
	 * @brief The maximum number of uniforms and samplers allowed at the
	 * global, instance and local levels combined. It's probably more than
	 * will ever be needed.
	 */
	VULKAN_SHADER_MAX_UNIFORMS uint32 = 128
	/** @brief The maximum number of bindings per descriptor set. */
	VULKAN_SHADER_MAX_BINDINGS uint32 = 2
	/** @brief The maximum number of push constant ranges for a shader. */
	VULKAN_SHADER_MAX_PUSH_CONST_RANGES uint32 = 32
)

/**
 * @brief Represents a single shader stage.
 */
type VulkanShaderStage struct {
	/** @brief The shader module creation info. */
	CreateInfo vk.ShaderModuleCreateInfo
	/** @brief The internal shader module Handle. */
	Handle vk.ShaderModule
	/** @brief The pipeline shader stage creation info. */
	ShaderStageCreateInfo vk.PipelineShaderStageCreateInfo
}

/**
 * @brief Configuration for a shader stage, such as vertex or fragment.
 */
type VulkanShaderStageConfig struct {
	/** @brief The shader Stage bit flag. */
	Stage vk.ShaderStageFlagBits
	/** @brief The shader file name. */
	FileName string
}

/** @brief Internal shader configuration generated by vulkan_shader_create(). */
type VulkanShaderConfig struct {
	/** @brief  The configuration for every stage of this shader. */
	Stages []VulkanShaderStageConfig
	/** @brief An array of descriptor pool sizes. */
	PoolSizes []vk.DescriptorPoolSize
	/**
	 * @brief The max number of descriptor sets that can be allocated from this shader.
	 * Should typically be a decently high number.
	 */
	MaxDescriptorSetCount uint16
	/** @brief Descriptor sets, max of 2. Index 0=global, 1=instance */
	DescriptorSets []*VulkanDescriptorSetConfig
	/** @brief An array of attribute descriptions for this shader. */
	Attributes []vk.VertexInputAttributeDescription
	/** @brief Face culling mode, provided by the front end. */
	CullMode metadata.FaceCullMode
}

/**
 * @brief The instance-level state for a shader.
 */
type VulkanShaderInstanceState struct {
	/** @brief The instance ID. INVALID_ID if not used. */
	ID uint32
	/** @brief The Offset in bytes in the instance uniform buffer. */
	Offset uint64
	/** @brief  A state for the descriptor set. */
	DescriptorSetState VulkanShaderDescriptorSetState
	/**
	 * @brief Instance texture map pointers, which are used during rendering. These
	 * are set by calls to set_sampler.
	 */
	InstanceTextureMaps []*metadata.TextureMap
}

/**
 * @brief Represents a generic Vulkan shader. This uses a set of inputs
 * and parameters, as well as the shader programs contained in SPIR-V
 * files to construct a shader for use in rendering.
 */
type VulkanShader struct {
	/** @brief The block of memory mapped to the uniform buffer. */
	MappedUniformBufferBlock interface{}

	/** @brief The shader identifier. */
	ID uint32

	/** @brief The configuration of the shader generated by vulkan_create_shader(). */
	Config *VulkanShaderConfig

	/** @brief A pointer to the Renderpass to be used with this shader. */
	Renderpass *VulkanRenderPass

	/** @brief An array of Stages (such as vertex and fragment) for this shader. Count is located in config.*/
	Stages []*VulkanShaderStage

	/** @brief The descriptor pool used for this shader. */
	DescriptorPool vk.DescriptorPool

	/** @brief Descriptor set layouts, max of 2. Index 0=global, 1=instance. */
	DescriptorSetLayouts []vk.DescriptorSetLayout
	/** @brief Global descriptor sets, one per frame. */
	GlobalDescriptorSets []vk.DescriptorSet
	/** @brief The uniform buffer used by this shader. */
	UniformBuffer *metadata.RenderBuffer

	/** @brief The Pipeline associated with this shader. */
	Pipeline *VulkanPipeline

	/** @brief The instance states for all instances. @todo TODO: make dynamic */
	InstanceCount  uint32
	InstanceStates []*VulkanShaderInstanceState

	/** @brief The number of global non-sampler uniforms. */
	GlobalUniformCount uint8
	/** @brief The number of global sampler uniforms. */
	GlobalUniformSamplerCount uint8
	/** @brief The number of instance non-sampler uniforms. */
	InstanceUniformCount uint8
	/** @brief The number of instance sampler uniforms. */
	InstanceUniformSamplerCount uint8
	/** @brief The number of local non-sampler uniforms. */
	LocalUniformCount uint8
}
