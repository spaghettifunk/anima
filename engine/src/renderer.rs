use anyhow::Result;
use winit::window::Window;

use crate::vulkan::VulkanRenderer;

const VALIDATION_ENABLED: bool = cfg!(debug_assertions);

#[derive(Debug)]
pub struct Renderer {
    pub vk_renderer: VulkanRenderer,
}

impl Renderer {
    /// Creates our Vulkan app.
    pub unsafe fn create(window: &Window) -> Result<Self> {
        let vk_renderer = VulkanRenderer::new(window)?;

        Ok(Self { vk_renderer })
    }

    /// Renders a frame for our Vulkan app.
    pub unsafe fn render(&mut self, window: &Window) -> Result<()> {
        self.vk_renderer.render()?;
        Ok(())
    }

    /// Destroys our Vulkan app.
    pub unsafe fn destroy(&mut self) {
        self.vk_renderer.destroy();
    }
}
