use std::collections::HashSet;

use anyhow::{anyhow, Ok, Result};
use log::*;
use thiserror::Error;
use vulkanalia::{
    vk::{self, DeviceV1_0, HasBuilder, InstanceV1_0, KhrSurfaceExtension, KhrSwapchainExtension},
    Device, Entry,
};

use super::{
    constants, context::VulkanContext, instance::VulkanInstance, swapchain::VulkanSwapchain,
};

#[derive(Debug)]
pub struct VulkanDevice {
    pub vk_device: Device,
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
        VulkanDevice::check_physical_device_extensions(instance, physical_device)?;

        let support = VulkanSwapchain::get(instance, context, physical_device)?;
        if support.formats.is_empty() || support.present_modes.is_empty() {
            return Err(anyhow!(SuitabilityError("Insufficient swapchain support.")));
        }

        Ok(())
    }

    unsafe fn check_physical_device_extensions(
        instance: &VulkanInstance,
        physical_device: vk::PhysicalDevice,
    ) -> Result<()> {
        let extensions = instance
            .vk_instance
            .enumerate_device_extension_properties(physical_device, None)?
            .iter()
            .map(|e| e.extension_name)
            .collect::<HashSet<_>>();

        if constants::DEVICE_EXTENSIONS
            .iter()
            .all(|e| extensions.contains(e))
        {
            Ok(())
        } else {
            Err(anyhow!(SuitabilityError(
                "Missing required device extensions."
            )))
        }
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

        let mut extensions = constants::DEVICE_EXTENSIONS
            .iter()
            .map(|n| n.as_ptr())
            .collect::<Vec<_>>();

        // Required by Vulkan SDK on macOS since 1.3.216.
        if cfg!(target_os = "macos") && entry.version()? >= constants::PORTABILITY_MACOS_VERSION {
            extensions.push(vk::KHR_PORTABILITY_SUBSET_EXTENSION.name.as_ptr());
        }

        let features = vk::PhysicalDeviceFeatures::builder();

        let indices = QueueFamilyIndices::get(instance, context, context.physical_device)?;

        let mut unique_indices = HashSet::new();
        unique_indices.insert(indices.graphics);
        unique_indices.insert(indices.present);

        let queue_priorities = &[1.0];
        let queue_infos = unique_indices
            .iter()
            .map(|i| {
                vk::DeviceQueueCreateInfo::builder()
                    .queue_family_index(*i)
                    .queue_priorities(queue_priorities)
            })
            .collect::<Vec<_>>();

        let info = vk::DeviceCreateInfo::builder()
            .queue_create_infos(&queue_infos)
            .enabled_layer_names(&layers)
            .enabled_extension_names(&extensions)
            .enabled_features(&features);

        let device = instance
            .vk_instance
            .create_device(context.physical_device, &info, None)?;

        context.graphics_queue = device.get_device_queue(indices.graphics, 0);
        context.present_queue = device.get_device_queue(indices.present, 0);

        Ok(VulkanDevice { vk_device: device })
    }

    pub unsafe fn destroy(&mut self, context: &mut VulkanContext) {
        context
            .swapchain_image_views
            .iter()
            .for_each(|v| self.vk_device.destroy_image_view(*v, None));
        self.vk_device
            .destroy_swapchain_khr(context.swapchain, None);
        self.vk_device.destroy_device(None);
    }
}

#[derive(Copy, Clone, Debug)]
pub struct QueueFamilyIndices {
    pub graphics: u32,
    pub present: u32,
}

impl QueueFamilyIndices {
    pub unsafe fn get(
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

        let mut present = None;
        for (index, properties) in properties.iter().enumerate() {
            if instance
                .vk_instance
                .get_physical_device_surface_support_khr(
                    physical_device,
                    index as u32,
                    context.surface,
                )?
            {
                present = Some(index as u32);
                break;
            }
        }

        if let (Some(graphics), Some(present)) = (graphics, present) {
            Ok(Self { graphics, present })
        } else {
            Err(anyhow!(SuitabilityError(
                "Missing required queue families."
            )))
        }
    }
}
