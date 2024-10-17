package vulkan

import vk "github.com/goki/vulkan"

/**
 * @brief The configuration for a descriptor set.
 */
type VulkanDescriptorSetConfig struct {
	/** @brief The number of bindings in this set. */
	BindingCount uint8
	/** @brief An array of binding layouts for this set. */
	Bindings [VULKAN_SHADER_MAX_BINDINGS]vk.DescriptorSetLayoutBinding
	/** @brief The index of the sampler binding. */
	SamplerBindingIndex uint8
}

/**
 * @brief Represents a state for a given descriptor. This is used
 * to determine when a descriptor needs updating. There is a state
 * per frame (with a max of 3).
 */
type VulkanDescriptorState struct {
	/** @brief The descriptor generation, per frame. */
	Generations [3]uint8
	/** @brief The identifier, per frame. Typically used for texture IDs. */
	IDs [3]uint32
}

/**
 * @brief Represents the state for a descriptor set. This is used to track
 * generations and updates, potentially for optimization via skipping
 * sets which do not need updating.
 */
type VulkanShaderDescriptorSetState struct {
	/** @brief The descriptor sets for this instance, one per frame. */
	DescriptorSets [3]vk.DescriptorSet
	/** @brief A descriptor state per descriptor, which in turn handles frames. Count is managed in shader config. */
	DescriptorStates [VULKAN_SHADER_MAX_BINDINGS]VulkanDescriptorState
}
