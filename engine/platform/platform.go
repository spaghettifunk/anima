package platform

import (
	"runtime"

	"github.com/go-gl/glfw/v3.3/glfw"
	"github.com/spaghettifunk/anima/engine/core"
)

var startTime float64 = 0

func init() {
	// GLFW event handling must run on the main OS thread
	runtime.LockOSThread()
}

type Platform struct {
	Window *glfw.Window
}

func New() *Platform {
	return &Platform{
		Window: nil,
	}
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
	p.Window.Destroy()
	glfw.Terminate()
	return nil
}

func (p *Platform) GetAbsoluteTime() float64 {
	return glfw.GetTime()
}

func (p *Platform) PumpMessages() bool {
	glfw.PollEvents()
	return !p.Window.ShouldClose()
}

func (p *Platform) GetRequiredExtensionNames() []string {
	result := []string{}
	extensions := p.Window.GetRequiredInstanceExtensions()
	for i := 0; i < len(extensions); i++ {
		if extensions[i] == "VK_KHR_surface" {
			// We already include "VK_KHR_surface", so skip this.
			continue
		}
		result = append(result, extensions[i])
	}
	return result
}

func keyCallback(w *glfw.Window, key glfw.Key, scancode int, action glfw.Action, mods glfw.ModifierKey) {
	ourKey := translateKey(key)
	if ourKey != core.KEYS_MAX_KEYS {
		pressed := action == glfw.Press || action == glfw.Repeat
		core.InputProcessKey(ourKey, pressed)
	}
}

func mouseButtonCallback(w *glfw.Window, button glfw.MouseButton, action glfw.Action, mods glfw.ModifierKey) {
	mouseButton := core.BUTTON_MAX_BUTTONS

	switch button {
	case glfw.MouseButtonLeft:
		mouseButton = core.BUTTON_LEFT
		break
	case glfw.MouseButtonMiddle:
		mouseButton = core.BUTTON_MIDDLE
		break
	case glfw.MouseButtonRight:
		mouseButton = core.BUTTON_RIGHT
		break
	default:
		mouseButton = core.BUTTON_MAX_BUTTONS
	}

	if mouseButton != core.BUTTON_MAX_BUTTONS {
		pressed := action == glfw.Press
		core.InputProcessButton(mouseButton, pressed)
	}
}

func cursorPosCallback(w *glfw.Window, xpos, ypos float64) {
	core.InputProcessMouseMove(uint16(xpos), uint16(ypos))
}

func scrollCallback(w *glfw.Window, xoff, yoff float64) {
	// We ignore horizontal scroll and also flatten to OS-independent values (-1, +1).
	zDelta := int8(yoff)
	if zDelta != 0 {
		if zDelta < 0 {
			zDelta = -1
		} else {
			zDelta = 1
		}
	}
	if err := core.InputProcessMouseWheel(zDelta); err != nil {
		core.LogError("input process mouse wheel returned an err: %s", err)
	}
}

func framebufferSizeCallback(w *glfw.Window, width, height int) {
	core.EventFire(core.EventContext{
		Type: core.EVENT_CODE_RESIZED,
		Data: &core.SystemEvent{
			WindowWidth:  uint32(width),
			WindowHeight: uint32(height),
		},
	})
}

func translateKey(key glfw.Key) core.KeyCode {
	ourKey := core.KEYS_MAX_KEYS

	switch key {
	case glfw.KeySpace:
		ourKey = core.KEY_SPACE
		break
	case glfw.KeyComma:
		ourKey = core.KEY_COMMA
		break
	case glfw.KeyMinus:
		ourKey = core.KEY_MINUS
		break
	case glfw.KeyPeriod:
		ourKey = core.KEY_PERIOD
		break
	case glfw.KeySlash:
		ourKey = core.KEY_SLASH
		break
	case glfw.Key0:
		ourKey = core.KEY_NUMPAD0
		break
	case glfw.Key1:
		ourKey = core.KEY_NUMPAD1
		break
	case glfw.Key2:
		ourKey = core.KEY_NUMPAD2
		break
	case glfw.Key3:
		ourKey = core.KEY_NUMPAD3
		break
	case glfw.Key4:
		ourKey = core.KEY_NUMPAD4
		break
	case glfw.Key5:
		ourKey = core.KEY_NUMPAD5
		break
	case glfw.Key6:
		ourKey = core.KEY_NUMPAD6
		break
	case glfw.Key7:
		ourKey = core.KEY_NUMPAD7
		break
	case glfw.Key8:
		ourKey = core.KEY_NUMPAD8
		break
	case glfw.Key9:
		ourKey = core.KEY_NUMPAD9
		break
	case glfw.KeySemicolon:
		ourKey = core.KEY_SEMICOLON
		break
	case glfw.KeyEqual:
		ourKey = core.KEY_PLUS
		break
	case glfw.KeyA:
		ourKey = core.KEY_A
		break
	case glfw.KeyB:
		ourKey = core.KEY_B
		break
	case glfw.KeyC:
		ourKey = core.KEY_C
		break
	case glfw.KeyD:
		ourKey = core.KEY_D
		break
	case glfw.KeyE:
		ourKey = core.KEY_E
		break
	case glfw.KeyF:
		ourKey = core.KEY_F
		break
	case glfw.KeyG:
		ourKey = core.KEY_G
		break
	case glfw.KeyH:
		ourKey = core.KEY_H
		break
	case glfw.KeyI:
		ourKey = core.KEY_I
		break
	case glfw.KeyJ:
		ourKey = core.KEY_J
		break
	case glfw.KeyK:
		ourKey = core.KEY_K
		break
	case glfw.KeyL:
		ourKey = core.KEY_L
		break
	case glfw.KeyM:
		ourKey = core.KEY_M
		break
	case glfw.KeyN:
		ourKey = core.KEY_N
		break
	case glfw.KeyO:
		ourKey = core.KEY_O
		break
	case glfw.KeyP:
		ourKey = core.KEY_P
		break
	case glfw.KeyQ:
		ourKey = core.KEY_Q
		break
	case glfw.KeyR:
		ourKey = core.KEY_R
		break
	case glfw.KeyS:
		ourKey = core.KEY_S
		break
	case glfw.KeyT:
		ourKey = core.KEY_T
		break
	case glfw.KeyU:
		ourKey = core.KEY_U
		break
	case glfw.KeyV:
		ourKey = core.KEY_V
		break
	case glfw.KeyW:
		ourKey = core.KEY_W
		break
	case glfw.KeyX:
		ourKey = core.KEY_X
		break
	case glfw.KeyY:
		ourKey = core.KEY_Y
		break
	case glfw.KeyZ:
		ourKey = core.KEY_Z
		break
	case glfw.KeyGraveAccent:
		ourKey = core.KEY_GRAVE
		break
	case glfw.KeyEscape:
		ourKey = core.KEY_ESCAPE
		break
	case glfw.KeyEnter:
		ourKey = core.KEY_ENTER
		break
	case glfw.KeyTab:
		ourKey = core.KEY_TAB
		break
	case glfw.KeyBackspace:
		ourKey = core.KEY_BACKSPACE
		break
	case glfw.KeyInsert:
		ourKey = core.KEY_INSERT
		break
	case glfw.KeyDelete:
		ourKey = core.KEY_DELETE
		break
	case glfw.KeyRight:
		ourKey = core.KEY_RIGHT
		break
	case glfw.KeyLeft:
		ourKey = core.KEY_LEFT
		break
	case glfw.KeyDown:
		ourKey = core.KEY_DOWN
		break
	case glfw.KeyUp:
		ourKey = core.KEY_UP
		break
	case glfw.KeyPageUp:
		ourKey = core.KEY_PRIOR
		break
	case glfw.KeyPageDown:
		ourKey = core.KEY_NEXT
		break
	case glfw.KeyHome:
		ourKey = core.KEY_HOME
		break
	case glfw.KeyEnd:
		ourKey = core.KEY_END
		break
	case glfw.KeyCapsLock:
		ourKey = core.KEY_CAPITAL
		break
	case glfw.KeyScrollLock:
		ourKey = core.KEY_SCROLL
		break
	case glfw.KeyNumLock:
		ourKey = core.KEY_NUMLOCK
		break
	case glfw.KeyPrintScreen:
		ourKey = core.KEY_SNAPSHOT
		break
	case glfw.KeyPause:
		ourKey = core.KEY_PAUSE
		break
	case glfw.KeyF1:
		ourKey = core.KEY_F1
		break
	case glfw.KeyF2:
		ourKey = core.KEY_F2
		break
	case glfw.KeyF3:
		ourKey = core.KEY_F3
		break
	case glfw.KeyF4:
		ourKey = core.KEY_F4
		break
	case glfw.KeyF5:
		ourKey = core.KEY_F5
		break
	case glfw.KeyF6:
		ourKey = core.KEY_F6
		break
	case glfw.KeyF7:
		ourKey = core.KEY_F7
		break
	case glfw.KeyF8:
		ourKey = core.KEY_F8
		break
	case glfw.KeyF9:
		ourKey = core.KEY_F9
		break
	case glfw.KeyF10:
		ourKey = core.KEY_F10
		break
	case glfw.KeyF11:
		ourKey = core.KEY_F11
		break
	case glfw.KeyF12:
		ourKey = core.KEY_F12
		break
	case glfw.KeyF13:
		ourKey = core.KEY_F13
		break
	case glfw.KeyF14:
		ourKey = core.KEY_F14
		break
	case glfw.KeyF15:
		ourKey = core.KEY_F15
		break
	case glfw.KeyF16:
		ourKey = core.KEY_F16
		break
	case glfw.KeyF17:
		ourKey = core.KEY_F17
		break
	case glfw.KeyF18:
		ourKey = core.KEY_F18
		break
	case glfw.KeyF19:
		ourKey = core.KEY_F19
		break
	case glfw.KeyF20:
		ourKey = core.KEY_F20
		break
	case glfw.KeyF21:
		ourKey = core.KEY_F21
		break
	case glfw.KeyF22:
		ourKey = core.KEY_F22
		break
	case glfw.KeyF23:
		ourKey = core.KEY_F23
		break
	case glfw.KeyF24:
		ourKey = core.KEY_F24
		break
	case glfw.KeyKP0:
		ourKey = core.KEY_NUMPAD0
		break
	case glfw.KeyKP1:
		ourKey = core.KEY_NUMPAD1
		break
	case glfw.KeyKP2:
		ourKey = core.KEY_NUMPAD2
		break
	case glfw.KeyKP3:
		ourKey = core.KEY_NUMPAD3
		break
	case glfw.KeyKP4:
		ourKey = core.KEY_NUMPAD4
		break
	case glfw.KeyKP5:
		ourKey = core.KEY_NUMPAD5
		break
	case glfw.KeyKP6:
		ourKey = core.KEY_NUMPAD6
		break
	case glfw.KeyKP7:
		ourKey = core.KEY_NUMPAD7
		break
	case glfw.KeyKP8:
		ourKey = core.KEY_NUMPAD8
		break
	case glfw.KeyKP9:
		ourKey = core.KEY_NUMPAD9
		break
	case glfw.KeyKPDecimal:
		ourKey = core.KEY_DECIMAL
		break
	case glfw.KeyKPDivide:
		ourKey = core.KEY_DIVIDE
		break
	case glfw.KeyKPMultiply:
		ourKey = core.KEY_MULTIPLY
		break
	case glfw.KeyKPSubtract:
		ourKey = core.KEY_SUBTRACT
		break
	case glfw.KeyKPAdd:
		ourKey = core.KEY_ADD
		break
	case glfw.KeyKPEnter:
		ourKey = core.KEY_ENTER
		break
	case glfw.KeyKPEqual:
		ourKey = core.KEY_NUMPAD_EQUAL
		break
	case glfw.KeyLeftShift:
		ourKey = core.KEY_LSHIFT
		break
	case glfw.KeyLeftControl:
		ourKey = core.KEY_LCONTROL
		break
	case glfw.KeyLeftAlt:
		ourKey = core.KEY_LMENU
		break
	case glfw.KeyLeftSuper:
		ourKey = core.KEY_LWIN
		break
	case glfw.KeyRightShift:
		ourKey = core.KEY_RSHIFT
		break
	case glfw.KeyRightControl:
		ourKey = core.KEY_RCONTROL
		break
	case glfw.KeyRightAlt:
		ourKey = core.KEY_RMENU
		break
	case glfw.KeyRightSuper:
		ourKey = core.KEY_RWIN
		break
	default:
		// glfw.KeyUnknown
		// glfw.KeyLast
		// glfw.KeyApostrophe
		// glfw.KeyLeftbracket
		// glfw.KeyBackslash
		// glfw.KeyRightbracket
		// glfw.KeyF25
		// glfw.KeyWorld1
		// glfw.KeyWorld2
		// glfw.KeyMenu
		ourKey = core.KEYS_MAX_KEYS
	}

	return ourKey
}
