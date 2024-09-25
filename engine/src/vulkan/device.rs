use anyhow::{anyhow, Ok, Result};
use log::*;
use thiserror::Error;
use vulkanalia::{
    vk::{self, DeviceV1_0, HasBuilder, InstanceV1_0},
    Device, Entry, Instance,
};

use super::{constants, context::VulkanContext, instance::VulkanInstance};

#[derive(Debug)]
pub struct VulkanDevice {
    vk_device: Device,
}

#[derive(Debug, Error)]
#[error("Missing {0}.")]
pub struct SuitabilityError(pub &'static str);

impl VulkanDevice {
    unsafe fn pick_physical_device(
        instance: &VulkanInstance,
        context: &mut VulkanContext,
    ) -> Result<()> {
        for physical_device in instance.vk_instance.enumerate_physical_devices()? {
            let properties = instance
                .vk_instance
                .get_physical_device_properties(physical_device);

            if let Err(error) =
                VulkanDevice::check_physical_device(instance, context, physical_device)
            {
                warn!(
                    "Skipping physical device (`{}`): {}",
                    properties.device_name, error
                );
            } else {
                info!("Selected physical device (`{}`).", properties.device_name);
                context.physical_device = physical_device;
                return Ok(());
            }
        }
        Err(anyhow!("Failed to find suitable physical device."))
    }

    unsafe fn check_physical_device(
        instance: &VulkanInstance,
        context: &VulkanContext,
        physical_device: vk::PhysicalDevice,
    ) -> Result<()> {
        QueueFamilyIndices::get(instance, context, physical_device)?;
        Ok(())
    }

    pub unsafe fn new(
        entry: &Entry,
        instance: &VulkanInstance,
        context: &mut VulkanContext,
    ) -> Result<VulkanDevice> {
        VulkanDevice::pick_physical_device(instance, context)?;

        let indices = QueueFamilyIndices::get(instance, context, context.physical_device)?;

        let queue_priorities = &[1.0];
        let queue_info = vk::DeviceQueueCreateInfo::builder()
            .queue_family_index(indices.graphics)
            .queue_priorities(queue_priorities);

        let layers = if constants::VALIDATION_ENABLED {
            vec![constants::VALIDATION_LAYER.as_ptr()]
        } else {
            vec![]
        };

        let mut extensions = vec![];

        // Required by Vulkan SDK on macOS since 1.3.216.
        if cfg!(target_os = "macos") && entry.version()? >= constants::PORTABILITY_MACOS_VERSION {
            extensions.push(vk::KHR_PORTABILITY_SUBSET_EXTENSION.name.as_ptr());
        }

        let features = vk::PhysicalDeviceFeatures::builder();

        let queue_infos = &[queue_info];
        let info = vk::DeviceCreateInfo::builder()
            .queue_create_infos(queue_infos)
            .enabled_layer_names(&layers)
            .enabled_extension_names(&extensions)
            .enabled_features(&features);

        let device = instance
            .vk_instance
            .create_device(context.physical_device, &info, None)?;

        context.graphics_queue = device.get_device_queue(indices.graphics, 0);

        Ok(VulkanDevice { vk_device: device })
    }

    pub unsafe fn destroy(&mut self) {
        self.vk_device.destroy_device(None);
    }
}

#[derive(Copy, Clone, Debug)]
struct QueueFamilyIndices {
    graphics: u32,
}

impl QueueFamilyIndices {
    unsafe fn get(
        instance: &VulkanInstance,
        context: &VulkanContext,
        physical_device: vk::PhysicalDevice,
    ) -> Result<Self> {
        let properties = instance
            .vk_instance
            .get_physical_device_queue_family_properties(physical_device);

        let graphics = properties
            .iter()
            .position(|p| p.queue_flags.contains(vk::QueueFlags::GRAPHICS))
            .map(|i| i as u32);

        if let Some(graphics) = graphics {
            Ok(Self { graphics })
        } else {
            Err(anyhow!(SuitabilityError(
                "Missing required queue families."
            )))
        }
    }
}
