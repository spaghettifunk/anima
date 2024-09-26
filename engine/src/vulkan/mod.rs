use anyhow::{anyhow, Ok, Result};
use command_buffer::VulkanCommandBuffer;
use context::VulkanContext;
use device::VulkanDevice;
use framebuffer::VulkanFramebuffer;
use instance::VulkanInstance;
use pipeline::VulkanPipeline;
use swapchain::VulkanSwapchain;
use vulkanalia::{
    loader::{LibloadingLoader, LIBRARY},
    vk::{self, DeviceV1_0, Handle, HasBuilder, KhrSwapchainExtension},
    Entry,
};
use winit::window::Window;

mod command_buffer;
mod constants;
mod context;
mod device;
mod framebuffer;
mod image;
mod instance;
mod pipeline;
mod render_pass;
mod swapchain;

#[derive(Debug)]
pub struct VulkanRenderer {
    instance: VulkanInstance,
    device: VulkanDevice,
    pipeline: VulkanPipeline,
    context: VulkanContext,
}

impl VulkanRenderer {
    pub unsafe fn new(window: &Window) -> Result<VulkanRenderer> {
        let loader = LibloadingLoader::new(LIBRARY)?;
        let entry = Entry::new(loader).map_err(|b| anyhow!("{}", b))?;

        let mut context = VulkanContext::default();
        let instance = VulkanInstance::new(window, &entry, &mut context)?;
        let swapchain = VulkanSwapchain::new(window, &instance, &mut context)?;
        let device = VulkanDevice::new(&entry, &instance, &mut context)?;

        VulkanSwapchain::create(window, &instance, &device, &mut context)?;
        VulkanSwapchain::create_image_views(&device, &mut context)?;

        let pipeline = VulkanPipeline::create(&instance, &device, &mut context)?;
        VulkanFramebuffer::create(&device, &mut context)?;
        VulkanCommandBuffer::create_command_pool(&instance, &device, &mut context)?;
        VulkanCommandBuffer::create_command_buffers(&device, &mut context)?;

        VulkanRenderer::create_sync_objects(&device, &mut context)?;

        Ok(VulkanRenderer {
            instance,
            device,
            pipeline,
            context,
        })
    }

    unsafe fn create_sync_objects(
        device: &VulkanDevice,
        context: &mut VulkanContext,
    ) -> Result<()> {
        let semaphore_info = vk::SemaphoreCreateInfo::builder();

        context.image_available_semaphore =
            device.vk_device.create_semaphore(&semaphore_info, None)?;
        context.render_finished_semaphore =
            device.vk_device.create_semaphore(&semaphore_info, None)?;

        Ok(())
    }

    pub unsafe fn render(&mut self) -> Result<()> {
        let image_index = self
            .device
            .vk_device
            .acquire_next_image_khr(
                self.context.swapchain,
                u64::MAX,
                self.context.image_available_semaphore,
                vk::Fence::null(),
            )?
            .0 as usize;

        let wait_semaphores = &[self.context.image_available_semaphore];
        let wait_stages = &[vk::PipelineStageFlags::COLOR_ATTACHMENT_OUTPUT];
        let command_buffers = &[self.context.command_buffers[image_index as usize]];
        let signal_semaphores = &[self.context.render_finished_semaphore];
        let submit_info = vk::SubmitInfo::builder()
            .wait_semaphores(wait_semaphores)
            .wait_dst_stage_mask(wait_stages)
            .command_buffers(command_buffers)
            .signal_semaphores(signal_semaphores);

        self.device.vk_device.queue_submit(
            self.context.graphics_queue,
            &[submit_info],
            vk::Fence::null(),
        )?;

        Ok(())
    }

    pub unsafe fn destroy(&mut self) {
        self.device
            .vk_device
            .destroy_semaphore(self.context.render_finished_semaphore, None);
        self.device
            .vk_device
            .destroy_semaphore(self.context.image_available_semaphore, None);
        self.pipeline.destroy(&self.device, &mut self.context);
        self.device.destroy(&mut self.context);
        self.instance.destroy(&mut self.context);
    }
}
