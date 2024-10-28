package core

import (
	"sync"
)

// System internal event codes. Application should use codes beyond 255.
type EventCode int

const (
	// Shuts the application down on the next frame.
	EVENT_CODE_APPLICATION_QUIT EventCode = 0x01

	// Keyboard key pressed.
	/* Context usage:
	 * u16 key_code = data.data.u16[0];
	 */
	EVENT_CODE_KEY_PRESSED EventCode = 0x02

	// Keyboard key released.
	/* Context usage:
	 * u16 key_code = data.data.u16[0];
	 */
	EVENT_CODE_KEY_RELEASED EventCode = 0x03

	// Mouse button pressed.
	/* Context usage:
	 * u16 button = data.data.u16[0];
	 */
	EVENT_CODE_BUTTON_PRESSED EventCode = 0x04

	// Mouse button released.
	/* Context usage:
	 * u16 button = data.data.u16[0];
	 */
	EVENT_CODE_BUTTON_RELEASED EventCode = 0x05

	// Mouse moved.
	/* Context usage:
	 * u16 x = data.data.u16[0];
	 * u16 y = data.data.u16[1];
	 */
	EVENT_CODE_MOUSE_MOVED EventCode = 0x06

	// Mouse moved.
	/* Context usage:
	 * u8 z_delta = data.data.u8[0];
	 */
	EVENT_CODE_MOUSE_WHEEL EventCode = 0x07

	// Resized/resolution changed from the OS.
	/* Context usage:
	 * u16 width = data.data.u16[0];
	 * u16 height = data.data.u16[1];
	 */
	EVENT_CODE_RESIZED EventCode = 0x08

	MAX_EVENT_CODE EventCode = 0xFF
)

type EventContext struct {
	Type EventCode
	Data interface{}
}

type MouseEvent struct {
	PosX   uint16
	PosY   uint16
	Button Button
	Scroll int8
}

type KeyEvent struct {
	KeyCode KeyCode
}

type SystemEvent struct {
	WindowWidth  uint32
	WindowHeight uint32
}

type EventCallback func(event EventContext)

type EventSystem struct {
	subscribers map[EventCode][]EventCallback
	eventChan   chan EventContext // Channel for event communication
}

const MAX_EVENTS_BUFFER_SIZE = 100

var onceEventInit sync.Once
var eventSystem *EventSystem

// Initialize the event system with a buffered channel
func EventSystemInitialize() bool {
	onceEventInit.Do(func() {
		eventSystem = &EventSystem{
			subscribers: make(map[EventCode][]EventCallback),
			eventChan:   make(chan EventContext, MAX_EVENTS_BUFFER_SIZE), // Buffered channel
		}
	})
	return true
}

// Subscribe to an event type
func EventRegister(code EventCode, callback EventCallback) {
	eventSystem.subscribers[code] = append(eventSystem.subscribers[code], callback)
}

func EventFire(context EventContext) {
	eventSystem.eventChan <- context
}

// Process events in a separate goroutine or in the game loop
func ProcessEvents() {
	for event := range eventSystem.eventChan {
		if callbacks, ok := eventSystem.subscribers[event.Type]; ok {
			for _, callback := range callbacks {
				callback(event)
			}
		}
	}
}

func EventSystemShutdown() error {
	close(eventSystem.eventChan)
	eventSystem = nil
	return nil
}
