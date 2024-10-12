package vulkan

import (
	vk "github.com/goki/vulkan"
	"github.com/spaghettifunk/anima/engine/resources"
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

func NewShaderModule(context *VulkanContext, name string, typeStr string, shaderStageFlag vk.ShaderStageFlagBits, stageIndex uint32) ([]VulkanShaderStage, error) {

	shaderStages := make([]VulkanShaderStage, 1)
	// Build file name, which will also be used as the resource name..
	// fileName := fmt.Sprintf("shaders/%s.%s.spv", name, typeStr)

	// Read the resource.
	var binaryResource resources.Resource
	// if !resource_system_load(fileName, resources.ResourceTypeBinary, 0, &binaryResource) {
	// 	err := fmt.Errorf("unable to read shader module: %s", fileName)
	// 	core.LogError(err.Error())
	// 	return nil, err
	// }

	// kzero_memory(&shader_stages[stage_index].create_info, sizeof(VkShaderModuleCreateInfo));
	shaderStages[stageIndex].CreateInfo.SType = vk.StructureTypeShaderModuleCreateInfo
	// Use the resource's size and data directly.
	shaderStages[stageIndex].CreateInfo.CodeSize = binaryResource.DataSize
	shaderStages[stageIndex].CreateInfo.PCode = []uint32{binaryResource.Data.(uint32)}

	if res := vk.CreateShaderModule(
		context.Device.LogicalDevice,
		&shaderStages[stageIndex].CreateInfo,
		context.Allocator,
		&shaderStages[stageIndex].Handle); res != vk.Success {
		return nil, nil
	}

	// Release the resource.
	// resource_system_unload(&binaryResource)

	// Shader stage info
	shaderStages[stageIndex].ShaderStageCreateInfo.SType = vk.StructureTypePipelineShaderStageCreateInfo
	shaderStages[stageIndex].ShaderStageCreateInfo.Stage = shaderStageFlag
	shaderStages[stageIndex].ShaderStageCreateInfo.Module = shaderStages[stageIndex].Handle
	shaderStages[stageIndex].ShaderStageCreateInfo.PName = "main"

	return shaderStages, nil
}
