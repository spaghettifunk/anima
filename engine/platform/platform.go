package platform

import (
	"runtime"

	"github.com/go-gl/glfw/v3.3/glfw"
	"github.com/spaghettifunk/alaska-engine/engine/core"
)

var startTime float64 = 0

func init() {
	// GLFW event handling must run on the main OS thread
	runtime.LockOSThread()
}

type Platform struct {
	Window *glfw.Window
}

func New() (*Platform, error) {
	return &Platform{
		Window: nil,
	}, nil
}

func (p *Platform) Startup(applicationName string, x uint32, y uint32, width uint32, height uint32) error {
	if err := glfw.Init(); err != nil {
		core.LogFatal("failed to initialize glfw: %s", err)
		return err
	}

	glfw.WindowHint(glfw.Visible, glfw.False)
	glfw.WindowHint(glfw.Resizable, glfw.True)
	glfw.WindowHint(glfw.ClientAPI, glfw.NoAPI) // Required for Vulkan.

	window, err := glfw.CreateWindow(int(width), int(height), applicationName, nil, nil)
	if err != nil {
		core.LogFatal("failed to create window: %s", err)
		return err
	}
	// window.MakeContextCurrent()
	p.Window = window

	p.Window.SetKeyCallback(keyCallback)
	p.Window.SetMouseButtonCallback(mouseButtonCallback)
	p.Window.SetCursorPosCallback(cursorPosCallback)
	p.Window.SetScrollCallback(scrollCallback)
	p.Window.SetFramebufferSizeCallback(framebufferSizeCallback)
	p.Window.SetPos(int(x), int(y))
	p.Window.Show()

	startTime = glfw.GetTime()

	return nil
}

func (p *Platform) Shutdown() error {
	glfw.Terminate()
	return nil
}

func (p *Platform) PumpMessages() {}

func keyCallback(w *glfw.Window, key glfw.Key, scancode int, action glfw.Action, mods glfw.ModifierKey) {

}

func mouseButtonCallback(w *glfw.Window, button glfw.MouseButton, action glfw.Action, mods glfw.ModifierKey) {
}

func cursorPosCallback(w *glfw.Window, xpos, ypos float64) {}

func scrollCallback(w *glfw.Window, xoff, yoff float64) {}

func framebufferSizeCallback(w *glfw.Window, width, height int) {}
