#![allow(
    dead_code,
    unused_variables,
    clippy::too_many_arguments,
    clippy::unnecessary_wraps
)]

use anyhow::{Ok, Result};
use renderer::Renderer;
use winit::dpi::LogicalSize;
use winit::event::{Event, WindowEvent};
use winit::event_loop::EventLoop;
use winit::window::{Window, WindowBuilder};

mod renderer;
mod vulkan;

#[derive(Debug)]
pub struct Engine {
    window: Window,
    renderer: Renderer,
    event_loop: EventLoop<()>,
}

impl Engine {
    pub fn new() -> Result<Engine> {
        // Window
        let event_loop = EventLoop::new()?;
        let window = WindowBuilder::new()
            .with_title("Alaska Engine")
            .with_inner_size(LogicalSize::new(1024, 768))
            .build(&event_loop)?;

        let renderer = unsafe { 
            Renderer::create(&window)? 
        };

        return Ok(Engine{
            window,
            renderer,
            event_loop,
        })
    } 

    pub fn run(mut self) -> Result<()> {                
        self.event_loop.run(move |event, elwt| {
            match event {
                // Request a redraw when all events were processed.
                Event::AboutToWait => self.window.request_redraw(),
                Event::WindowEvent { event, .. } => match event {
                    // Render a frame if our Vulkan app is not being destroyed.
                    WindowEvent::RedrawRequested if !elwt.exiting() => unsafe { 
                        self.renderer.render(&self.window) 
                    }.unwrap(),
                    // Destroy our Vulkan app.
                    WindowEvent::CloseRequested => {
                        elwt.exit();
                        unsafe { 
                            self.renderer.destroy(); 
                        }
                    }
                    _ => {}
                }
                _ => {}
            }
        })?;    

        Ok(())
    }
}