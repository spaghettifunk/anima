package engine

import (
	"fmt"
	"sync"

	"github.com/spaghettifunk/anima/engine/core"
	"github.com/spaghettifunk/anima/engine/platform"
	"github.com/spaghettifunk/anima/engine/renderer"
	"github.com/spaghettifunk/anima/engine/renderer/metadata"
)

type ApplicationConfig struct {
	// Window starting position x axis, if applicable.
	StartPosX uint32
	// Window starting position y axis, if applicable.
	StartPosY uint32
	// Window starting width, if applicable.
	StartWidth uint32
	// Window starting height, if applicable.
	StartHeight uint32
	// The application name used in windowing, if applicable.
	Name string
}

type applicationState struct {
	GameInstance *Game
	IsRunning    bool
	IsSuspended  bool
	Platform     *platform.Platform
	Width        uint32
	Height       uint32
	Clock        *core.Clock
	LastTime     float64
}

var newApplication sync.Once

var (
	initialize bool = false
	appState   *applicationState
)

func ApplicationCreate(gameInstance *Game) error {
	if initialize {
		return fmt.Errorf("application already initialized")
	}

	newApplication.Do(func() {
		appState = &applicationState{
			GameInstance: gameInstance,
			Clock:        core.NewClock(),
			Platform:     platform.New(),
			IsRunning:    true,
			IsSuspended:  false,
			Width:        gameInstance.ApplicationConfig.StartWidth,
			Height:       gameInstance.ApplicationConfig.StartHeight,
			LastTime:     0,
		}
	})

	// initialize input
	if err := core.InputInitialize(); err != nil {
		return err
	}

	// initialize events
	if !core.EventSystemInitialize() {
		return fmt.Errorf("failed to initialize the event system")
	}

	// register some events
	core.EventRegister(core.EVENT_CODE_APPLICATION_QUIT, applicationOnEvent)
	core.EventRegister(core.EVENT_CODE_KEY_PRESSED, applicationOnKey)
	core.EventRegister(core.EVENT_CODE_KEY_RELEASED, applicationOnKey)
	core.EventRegister(core.EVENT_CODE_RESIZED, applicationOnResized)

	if err := appState.Platform.Startup(appState.GameInstance.ApplicationConfig.Name,
		appState.GameInstance.ApplicationConfig.StartPosX,
		appState.GameInstance.ApplicationConfig.StartPosY,
		appState.GameInstance.ApplicationConfig.StartWidth,
		appState.GameInstance.ApplicationConfig.StartHeight); err != nil {
		return err
	}

	// initialize renderer
	if err := renderer.Initialize(appState.GameInstance.ApplicationConfig.Name, appState.Width, appState.Height, appState.Platform); err != nil {
		return err
	}

	if err := appState.GameInstance.FnInitialize(); err != nil {
		return err
	}

	if err := appState.GameInstance.FnOnResize(appState.Width, appState.Height); err != nil {
		return err
	}

	initialize = true

	return nil
}

func ApplicationRun() error {
	appState.Clock.Start()
	appState.Clock.Update()

	appState.LastTime = appState.Clock.Elapsed()

	// start goroutine to process all the events around the engine
	go core.ProcessEvents()

	var runningTime float64 = 0.0
	var frameCount uint8 = 0
	var targetFrameSeconds float64 = 1.0 / 60.0

	for appState.IsRunning {
		if !appState.Platform.PumpMessages() {
			appState.IsRunning = false
		}

		if !appState.IsSuspended {
			// Update clock and get delta time.
			appState.Clock.Update()

			var current_time float64 = appState.Clock.Elapsed()
			var delta float64 = (current_time - appState.LastTime)
			var frame_start_time float64 = platform.GetAbsoluteTime()

			if err := appState.GameInstance.FnUpdate(delta); err != nil {
				core.LogFatal("Game update failed, shutting down.")
				appState.IsRunning = false
				break
			}

			// Call the game's render routine.
			if err := appState.GameInstance.FnRender(delta); err != nil {
				core.LogFatal("Game render failed, shutting down.")
				appState.IsRunning = false
				break
			}

			// TODO: refactor packet creation
			packet := &metadata.RenderPacket{
				DeltaTime: delta,
			}
			renderer.DrawFrame(packet)

			// Figure out how long the frame took and, if below
			var frame_end_time float64 = platform.GetAbsoluteTime()
			var frame_elapsed_time float64 = frame_end_time - frame_start_time
			runningTime += frame_elapsed_time
			var remaining_seconds float64 = targetFrameSeconds - frame_elapsed_time

			if remaining_seconds > 0 {
				remaining_ms := (remaining_seconds * 1000)
				// If there is time left, give it back to the OS.
				limit_frames := false
				if remaining_ms > 0 && limit_frames {
					//    platform_sleep(remaining_ms - 1);
				}
				frameCount++
			}

			// NOTE: Input update/state copying should always be handled
			// after any input should be recorded; I.E. before this line.
			// As a safety, input is the last thing to be updated before
			// this frame ends.
			core.InputUpdate(delta)

			// Update last time
			appState.LastTime = current_time
		}
	}

	appState.IsRunning = false

	core.EventSystemShutdown()
	core.InputShutdown()
	renderer.Shutdown()

	appState.Platform.Shutdown()

	return nil
}

// ApplicationGetFramebufferSize returns the width and height (in this order)
// of the application Framebuffer
func ApplicationGetFramebufferSize() (uint32, uint32) {
	return 0, 0
}

func applicationOnEvent(context core.EventContext) {
	switch context.Type {
	case core.EVENT_CODE_APPLICATION_QUIT:
		{
			core.LogInfo("EVENT_CODE_APPLICATION_QUIT recieved, shutting down.\n")
			appState.IsRunning = false
		}
	}
}

func applicationOnKey(context core.EventContext) {
	ke, ok := context.Data.(*core.KeyEvent)
	if !ok {
		core.LogError("wrong event associated with the event type `%d`", context.Type)
		return
	}

	key_code := ke.KeyCode

	if context.Type == core.EVENT_CODE_KEY_PRESSED {
		if key_code == core.KEY_ESCAPE {
			// NOTE: Technically firing an event to itself, but there may be other listeners.
			data := core.EventContext{
				Type: core.EVENT_CODE_APPLICATION_QUIT,
			}
			core.EventFire(data)
			// Block anything else from processing this.
			return
		} else if key_code == core.KEY_A {
			// Example on checking for a key
			core.LogDebug("Explicit - A key pressed!")
		} else {
			core.LogDebug("'%c' key pressed in window.", key_code)
		}
	} else if context.Type == core.EVENT_CODE_KEY_RELEASED {
		if key_code == core.KEY_B {
			// Example on checking for a key
			core.LogDebug("Explicit - B key released!")
		} else {
			core.LogDebug("'%c' key released in window.", key_code)
		}
	}
}

func applicationOnResized(context core.EventContext) {
	if context.Type == core.EVENT_CODE_RESIZED {
		se, ok := context.Data.(*core.SystemEvent)
		if !ok {
			core.LogError("wrong event associated with the event type `%d`", context.Type)
			return
		}

		width := se.WindowWidth
		height := se.WindowHeight

		// Check if different. If so, trigger a resize event.
		if width != appState.Width || height != appState.Height {
			appState.Width = width
			appState.Height = height

			core.LogDebug("Window resize: %d, %d", width, height)

			// Handle minimization
			if width == 0 || height == 0 {
				core.LogInfo("Window minimized, suspending application.")
				appState.IsSuspended = true
				return
			} else {
				if appState.IsSuspended {
					core.LogInfo("Window restored, resuming application.")
					appState.IsSuspended = false
				}
				appState.GameInstance.FnOnResize(uint32(width), uint32(height))
				renderer.OnResize(uint16(width), uint16(height))
			}
		}
	}
}
