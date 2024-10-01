package engine

import (
	"fmt"
	"sync"

	"github.com/spaghettifunk/alaska-engine/engine/core"
	"github.com/spaghettifunk/alaska-engine/engine/platform"
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
	GameInstance  *Game
	IsRunning     bool
	IsSuspended   bool
	PlatformState *platform.Platform
	Width         uint32
	Height        uint32
	Clock         *core.Clock
	LastTime      float64
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
			IsRunning:    true,
			IsSuspended:  false,
			Width:        0,
			Height:       0,
			LastTime:     0,
		}
	})

	// initialize input
	if err := core.InputInitialize(); err != nil {
		return err
	}

	// initialize events
	if !core.EventInitialize() {
		return fmt.Errorf("failed to initialize the event system")
	}

	// register some events
	core.EventRegister(core.EVENT_CODE_APPLICATION_QUIT, 0, applicationOnEvent)
	core.EventRegister(core.EVENT_CODE_KEY_PRESSED, 0, applicationOnKey)
	core.EventRegister(core.EVENT_CODE_KEY_RELEASED, 0, applicationOnKey)
	core.EventRegister(core.EVENT_CODE_RESIZED, 0, applicationOnResized)

	p, err := platform.New()
	if err != nil {
		return err
	}

	if err := p.Startup(appState.GameInstance.ApplicationConfig.Name,
		appState.GameInstance.ApplicationConfig.StartPosX,
		appState.GameInstance.ApplicationConfig.StartPosY,
		appState.GameInstance.ApplicationConfig.StartWidth,
		appState.GameInstance.ApplicationConfig.StartHeight); err != nil {
		return err
	}

	// initialize renderer
	// ..

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

	// var runningTime float64 = 0.0
	// var frameCount uint8 = 0
	// var targetFrameSeconds float64 = 1.0 / 60.0

	// for appState.IsRunning {

	// }

	return nil
}

// ApplicationGetFramebufferSize returns the width and height (in this order)
// of the application Framebuffer
func ApplicationGetFramebufferSize() (uint32, uint32) {
	return 0, 0
}

func applicationOnEvent(code core.SystemEventCode, sender interface{}, listener_inst interface{}, context core.EventContext) bool {
	switch code {
	case core.EVENT_CODE_APPLICATION_QUIT:
		{
			core.LogInfo("EVENT_CODE_APPLICATION_QUIT recieved, shutting down.\n")
			appState.IsRunning = false
			return true
		}
	}
	return false
}

func applicationOnKey(code core.SystemEventCode, sender interface{}, listener_inst interface{}, context core.EventContext) bool {
	if code == core.EVENT_CODE_KEY_PRESSED {
		key_code := context.Data.U16[0]
		if key_code == uint16(core.KEY_ESCAPE) {
			// NOTE: Technically firing an event to itself, but there may be other listeners.
			data := core.EventContext{}
			core.EventFire(core.EVENT_CODE_APPLICATION_QUIT, 0, data)
			// Block anything else from processing this.
			return true
		} else if key_code == uint16(core.KEY_A) {
			// Example on checking for a key
			core.LogDebug("Explicit - A key pressed!")
		} else {
			core.LogDebug("'%c' key pressed in window.", key_code)
		}
	} else if code == core.EVENT_CODE_KEY_RELEASED {
		key_code := context.Data.U16[0]
		if key_code == uint16(core.KEY_B) {
			// Example on checking for a key
			core.LogDebug("Explicit - B key released!")
		} else {
			core.LogDebug("'%c' key released in window.", key_code)
		}
	}
	return false
}

func applicationOnResized(code core.SystemEventCode, sender interface{}, listener_inst interface{}, context core.EventContext) bool {
	if code == core.EVENT_CODE_RESIZED {
		width := context.Data.U16[0]
		height := context.Data.U16[1]

		// Check if different. If so, trigger a resize event.
		if width != uint16(appState.Width) || height != uint16(appState.Height) {
			appState.Width = uint32(width)
			appState.Height = uint32(height)

			core.LogDebug("Window resize: %d, %d", width, height)

			// Handle minimization
			if width == 0 || height == 0 {
				core.LogInfo("Window minimized, suspending application.")
				appState.IsSuspended = true
				return true
			} else {
				if appState.IsSuspended {
					core.LogInfo("Window restored, resuming application.")
					appState.IsSuspended = false
				}
				appState.GameInstance.FnOnResize(uint32(width), uint32(height))

				// renderer_on_resized(width, height)
			}
		}
	}
	// Event purposely not handled to allow other listeners to get this.
	return false
}
