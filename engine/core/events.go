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
	EVENT_CODE_KEY_PRESSED EventCode = 0x02

	// Keyboard key released.
	EVENT_CODE_KEY_RELEASED EventCode = 0x03

	// Mouse button pressed.
	EVENT_CODE_BUTTON_PRESSED EventCode = 0x04

	// Mouse button released.
	EVENT_CODE_BUTTON_RELEASED EventCode = 0x05

	// Mouse moved.
	EVENT_CODE_MOUSE_MOVED EventCode = 0x06

	// Mouse moved.
	EVENT_CODE_MOUSE_WHEEL EventCode = 0x07

	// Resized/resolution changed from the OS.
	EVENT_CODE_RESIZED EventCode = 0x08

	// Change the render mode for debugging purposes.
	EVENT_CODE_SET_RENDER_MODE EventCode = 0x0A

	/** @brief The hovered-over object id, if there is one.
	 */
	EVENT_CODE_OBJECT_HOVER_ID_CHANGED EventCode = 0x15

	EVENT_CODE_DEBUG0 EventCode = 0x16
	EVENT_CODE_DEBUG1 EventCode = 0x17

	/**
	 * @brief An event fired by the renderer backend to indicate when any render targets
	 * associated with the default window resources need to be refreshed (i.e. a window resize)
	 */
	EVENT_CODE_DEFAULT_RENDERTARGET_REFRESH_REQUIRED EventCode = 0x16

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
func EventSystemInitialize() error {
	onceEventInit.Do(func() {
		eventSystem = &EventSystem{
			subscribers: make(map[EventCode][]EventCallback),
			eventChan:   make(chan EventContext, MAX_EVENTS_BUFFER_SIZE), // Buffered channel
		}
	})
	return nil
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
