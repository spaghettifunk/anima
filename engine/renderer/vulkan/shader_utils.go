package vulkan

/**
 * @brief Represents a single shader stage.
 */
 type  vulkan_shader_stage struct {
    /** @brief The shader module creation info. */
    create_info vk.ShaderModuleCreateInfo
    /** @brief The internal shader module handle. */
    handle vk.ShaderModule
    /** @brief The pipeline shader stage creation info. */
    shader_stage_create_info vk.PipelineShaderStageCreateInfo
}

func create_shader_module(context *VulkanContext,name string,type_str string,shader_stage_flag vk.ShaderStageFlagBits,stage_index uint32) (*shader_stages, error) {

	shader_stages := &shader_stages{}
    // Build file name, which will also be used as the resource name..
    var file_name string
    file_name = fmt.Sprintf("shaders/%s.%s.spv", name, type_str)

    // Read the resource.
    binary_resource Resource;
    if (!resource_system_load(file_name, RESOURCE_TYPE_BINARY, 0, &binary_resource)) {
        KERROR("Unable to read shader module: %s.", file_name)
        return false;
    }

    // kzero_memory(&shader_stages[stage_index].create_info, sizeof(VkShaderModuleCreateInfo));
    shader_stages[stage_index].create_info.sType = VK_STRUCTURE_TYPE_SHADER_MODULE_CREATE_INFO
    // Use the resource's size and data directly.
    shader_stages[stage_index].create_info.codeSize = binary_resource.data_size
    shader_stages[stage_index].create_info.pCode = uint32(binary_resource.data)

    if res := vk.CreateShaderModule(
        context.Device.LogicalDevice,
        &shader_stages[stage_index].create_info,
        context.Allocator,
        &shader_stages[stage_index].handle); res != vk.Success {
			return nil, nil
		}

    // Release the resource.
    resource_system_unload(&binary_resource);

    // Shader stage info    
    shader_stages[stage_index].shader_stage_create_info.sType = VK_STRUCTURE_TYPE_PIPELINE_SHADER_STAGE_CREATE_INFO
    shader_stages[stage_index].shader_stage_create_info.stage = shader_stage_flag
    shader_stages[stage_index].shader_stage_create_info.module = shader_stages[stage_index].handle
    shader_stages[stage_index].shader_stage_create_info.pName = "main"

    return true;
}