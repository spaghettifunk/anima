package vulkan

import (
	vk "github.com/goki/vulkan"
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
