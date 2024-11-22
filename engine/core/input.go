package core

import "sync"

type Button uint16

const (
	BUTTON_LEFT Button = iota
	BUTTON_RIGHT
	BUTTON_MIDDLE
	BUTTON_MAX_BUTTONS
)

// Key code definitions
type KeyCode uint16

const (
	KEY_BACKSPACE    KeyCode = 0x08
	KEY_ENTER        KeyCode = 0x0D
	KEY_TAB          KeyCode = 0x09
	KEY_SHIFT        KeyCode = 0x10
	KEY_PAUSE        KeyCode = 0x13
	KEY_CAPITAL      KeyCode = 0x14
	KEY_ESCAPE       KeyCode = 0x1B
	KEY_CONVERT      KeyCode = 0x1C
	KEY_NONCONVERT   KeyCode = 0x1D
	KEY_ACCEPT       KeyCode = 0x1E
	KEY_MODECHANGE   KeyCode = 0x1F
	KEY_SPACE        KeyCode = 0x20
	KEY_PRIOR        KeyCode = 0x21
	KEY_NEXT         KeyCode = 0x22
	KEY_END          KeyCode = 0x23
	KEY_HOME         KeyCode = 0x24
	KEY_LEFT         KeyCode = 0x25
	KEY_UP           KeyCode = 0x26
	KEY_RIGHT        KeyCode = 0x27
	KEY_DOWN         KeyCode = 0x28
	KEY_SELECT       KeyCode = 0x29
	KEY_PRINT        KeyCode = 0x2A
	KEY_EXECUTE      KeyCode = 0x2B
	KEY_SNAPSHOT     KeyCode = 0x2C
	KEY_INSERT       KeyCode = 0x2D
	KEY_DELETE       KeyCode = 0x2E
	KEY_HELP         KeyCode = 0x2F
	KEY_A            KeyCode = 0x41
	KEY_B            KeyCode = 0x42
	KEY_C            KeyCode = 0x43
	KEY_D            KeyCode = 0x44
	KEY_E            KeyCode = 0x45
	KEY_F            KeyCode = 0x46
	KEY_G            KeyCode = 0x47
	KEY_H            KeyCode = 0x48
	KEY_I            KeyCode = 0x49
	KEY_J            KeyCode = 0x4A
	KEY_K            KeyCode = 0x4B
	KEY_L            KeyCode = 0x4C
	KEY_M            KeyCode = 0x4D
	KEY_N            KeyCode = 0x4E
	KEY_O            KeyCode = 0x4F
	KEY_P            KeyCode = 0x50
	KEY_Q            KeyCode = 0x51
	KEY_R            KeyCode = 0x52
	KEY_S            KeyCode = 0x53
	KEY_T            KeyCode = 0x54
	KEY_U            KeyCode = 0x55
	KEY_V            KeyCode = 0x56
	KEY_W            KeyCode = 0x57
	KEY_X            KeyCode = 0x58
	KEY_Y            KeyCode = 0x59
	KEY_Z            KeyCode = 0x5A
	KEY_LWIN         KeyCode = 0x5B
	KEY_RWIN         KeyCode = 0x5C
	KEY_APPS         KeyCode = 0x5D
	KEY_SLEEP        KeyCode = 0x5F
	KEY_NUMPAD0      KeyCode = 0x60
	KEY_NUMPAD1      KeyCode = 0x61
	KEY_NUMPAD2      KeyCode = 0x62
	KEY_NUMPAD3      KeyCode = 0x63
	KEY_NUMPAD4      KeyCode = 0x64
	KEY_NUMPAD5      KeyCode = 0x65
	KEY_NUMPAD6      KeyCode = 0x66
	KEY_NUMPAD7      KeyCode = 0x67
	KEY_NUMPAD8      KeyCode = 0x68
	KEY_NUMPAD9      KeyCode = 0x69
	KEY_MULTIPLY     KeyCode = 0x6A
	KEY_ADD          KeyCode = 0x6B
	KEY_SEPARATOR    KeyCode = 0x6C
	KEY_SUBTRACT     KeyCode = 0x6D
	KEY_DECIMAL      KeyCode = 0x6E
	KEY_DIVIDE       KeyCode = 0x6F
	KEY_F1           KeyCode = 0x70
	KEY_F2           KeyCode = 0x71
	KEY_F3           KeyCode = 0x72
	KEY_F4           KeyCode = 0x73
	KEY_F5           KeyCode = 0x74
	KEY_F6           KeyCode = 0x75
	KEY_F7           KeyCode = 0x76
	KEY_F8           KeyCode = 0x77
	KEY_F9           KeyCode = 0x78
	KEY_F10          KeyCode = 0x79
	KEY_F11          KeyCode = 0x7A
	KEY_F12          KeyCode = 0x7B
	KEY_F13          KeyCode = 0x7C
	KEY_F14          KeyCode = 0x7D
	KEY_F15          KeyCode = 0x7E
	KEY_F16          KeyCode = 0x7F
	KEY_F17          KeyCode = 0x80
	KEY_F18          KeyCode = 0x81
	KEY_F19          KeyCode = 0x82
	KEY_F20          KeyCode = 0x83
	KEY_F21          KeyCode = 0x84
	KEY_F22          KeyCode = 0x85
	KEY_F23          KeyCode = 0x86
	KEY_F24          KeyCode = 0x87
	KEY_NUMLOCK      KeyCode = 0x90
	KEY_SCROLL       KeyCode = 0x91
	KEY_NUMPAD_EQUAL KeyCode = 0x92
	KEY_LSHIFT       KeyCode = 0xA0
	KEY_RSHIFT       KeyCode = 0xA1
	KEY_LCONTROL     KeyCode = 0xA2
	KEY_RCONTROL     KeyCode = 0xA3
	KEY_LMENU        KeyCode = 0xA4
	KEY_RMENU        KeyCode = 0xA5
	KEY_SEMICOLON    KeyCode = 0xBA
	KEY_PLUS         KeyCode = 0xBB
	KEY_COMMA        KeyCode = 0xBC
	KEY_MINUS        KeyCode = 0xBD
	KEY_PERIOD       KeyCode = 0xBE
	KEY_SLASH        KeyCode = 0xBF
	KEY_GRAVE        KeyCode = 0xC0
	KEYS_MAX_KEYS
)

// Mouse state structure
type MouseState struct {
	X       uint16
	Y       uint16
	Buttons [BUTTON_MAX_BUTTONS]bool // button states (pressed/released)
}

// Keyboard state structure
type KeyboardState struct {
	Keys [256]bool
}

// Input state structure that holds current and previous states for keyboard and mouse
type InputState struct {
	KeyboardCurrent  KeyboardState
	KeyboardPrevious KeyboardState
	MouseCurrent     MouseState
	MousePrevious    MouseState
}

var onceInput sync.Once
var inputInitialized bool = false
var inputState *InputState = nil

func InputInitialize() error {
	onceInput.Do(func() {
		inputState = &InputState{}
		inputInitialized = true
	})
	LogInfo("Input subsystem initialized.")
	return nil
}

func InputShutdown() error {
	// TODO: Add shutdown routines when needed.
	inputInitialized = false
	return nil
}

func InputUpdate(deltaTime float64) error {
	if !inputInitialized {
		return nil
	}

	// Copy current states to previous states.
	inputState.KeyboardPrevious = inputState.KeyboardCurrent
	inputState.MousePrevious = inputState.MouseCurrent

	return nil
}

// keyboard input
func InputIsKeyDown(key KeyCode) bool {
	if !inputInitialized {
		return false
	}
	return inputState.KeyboardCurrent.Keys[key]
}

func InputIsKeyUp(key KeyCode) bool {
	if !inputInitialized {
		return false
	}
	return !inputState.KeyboardCurrent.Keys[key]
}

func InputWasKeyDown(key KeyCode) bool {
	if !inputInitialized {
		return false
	}
	return inputState.KeyboardPrevious.Keys[key]
}

func InputWasKeyUp(key KeyCode) bool {
	if !inputInitialized {
		return false
	}
	return !inputState.KeyboardPrevious.Keys[key]
}

func InputProcessKey(key KeyCode, pressed bool) error {
	// Only handle this if the state actually changed.
	if inputState.KeyboardCurrent.Keys[key] != pressed {
		// Update internal state.
		inputState.KeyboardCurrent.Keys[key] = pressed

		var code EventCode = 0
		if pressed {
			code = EVENT_CODE_KEY_PRESSED
		} else {
			code = EVENT_CODE_KEY_RELEASED
		}

		// Fire off an event for immediate processing.
		EventFire(EventContext{
			Type: code,
			Data: &KeyEvent{
				KeyCode: key,
			},
		})
	}
	return nil
}

// mouse input
func InputIsButtonDown(button Button) bool {
	if !inputInitialized {
		return false
	}
	return inputState.MouseCurrent.Buttons[button]
}

func InputIsButtonUp(button Button) bool {
	if !inputInitialized {
		return false
	}
	return !inputState.MouseCurrent.Buttons[button]
}

func InputWasButtonDown(button Button) bool {
	if !inputInitialized {
		return false
	}
	return inputState.MousePrevious.Buttons[button]
}

func InputWasButtonUp(button Button) bool {
	if !inputInitialized {
		return false
	}
	return !inputState.MousePrevious.Buttons[button]
}

func InputGetMousePosition() (int32, int32) {
	if !inputInitialized {
		return 0, 0
	}
	return int32(inputState.MouseCurrent.X), int32(inputState.MouseCurrent.Y)
}

func InputGetPreviousMousePosition() (int32, int32) {
	if !inputInitialized {
		return 0, 0
	}
	return int32(inputState.MousePrevious.X), int32(inputState.MousePrevious.Y)
}

func InputProcessButton(button Button, pressed bool) error {
	// If the state changed, fire an event.
	if inputState.MouseCurrent.Buttons[button] != pressed {
		inputState.MouseCurrent.Buttons[button] = pressed

		// Fire the event.
		var code EventCode = 0
		if pressed {
			code = EVENT_CODE_BUTTON_PRESSED
		} else {
			code = EVENT_CODE_BUTTON_RELEASED
		}
		EventFire(EventContext{
			Type: code,
			Data: &MouseEvent{
				Button: button,
			},
		})
	}
	return nil
}

func InputProcessMouseMove(x uint16, y uint16) error {
	// Only process if actually different
	if inputState.MouseCurrent.X != x || inputState.MouseCurrent.Y != y {
		// NOTE: Enable this if debugging.
		LogDebug("Mouse pos: %d, %d!", x, y)

		// Update internal state.
		inputState.MouseCurrent.X = x
		inputState.MouseCurrent.Y = y

		// Fire the event.
		EventFire(EventContext{
			Type: EVENT_CODE_MOUSE_MOVED,
			Data: &MouseEvent{
				PosX: x,
				PosY: y,
			},
		})
	}
	return nil
}

func InputProcessMouseWheel(zDelta int8) error {
	// Fire the event.
	EventFire(EventContext{
		Type: EVENT_CODE_MOUSE_WHEEL,
		Data: &MouseEvent{
			Scroll: zDelta,
		},
	})
	return nil
}
