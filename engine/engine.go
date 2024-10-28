package engine

import (
	"fmt"

	"github.com/spaghettifunk/anima/engine/core"
	"github.com/spaghettifunk/anima/engine/platform"
	"github.com/spaghettifunk/anima/engine/renderer"
	"github.com/spaghettifunk/anima/engine/renderer/metadata"
	"github.com/spaghettifunk/anima/engine/systems"
)

type Stage uint8

const (
	// Engine is in an uninitialized state
	EngineStageUninitialized Stage = iota
	// Engine is currently booting up
	EngineStageBooting
	// Engine completed boot process and is ready to be initialized
	EngineStageBootComplete
	// Engine is currently initializing
	EngineStageInitializing
	// Engine initialization is complete
	EngineStageInitialized
	// Engine is currently running
	EngineStageRunning
	// Engine is in the process of shutting down
	EngineStageShuttingDown
)

type Engine struct {
	currentStage  Stage
	gameInstance  *Game
	isRunning     bool
	isSuspended   bool
	platform      *platform.Platform
	renderer      *renderer.Renderer
	systemManager *systems.SystemManager
	width         uint32
	height        uint32
	clock         *core.Clock
	lastTime      float64
}

func New(g *Game) (*Engine, error) {
	p := platform.New()
	r, err := renderer.NewRenderer(g.ApplicationConfig.Name, g.ApplicationConfig.StartWidth, g.ApplicationConfig.StartHeight, p)
	if err != nil {
		core.LogError(err.Error())
		return nil, err
	}

	sm, err := systems.NewSystemManager(r)
	if err != nil {
		core.LogError(err.Error())
		return nil, err
	}

	return &Engine{
		currentStage:  EngineStageUninitialized,
		gameInstance:  g,
		clock:         core.NewClock(),
		platform:      p,
		renderer:      r,
		systemManager: sm,
		isRunning:     true,
		isSuspended:   false,
		width:         g.ApplicationConfig.StartWidth,
		height:        g.ApplicationConfig.StartHeight,
		lastTime:      0,
	}, nil
}

func (e *Engine) Initialize() error {
	// initialize input
	if err := core.InputInitialize(); err != nil {
		return err
	}

	// initialize events
	if !core.EventSystemInitialize() {
		return fmt.Errorf("failed to initialize the event system")
	}

	// register some events
	core.EventRegister(core.EVENT_CODE_APPLICATION_QUIT, e.onEvent)
	core.EventRegister(core.EVENT_CODE_KEY_PRESSED, e.onKey)
	core.EventRegister(core.EVENT_CODE_KEY_RELEASED, e.onKey)
	core.EventRegister(core.EVENT_CODE_RESIZED, e.onResized)

	if err := e.platform.Startup(e.gameInstance.ApplicationConfig.Name,
		e.gameInstance.ApplicationConfig.StartPosX,
		e.gameInstance.ApplicationConfig.StartPosY,
		e.gameInstance.ApplicationConfig.StartWidth,
		e.gameInstance.ApplicationConfig.StartHeight); err != nil {
		return err
	}

	// initialize renderer
	if err := e.renderer.Initialize(); err != nil {
		return err
	}

	if err := e.gameInstance.FnInitialize(); err != nil {
		return err
	}

	if err := e.gameInstance.FnOnResize(e.width, e.height); err != nil {
		return err
	}

	return nil
}

func (e *Engine) Run() error {
	e.clock.Start()
	e.clock.Update()

	e.lastTime = e.clock.Elapsed()

	// start goroutine to process all the events around the engine
	go core.ProcessEvents()

	var runningTime float64 = 0.0
	var frameCount uint8 = 0
	var targetFrameSeconds float64 = 1.0 / 60.0

	for e.isRunning {
		if !e.platform.PumpMessages() {
			e.isRunning = false
		}

		if !e.isSuspended {
			// Update clock and get delta time.
			e.clock.Update()

			var current_time float64 = e.clock.Elapsed()
			var delta float64 = (current_time - e.lastTime)
			var frame_start_time float64 = platform.GetAbsoluteTime()

			if err := e.gameInstance.FnUpdate(delta); err != nil {
				core.LogFatal("Game update failed, shutting down.")
				e.isRunning = false
				break
			}

			// Call the game's render routine.
			if err := e.gameInstance.FnRender(delta); err != nil {
				core.LogFatal("Game render failed, shutting down.")
				e.isRunning = false
				break
			}

			// TODO: refactor packet creation
			packet := &metadata.RenderPacket{
				DeltaTime: delta,
			}
			e.renderer.DrawFrame(packet)

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
			e.lastTime = current_time
		}
	}

	return nil
}

func (e *Engine) Shutdown() error {
	if err := core.EventSystemShutdown(); err != nil {
		return err
	}
	if err := core.InputShutdown(); err != nil {
		return err
	}
	if err := e.systemManager.Shutdown(); err != nil {
		return err
	}
	if err := e.renderer.Shutdown(); err != nil {
		return err
	}
	if err := e.platform.Shutdown(); err != nil {
		return err
	}
	return nil
}

// ApplicationGetFramebufferSize returns the width and height (in this order)
// of the application Framebuffer
func (e *Engine) GetFramebufferSize() (uint32, uint32) {
	return 0, 0
}

func (e *Engine) onEvent(context core.EventContext) {
	switch context.Type {
	case core.EVENT_CODE_APPLICATION_QUIT:
		{
			core.LogInfo("EVENT_CODE_APPLICATION_QUIT recieved, shutting down.\n")
			e.isRunning = false
		}
	}
}

func (e *Engine) onKey(context core.EventContext) {
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

func (e *Engine) onResized(context core.EventContext) {
	if context.Type == core.EVENT_CODE_RESIZED {
		se, ok := context.Data.(*core.SystemEvent)
		if !ok {
			core.LogError("wrong event associated with the event type `%d`", context.Type)
			return
		}

		width := se.WindowWidth
		height := se.WindowHeight

		// Check if different. If so, trigger a resize event.
		if width != e.width || height != e.height {
			e.width = width
			e.height = height

			core.LogDebug("Window resize: %d, %d", width, height)

			// Handle minimization
			if width == 0 || height == 0 {
				core.LogInfo("Window minimized, suspending application.")
				e.isSuspended = true
				return
			} else {
				if e.isSuspended {
					core.LogInfo("Window restored, resuming application.")
					e.isSuspended = false
				}
				e.gameInstance.FnOnResize(uint32(width), uint32(height))
				e.renderer.OnResize(uint16(width), uint16(height))
			}
		}
	}
}
