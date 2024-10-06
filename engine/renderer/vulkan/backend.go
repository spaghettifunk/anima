package vulkan

import (
	"fmt"

	"github.com/go-gl/glfw/v3.3/glfw"
	vk "github.com/goki/vulkan"
	"github.com/spaghettifunk/alaska-engine/engine/core"
	"github.com/spaghettifunk/alaska-engine/engine/platform"
)

type VulkanRenderer struct {
	platform    *platform.Platform
	FrameNumber uint64
}

func New(p *platform.Platform) *VulkanRenderer {
	return &VulkanRenderer{
		platform:    p,
		FrameNumber: 0,
	}
}

func (vr VulkanRenderer) Initialize(appName string) error {
	procAddr := glfw.GetVulkanGetInstanceProcAddress()
	if procAddr == nil {
		core.LogFatal("GetInstanceProcAddress is nil")
		return fmt.Errorf("GetInstanceProcAddress is nil")
	}
	vk.SetGetInstanceProcAddr(procAddr)

	if err := vk.Init(); err != nil {
		core.LogFatal("failed to initialize vk: %s", err)
		return err
	}

	return nil
}

func (vr VulkanRenderer) Shutdow() error { return nil }

func (vr VulkanRenderer) Resized(width, height uint16) error { return nil }

func (vr VulkanRenderer) BeginFrame(deltaTime float64) error { return nil }

func (vr VulkanRenderer) EndFrame(deltaTime float64) error { return nil }

func (vr VulkanRenderer) createCommandBuffers() {}

func (vr VulkanRenderer) regenerateFramebuffers(swapchain *VulkanSwapchain, renderpass *VulkanRenderpass) {
}

func (vr VulkanRenderer) recreateSwapchain() bool { return true }
