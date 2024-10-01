package core

import "sync"

type EventContext struct {
	// 128 bytes
	Data struct {
		I64 [2]int64
		U64 [2]uint64
		F64 [2]float64

		I32 [4]int32
		U32 [4]uint32
		F32 [4]float32

		I16 [8]int16
		U16 [8]uint16

		I8 [16]int8
		U8 [16]uint8

		C [16]string
	}
}

// System internal event codes. Application should use codes beyond 255.
type SystemEventCode int

const (
	// Shuts the application down on the next frame.
	EVENT_CODE_APPLICATION_QUIT SystemEventCode = 0x01

	// Keyboard key pressed.
	/* Context usage:
	 * u16 key_code = data.data.u16[0];
	 */
	EVENT_CODE_KEY_PRESSED SystemEventCode = 0x02

	// Keyboard key released.
	/* Context usage:
	 * u16 key_code = data.data.u16[0];
	 */
	EVENT_CODE_KEY_RELEASED SystemEventCode = 0x03

	// Mouse button pressed.
	/* Context usage:
	 * u16 button = data.data.u16[0];
	 */
	EVENT_CODE_BUTTON_PRESSED SystemEventCode = 0x04

	// Mouse button released.
	/* Context usage:
	 * u16 button = data.data.u16[0];
	 */
	EVENT_CODE_BUTTON_RELEASED SystemEventCode = 0x05

	// Mouse moved.
	/* Context usage:
	 * u16 x = data.data.u16[0];
	 * u16 y = data.data.u16[1];
	 */
	EVENT_CODE_MOUSE_MOVED SystemEventCode = 0x06

	// Mouse moved.
	/* Context usage:
	 * u8 z_delta = data.data.u8[0];
	 */
	EVENT_CODE_MOUSE_WHEEL SystemEventCode = 0x07

	// Resized/resolution changed from the OS.
	/* Context usage:
	 * u16 width = data.data.u16[0];
	 * u16 height = data.data.u16[1];
	 */
	EVENT_CODE_RESIZED SystemEventCode = 0x08

	MAX_EVENT_CODE SystemEventCode = 0xFF
)

// This should be more than enough codes...
const MAX_MESSAGE_CODES = 16384

type registeredEvent struct {
	listener interface{}
	callback FnOnEvent
}

type eventCodeEntry struct {
	events []*registeredEvent
}

// State structure.
type eventSystemState struct {
	// Lookup table for event codes.
	registered [MAX_MESSAGE_CODES]eventCodeEntry
}

/**
 * Event system internal state.
 */
var onceEvent sync.Once
var isInitialized bool = false
var eventState *eventSystemState = nil

// Should return true if handled.
type FnOnEvent func(code SystemEventCode, sender interface{}, listener_inst interface{}, data EventContext) bool

func EventInitialize() bool {
	if isInitialized {
		return false
	}
	isInitialized = false
	onceEvent.Do(func() {
		eventState = &eventSystemState{}
	})
	isInitialized = true
	return true
}

func EventShutdown() error {
	// Free the events arrays. And objects pointed to should be destroyed on their own.
	for i := 0; i < MAX_MESSAGE_CODES; i++ {
		if len(eventState.registered[i].events) != 0 {
			eventState.registered[i].events = nil
		}
	}
	return nil
}

/**
 * Register to listen for when events are sent with the provided code. Events with duplicate
 * listener/callback combos will not be registered again and will cause this to return FALSE.
 * @param code The event code to listen for.
 * @param listener A pointer to a listener instance. Can be 0/NULL.
 * @param on_event The callback function pointer to be invoked when the event code is fired.
 * @returns TRUE if the event is successfully registered; otherwise false.
 */
func EventRegister(code SystemEventCode, listener interface{}, onEvent FnOnEvent) bool {
	if !isInitialized {
		return false
	}
	if len(eventState.registered[code].events) == 0 {
		eventState.registered[code].events = make([]*registeredEvent, 1)
	}
	registeredCount := len(eventState.registered[code].events)
	for i := 0; i < registeredCount; i++ {
		if eventState.registered[code].events[i].listener == listener {
			// TODO: warn
			return false
		}
	}
	// If at this point, no duplicate was found. Proceed with registration.
	event := &registeredEvent{
		listener: listener,
		callback: onEvent,
	}
	eventState.registered[code].events = append(eventState.registered[code].events, event)
	return true
}

/**
 * Unregister from listening for when events are sent with the provided code. If no matching
 * registration is found, this function returns FALSE.
 * @param code The event code to stop listening for.
 * @param listener A pointer to a listener instance. Can be 0/NULL.
 * @param on_event The callback function pointer to be unregistered.
 * @returns TRUE if the event is successfully unregistered; otherwise false.
 */
func EventUnregister(code SystemEventCode, listener interface{}, onEvent FnOnEvent) bool {
	if !isInitialized {
		return false
	}

	// On nothing is registered for the code, boot out.
	if len(eventState.registered[code].events) == 0 {
		// TODO: warn
		return false
	}

	registeredCount := len(eventState.registered[code].events)
	for i := 0; i < registeredCount; i++ {
		e := eventState.registered[code].events[i]
		if e.listener == listener && e.callback != nil {
			// Found one, remove it
			_ = eventState.registered[code].events[len(eventState.registered[code].events)-1]
			eventState.registered[code].events = eventState.registered[code].events[:len(eventState.registered[code].events)-1]
			return true
		}
	}
	// Not found.
	return false
}

/**
 * Fires an event to listeners of the given code. If an event handler returns
 * TRUE, the event is considered handled and is not passed on to any more listeners.
 * @param code The event code to fire.
 * @param sender A pointer to the sender. Can be 0/NULL.
 * @param data The event data.
 * @returns TRUE if handled, otherwise FALSE.
 */
func EventFire(code SystemEventCode, sender interface{}, context EventContext) bool {
	if !isInitialized {
		return false
	}
	// If nothing is registered for the code, boot out.
	if len(eventState.registered[code].events) == 0 {
		return false
	}
	registeredCount := len(eventState.registered[code].events)
	for i := 0; i < registeredCount; i++ {
		e := eventState.registered[code].events[i]
		if e.callback(code, sender, e.listener, context) {
			// Message has been handled, do not send to other listeners.
			return true
		}
	}
	// Not found.
	return false
}
