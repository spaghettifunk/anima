use anyhow::{anyhow, Ok, Result};
use log::*;
use std::collections::HashSet;
use std::ffi::CStr;
use std::os::raw::c_void;
use vulkanalia::prelude::v1_0::*;
use vulkanalia::vk;
use vulkanalia::vk::ExtDebugUtilsExtension;
use vulkanalia::vk::KhrSurfaceExtension;
use vulkanalia::window as vk_window;
use vulkanalia::Entry;
use vulkanalia::Instance;
use winit::window::Window;

use super::constants;
use super::context::VulkanContext;

#[derive(Debug)]
pub struct VulkanInstance {
    pub vk_instance: Instance,
}

impl VulkanInstance {
    pub unsafe fn new(
        window: &Window,
        entry: &Entry,
        context: &mut VulkanContext,
    ) -> Result<VulkanInstance> {
        // Application Info
        let application_info = vk::ApplicationInfo::builder()
            .application_name(b"Alaska Engine\0")
            .application_version(vk::make_version(1, 0, 0))
            .engine_name(b"Alaska\0")
            .engine_version(vk::make_version(1, 0, 0))
            .api_version(vk::make_version(1, 0, 0));

        // Layers
        let available_layers = entry
            .enumerate_instance_layer_properties()?
            .iter()
            .map(|l| l.layer_name)
            .collect::<HashSet<_>>();

        if constants::VALIDATION_ENABLED && !available_layers.contains(&constants::VALIDATION_LAYER)
        {
            return Err(anyhow!("Validation layer requested but not supported."));
        }

        let layers = if constants::VALIDATION_ENABLED {
            vec![constants::VALIDATION_LAYER.as_ptr()]
        } else {
            Vec::new()
        };

        // Extensions
        let mut extensions = vk_window::get_required_instance_extensions(window)
            .iter()
            .map(|e| e.as_ptr())
            .collect::<Vec<_>>();

        // Required by Vulkan SDK on macOS since 1.3.216.
        let flags = if cfg!(target_os = "macos")
            && entry.version()? >= constants::PORTABILITY_MACOS_VERSION
        {
            info!("Enabling extensions for macOS portability.");
            extensions.push(
                vk::KHR_GET_PHYSICAL_DEVICE_PROPERTIES2_EXTENSION
                    .name
                    .as_ptr(),
            );
            extensions.push(vk::KHR_PORTABILITY_ENUMERATION_EXTENSION.name.as_ptr());
            vk::InstanceCreateFlags::ENUMERATE_PORTABILITY_KHR
        } else {
            vk::InstanceCreateFlags::empty()
        };

        if constants::VALIDATION_ENABLED {
            extensions.push(vk::EXT_DEBUG_UTILS_EXTENSION.name.as_ptr());
        }

        // Create
        let mut info = vk::InstanceCreateInfo::builder()
            .application_info(&application_info)
            .enabled_layer_names(&layers)
            .enabled_extension_names(&extensions)
            .flags(flags);

        let mut debug_info = vk::DebugUtilsMessengerCreateInfoEXT::builder()
            .message_severity(vk::DebugUtilsMessageSeverityFlagsEXT::all())
            .message_type(
                vk::DebugUtilsMessageTypeFlagsEXT::GENERAL
                    | vk::DebugUtilsMessageTypeFlagsEXT::VALIDATION
                    | vk::DebugUtilsMessageTypeFlagsEXT::PERFORMANCE,
            )
            .user_callback(Some(debug_callback));

        if constants::VALIDATION_ENABLED {
            info = info.push_next(&mut debug_info);
        }

        let instance = entry.create_instance(&info, None)?;

        // Messenger
        if constants::VALIDATION_ENABLED {
            context.messenger = instance.create_debug_utils_messenger_ext(&debug_info, None)?;
        }

        Ok(VulkanInstance {
            vk_instance: instance,
        })
    }

    pub unsafe fn destroy(&mut self, context: &mut VulkanContext) {
        self.vk_instance.destroy_surface_khr(context.surface, None);
        if constants::VALIDATION_ENABLED {
            self.vk_instance
                .destroy_debug_utils_messenger_ext(context.messenger, None);
        }
        self.vk_instance.destroy_instance(None);
    }
}

extern "system" fn debug_callback(
    severity: vk::DebugUtilsMessageSeverityFlagsEXT,
    type_: vk::DebugUtilsMessageTypeFlagsEXT,
    data: *const vk::DebugUtilsMessengerCallbackDataEXT,
    _: *mut c_void,
) -> vk::Bool32 {
    let data = unsafe { *data };
    let message = unsafe { CStr::from_ptr(data.message) }.to_string_lossy();

    if severity >= vk::DebugUtilsMessageSeverityFlagsEXT::ERROR {
        error!("({:?}) {}", type_, message);
    } else if severity >= vk::DebugUtilsMessageSeverityFlagsEXT::WARNING {
        warn!("({:?}) {}", type_, message);
    } else if severity >= vk::DebugUtilsMessageSeverityFlagsEXT::INFO {
        debug!("({:?}) {}", type_, message);
    } else {
        trace!("({:?}) {}", type_, message);
    }

    vk::FALSE
}
