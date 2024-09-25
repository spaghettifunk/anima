use anyhow::{anyhow, Ok, Result};
use context::VulkanContext;
use device::VulkanDevice;
use instance::VulkanInstance;
use swapchain::VulkanSwapchain;
use vulkanalia::{
    loader::{LibloadingLoader, LIBRARY},
    Entry,
};
use winit::window::Window;

mod constants;
mod context;
mod device;
mod instance;
mod swapchain;

#[derive(Debug)]
pub struct VulkanRenderer {
    instance: VulkanInstance,
    device: VulkanDevice,
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
        VulkanSwapchain::create_swapchain(window, &instance, &device, &mut context)?;
        VulkanSwapchain::create_swapchain_image_views(&device, &mut context)?;

        Ok(VulkanRenderer {
            instance,
            device,
            context,
        })
    }

    pub unsafe fn destroy(&mut self) {
        self.device.destroy(&mut self.context);
        self.instance.destroy(&mut self.context);
    }
}
