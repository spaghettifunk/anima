package engine

import (
	"fmt"
	"os"
	"runtime"

	"github.com/spaghettifunk/anima/engine/assets"
	"github.com/spaghettifunk/anima/engine/core"
	"github.com/spaghettifunk/anima/engine/platform"
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
	assetManager  *assets.AssetManager
	systemManager *systems.SystemManager
	width         uint32
	height        uint32
	clock         *core.Clock
	lastTime      float64
}

func init() {
	runtime.LockOSThread()
}

func New(g *Game) (*Engine, error) {
	// initialize the logger immediately
	core.InitializeLogger(g.ApplicationConfig.LogLevel)

	p := platform.New()

	am, err := assets.NewAssetManager()
	if err != nil {
		return nil, err
	}

	sm, err := systems.NewSystemManager(g.ApplicationConfig.Name, g.ApplicationConfig.StartWidth, g.ApplicationConfig.StartHeight, p, am)
	if err != nil {
		return nil, err
	}

	g.SystemManager = sm

	if err := g.FnBoot(); err != nil {
		return nil, err
	}

	return &Engine{
		currentStage:  EngineStageUninitialized,
		gameInstance:  g,
		clock:         core.NewClock(),
		platform:      p,
		assetManager:  am,
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
	if err := core.EventSystemInitialize(); err != nil {
		return err
	}

	// initialize metrics
	if err := core.MetricsInitialize(); err != nil {
		return err
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

	// initialize subsystems
	wd, err := os.Getwd()
	if err != nil {
		return err
	}
	if err := e.assetManager.Initialize(fmt.Sprintf("%s/assets", wd)); err != nil {
		return err
	}

	// initialize all the managers (including the rendering system)
	if err := e.systemManager.Initialize(); err != nil {
		return err
	}

	// Load render views from app config.
	viewConfigsCount := len(e.gameInstance.ApplicationConfig.RenderViewConfigs)
	for v := 0; v < viewConfigsCount; v++ {
		viewConfig := e.gameInstance.ApplicationConfig.RenderViewConfigs[v]
		if err := e.systemManager.RenderViewSystem.Create(viewConfig); err != nil {
			core.LogError("failed to create view '%s'. Aborting application", viewConfig.Name)
			return err
		}
	}

	if err := e.gameInstance.FnInitialize(); err != nil {
		return err
	}

	if err := e.systemManager.OnResize(uint16(e.width), uint16(e.height)); err != nil {
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

	// var runningTime float64 = 0.0
	var frameCount uint8 = 0
	var targetFrameSeconds float64 = 1.0 / 60.0
	var frameElapsedTime float64 = 0

	for e.isRunning {
		if !e.platform.PumpMessages() {
			e.isRunning = false
		}

		if !e.isSuspended {
			// Update clock and get delta time.
			e.clock.Update()

			var currentTime float64 = e.clock.Elapsed()
			var delta float64 = (currentTime - e.lastTime)
			var frameStartTime float64 = platform.GetAbsoluteTime()

			core.MetricsUpdate(frameElapsedTime)

			if err := e.gameInstance.FnUpdate(delta); err != nil {
				core.LogFatal("Game update failed, shutting down.")
				e.isRunning = false
				break
			}

			// TODO: refactor packet creation
			packet := &metadata.RenderPacket{
				DeltaTime: delta,
			}

			// Call the game's render routine.
			if err := e.gameInstance.FnRender(packet, delta); err != nil {
				core.LogFatal("Game render failed, shutting down.")
				e.isRunning = false
				break
			}

			// Draw frame
			if err := e.systemManager.DrawFrame(packet); err != nil {
				core.LogError("failed to draw frame")
				return err
			}

			// Cleanup the packet.
			for i := 0; i < int(packet.ViewCount); i++ {
				if err := e.systemManager.RenderViewSystem.OnDestroyPacket(packet.ViewPackets[i]); err != nil {
					core.LogError("failed to destroy renderview packet")
					return err
				}
			}

			// Figure out how long the frame took and, if below
			var frameEndTime float64 = platform.GetAbsoluteTime()
			frameElapsedTime = frameEndTime - frameStartTime
			// runningTime += frameElapsedTime
			var remainingSeconds float64 = targetFrameSeconds - frameElapsedTime

			if remainingSeconds > 0 {
				remainingMS := (remainingSeconds * 1000)
				// If there is time left, give it back to the OS.
				limitFrames := false
				if remainingMS > 0 && limitFrames {
					e.platform.Sleep(remainingMS - 1)
				}
				frameCount++
			}

			// NOTE: Input update/state copying should always be handled
			// after any input should be recorded; I.E. before this line.
			// As a safety, input is the last thing to be updated before
			// this frame ends.
			core.InputUpdate(delta)

			// Update last time
			e.lastTime = currentTime
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
	if err := e.platform.Shutdown(); err != nil {
		return err
	}
	return nil
}

// ApplicationGetFramebufferSize returns the width and height (in this order)
// of the application Framebuffer
func (e *Engine) GetFramebufferSize() (uint32, uint32) {
	return e.width, e.height
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

	keyCode := ke.KeyCode

	if context.Type == core.EVENT_CODE_KEY_PRESSED {
		if keyCode == core.KEY_ESCAPE {
			// NOTE: Technically firing an event to itself, but there may be other listeners.
			data := core.EventContext{
				Type: core.EVENT_CODE_APPLICATION_QUIT,
			}
			core.EventFire(data)
			// Block anything else from processing this.
			return
		} else if keyCode == core.KEY_A {
			// Example on checking for a key
			core.LogInfo("Explicit - A key pressed!")
		} else {
			core.LogInfo("'%c' key pressed in window.", keyCode)
		}
	} else if context.Type == core.EVENT_CODE_KEY_RELEASED {
		if keyCode == core.KEY_B {
			// Example on checking for a key
			core.LogInfo("Explicit - B key released!")
		} else {
			core.LogInfo("'%c' key released in window.", keyCode)
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
				if err := e.systemManager.OnResize(uint16(width), uint16(height)); err != nil {
					core.LogError(err.Error())
				}
			}
		}
	}
}
