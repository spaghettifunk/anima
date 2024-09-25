use anyhow::{anyhow, Ok, Result};
use context::VulkanContext;
use device::VulkanDevice;
use instance::VulkanInstance;
use vulkanalia::{
    loader::{LibloadingLoader, LIBRARY},
    Entry,
};
use winit::window::Window;

mod constants;
mod context;
mod device;
mod instance;

#[derive(Debug)]
pub struct VulkanRenderer {
    instance: VulkanInstance,
    device: VulkanDevice,
    data: VulkanContext,
}

impl VulkanRenderer {
    pub unsafe fn new(window: &Window) -> Result<VulkanRenderer> {
        let loader = LibloadingLoader::new(LIBRARY)?;
        let entry = Entry::new(loader).map_err(|b| anyhow!("{}", b))?;

        let mut data = VulkanContext::default();
        let instance = VulkanInstance::new(window, &entry, &mut data)?;
        let device = VulkanDevice::new(&entry, &instance, &mut data)?;

        Ok(VulkanRenderer {
            instance,
            device,
            data,
        })
    }

    pub unsafe fn destroy(&mut self) {
        self.instance.destroy_instance(&mut self.data);
    }
}
