use anyhow::{anyhow, Result};
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
    pub instance: VulkanInstance,
    pub device: VulkanDevice,
    pub pipeline: VulkanPipeline,
    context: VulkanContext,
    frame: usize,
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
            frame: 0,
        })
    }

    unsafe fn create_sync_objects(
        device: &VulkanDevice,
        context: &mut VulkanContext,
    ) -> Result<()> {
        let semaphore_info = vk::SemaphoreCreateInfo::builder();
        let fence_info = vk::FenceCreateInfo::builder().flags(vk::FenceCreateFlags::SIGNALED);

        for _ in 0..constants::MAX_FRAMES_IN_FLIGHT {
            context
                .image_available_semaphores
                .push(device.vk_device.create_semaphore(&semaphore_info, None)?);
            context
                .render_finished_semaphores
                .push(device.vk_device.create_semaphore(&semaphore_info, None)?);
            context
                .in_flight_fences
                .push(device.vk_device.create_fence(&fence_info, None)?);
        }

        context.images_in_flight = context
            .swapchain_images
            .iter()
            .map(|_| vk::Fence::null())
            .collect();

        Ok(())
    }

    pub unsafe fn render(&mut self) -> Result<()> {
        self.device.vk_device.wait_for_fences(
            &[self.context.in_flight_fences[self.frame]],
            true,
            u64::MAX,
        )?;

        let image_index = self
            .device
            .vk_device
            .acquire_next_image_khr(
                self.context.swapchain,
                u64::MAX,
                self.context.image_available_semaphores[self.frame],
                vk::Fence::null(),
            )?
            .0 as usize;

        if !self.context.images_in_flight[image_index as usize].is_null() {
            self.device.vk_device.wait_for_fences(
                &[self.context.images_in_flight[image_index as usize]],
                true,
                u64::MAX,
            )?;
        }

        self.context.images_in_flight[image_index as usize] =
            self.context.in_flight_fences[self.frame];

        let wait_semaphores = &[self.context.image_available_semaphores[self.frame]];
        let wait_stages = &[vk::PipelineStageFlags::COLOR_ATTACHMENT_OUTPUT];
        let command_buffers = &[self.context.command_buffers[image_index as usize]];
        let signal_semaphores = &[self.context.render_finished_semaphores[self.frame]];
        let submit_info = vk::SubmitInfo::builder()
            .wait_semaphores(wait_semaphores)
            .wait_dst_stage_mask(wait_stages)
            .command_buffers(command_buffers)
            .signal_semaphores(signal_semaphores);

        self.device
            .vk_device
            .reset_fences(&[self.context.in_flight_fences[self.frame]])?;

        self.device.vk_device.queue_submit(
            self.context.graphics_queue,
            &[submit_info],
            self.context.in_flight_fences[self.frame],
        )?;

        let swapchains = &[self.context.swapchain];
        let image_indices = &[image_index as u32];
        let present_info = vk::PresentInfoKHR::builder()
            .wait_semaphores(signal_semaphores)
            .swapchains(swapchains)
            .image_indices(image_indices);

        self.device
            .vk_device
            .queue_present_khr(self.context.present_queue, &present_info)?;
        self.device
            .vk_device
            .queue_wait_idle(self.context.present_queue)?;

        self.frame = (self.frame + 1) % constants::MAX_FRAMES_IN_FLIGHT;

        Ok(())
    }

    pub unsafe fn destroy(&mut self) {
        self.context
            .in_flight_fences
            .iter()
            .for_each(|f| self.device.vk_device.destroy_fence(*f, None));
        self.context
            .render_finished_semaphores
            .iter()
            .for_each(|s| self.device.vk_device.destroy_semaphore(*s, None));
        self.context
            .image_available_semaphores
            .iter()
            .for_each(|s| self.device.vk_device.destroy_semaphore(*s, None));
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

    pub unsafe fn device_wait_idle(&mut self) {
        self.device.vk_device.device_wait_idle().unwrap();
    }

    // TODO: need to find a way to check if Vulkan is supported
    pub fn supports_vulkan() -> bool {
        true // Vulkan is supported if we have physical devices
    }
}