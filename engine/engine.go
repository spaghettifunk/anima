package engine

import "github.com/spaghettifunk/anima/engine/core"

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
	currentStage Stage
	game         *Game
}

func New(g *Game) (*Engine, error) {
	return &Engine{
		currentStage: EngineStageUninitialized,
		game:         g,
	}, nil
}

func (e *Engine) Initialize() error {
	// initialize memory
	// ....

	if err := ApplicationCreate(e.game); err != nil {
		core.LogError(err.Error())
		return err
	}

	return nil
}

func (e *Engine) Run() error {
	if err := ApplicationRun(); err != nil {
		return err
	}
	return nil
}

func (e *Engine) Shutdown() error {
	// shutdown memory
	// ...
	return nil
}
