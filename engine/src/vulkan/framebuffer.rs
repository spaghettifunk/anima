use super::{context::VulkanContext, device::VulkanDevice};
use anyhow::Result;
use vulkanalia::vk::{self, DeviceV1_0, HasBuilder};

pub struct VulkanFramebuffer;

impl VulkanFramebuffer {
    pub unsafe fn create(device: &VulkanDevice, context: &mut VulkanContext) -> Result<()> {
        context.framebuffers = context
            .swapchain_image_views
            .iter()
            .map(|i| {
                let attachments = &[*i];
                let create_info = vk::FramebufferCreateInfo::builder()
                    .render_pass(context.render_pass)
                    .attachments(attachments)
                    .width(context.swapchain_extent.width)
                    .height(context.swapchain_extent.height)
                    .layers(1);

                device.vk_device.create_framebuffer(&create_info, None)
            })
            .collect::<Result<Vec<_>, _>>()?;
        Ok(())
    }
}
