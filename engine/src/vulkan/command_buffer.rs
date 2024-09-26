use super::{
    context::VulkanContext,
    device::{QueueFamilyIndices, VulkanDevice},
    instance::VulkanInstance,
};
use anyhow::Result;
use vulkanalia::vk::{self, DeviceV1_0, HasBuilder};

#[derive(Debug)]
pub struct VulkanCommandBuffer;

impl VulkanCommandBuffer {
    pub unsafe fn create_command_pool(
        instance: &VulkanInstance,
        device: &VulkanDevice,
        context: &mut VulkanContext,
    ) -> Result<()> {
        let indices = QueueFamilyIndices::get(instance, context, context.physical_device)?;

        let info = vk::CommandPoolCreateInfo::builder()
            .flags(vk::CommandPoolCreateFlags::empty())
            .queue_family_index(indices.graphics);

        context.command_pool = device.vk_device.create_command_pool(&info, None)?;

        Ok(())
    }

    pub unsafe fn create_command_buffers(
        device: &VulkanDevice,
        context: &mut VulkanContext,
    ) -> Result<()> {
        let allocate_info = vk::CommandBufferAllocateInfo::builder()
            .command_pool(context.command_pool)
            .level(vk::CommandBufferLevel::PRIMARY)
            .command_buffer_count(context.framebuffers.len() as u32);

        context.command_buffers = device.vk_device.allocate_command_buffers(&allocate_info)?;

        for (i, command_buffer) in context.command_buffers.iter().enumerate() {
            let info = vk::CommandBufferBeginInfo::builder();

            device
                .vk_device
                .begin_command_buffer(*command_buffer, &info)?;

            let render_area = vk::Rect2D::builder()
                .offset(vk::Offset2D::default())
                .extent(context.swapchain_extent);

            let color_clear_value = vk::ClearValue {
                color: vk::ClearColorValue {
                    float32: [0.0, 0.0, 0.0, 1.0],
                },
            };

            let clear_values = &[color_clear_value];
            let info = vk::RenderPassBeginInfo::builder()
                .render_pass(context.render_pass)
                .framebuffer(context.framebuffers[i])
                .render_area(render_area)
                .clear_values(clear_values);

            device.vk_device.cmd_begin_render_pass(
                *command_buffer,
                &info,
                vk::SubpassContents::INLINE,
            );

            device.vk_device.cmd_bind_pipeline(
                *command_buffer,
                vk::PipelineBindPoint::GRAPHICS,
                context.pipeline,
            );

            device.vk_device.cmd_draw(*command_buffer, 3, 1, 0, 0);
            device.vk_device.cmd_end_render_pass(*command_buffer);

            device.vk_device.end_command_buffer(*command_buffer)?;
        }

        Ok(())
    }
}
