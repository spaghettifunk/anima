use super::instance::VulkanInstance;
use super::render_pass::{self, VulkanRenderPass};
use super::{context::VulkanContext, device::VulkanDevice};
use anyhow::{Ok, Result};
use vulkanalia::bytecode::Bytecode;
use vulkanalia::vk::{self, DeviceV1_0, Handle, HasBuilder};

#[derive(Debug)]
pub struct VulkanPipeline {
    render_pass: VulkanRenderPass,
}

impl VulkanPipeline {
    pub unsafe fn create(
        instance: &VulkanInstance,
        device: &VulkanDevice,
        context: &mut VulkanContext,
    ) -> Result<VulkanPipeline> {
        // Render pass
        let render_pass = VulkanRenderPass::create(device, context)?;

        let vert = include_bytes!("../../../shaders/vert.spv");
        let frag = include_bytes!("../../../shaders/frag.spv");

        let vertex_shader_module = VulkanPipeline::create_shader_module(device, &vert[..])?;
        let fragment_shader_module = VulkanPipeline::create_shader_module(device, &frag[..])?;

        let vert_stage = vk::PipelineShaderStageCreateInfo::builder()
            .stage(vk::ShaderStageFlags::VERTEX)
            .module(vertex_shader_module)
            .name(b"main\0");

        let frag_stage = vk::PipelineShaderStageCreateInfo::builder()
            .stage(vk::ShaderStageFlags::FRAGMENT)
            .module(fragment_shader_module)
            .name(b"main\0");

        let vertex_input_state = vk::PipelineVertexInputStateCreateInfo::builder();
        let input_assembly_state = vk::PipelineInputAssemblyStateCreateInfo::builder()
            .topology(vk::PrimitiveTopology::TRIANGLE_LIST)
            .primitive_restart_enable(false);

        let viewport = vk::Viewport::builder()
            .x(0.0)
            .y(0.0)
            .width(context.swapchain_extent.width as f32)
            .height(context.swapchain_extent.height as f32)
            .min_depth(0.0)
            .max_depth(1.0);

        let scissor = vk::Rect2D::builder()
            .offset(vk::Offset2D { x: 0, y: 0 })
            .extent(context.swapchain_extent);

        let viewports = &[viewport];
        let scissors = &[scissor];
        let viewport_state = vk::PipelineViewportStateCreateInfo::builder()
            .viewports(viewports)
            .scissors(scissors);

        // rasterizer
        let rasterization_state = vk::PipelineRasterizationStateCreateInfo::builder()
            .depth_clamp_enable(false)
            .rasterizer_discard_enable(false)
            .polygon_mode(vk::PolygonMode::FILL)
            .line_width(1.0)
            .cull_mode(vk::CullModeFlags::BACK)
            .front_face(vk::FrontFace::CLOCKWISE)
            .depth_bias_enable(false);

        // multisampling
        let multisample_state = vk::PipelineMultisampleStateCreateInfo::builder()
            .sample_shading_enable(false)
            .rasterization_samples(vk::SampleCountFlags::_1);

        // color blending
        let attachment = vk::PipelineColorBlendAttachmentState::builder()
            .color_write_mask(vk::ColorComponentFlags::all())
            .blend_enable(true)
            .src_color_blend_factor(vk::BlendFactor::SRC_ALPHA)
            .dst_color_blend_factor(vk::BlendFactor::ONE_MINUS_SRC_ALPHA)
            .color_blend_op(vk::BlendOp::ADD)
            .src_alpha_blend_factor(vk::BlendFactor::ONE)
            .dst_alpha_blend_factor(vk::BlendFactor::ZERO)
            .alpha_blend_op(vk::BlendOp::ADD);

        let attachments = &[attachment];
        let color_blend_state = vk::PipelineColorBlendStateCreateInfo::builder()
            .logic_op_enable(false)
            .logic_op(vk::LogicOp::COPY)
            .attachments(attachments)
            .blend_constants([0.0, 0.0, 0.0, 0.0]);

        // layout
        let layout_info = vk::PipelineLayoutCreateInfo::builder();
        context.pipeline_layout = device
            .vk_device
            .create_pipeline_layout(&layout_info, None)?;

        let stages = &[vert_stage, frag_stage];
        let info = vk::GraphicsPipelineCreateInfo::builder()
            .stages(stages)
            .vertex_input_state(&vertex_input_state)
            .input_assembly_state(&input_assembly_state)
            .viewport_state(&viewport_state)
            .rasterization_state(&rasterization_state)
            .multisample_state(&multisample_state)
            .color_blend_state(&color_blend_state)
            .layout(context.pipeline_layout)
            .render_pass(context.render_pass)
            .subpass(0);

        context.pipeline = device
            .vk_device
            .create_graphics_pipelines(vk::PipelineCache::null(), &[info], None)?
            .0[0];

        // destroy shader modules
        device
            .vk_device
            .destroy_shader_module(vertex_shader_module, None);
        device
            .vk_device
            .destroy_shader_module(fragment_shader_module, None);

        Ok(VulkanPipeline { render_pass })
    }

    unsafe fn create_shader_module(
        device: &VulkanDevice,
        bytecode: &[u8],
    ) -> Result<vk::ShaderModule> {
        let bytecode = Bytecode::new(bytecode).unwrap();
        let info = vk::ShaderModuleCreateInfo::builder()
            .code_size(bytecode.code_size())
            .code(bytecode.code());

        Ok(device.vk_device.create_shader_module(&info, None)?)
    }

    pub unsafe fn destroy(&mut self, device: &VulkanDevice, context: &mut VulkanContext) {
        device
            .vk_device
            .destroy_command_pool(context.command_pool, None);
        context
            .framebuffers
            .iter()
            .for_each(|f| device.vk_device.destroy_framebuffer(*f, None));
        device.vk_device.destroy_pipeline(context.pipeline, None);
        device
            .vk_device
            .destroy_pipeline_layout(context.pipeline_layout, None);
        device
            .vk_device
            .destroy_render_pass(context.render_pass, None);
    }
}
