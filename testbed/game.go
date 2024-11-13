package testbed

import (
	"fmt"

	"github.com/spaghettifunk/anima/engine"
	"github.com/spaghettifunk/anima/engine/core"
	"github.com/spaghettifunk/anima/engine/math"
	"github.com/spaghettifunk/anima/engine/renderer/components"
	"github.com/spaghettifunk/anima/engine/renderer/metadata"
)

type TestGame struct {
	*engine.Game
}

type gameState struct {
	DeltaTime   uint32
	WorldCamera *components.Camera
}

func NewTestGame() *TestGame {
	tg := &TestGame{
		Game: &engine.Game{
			ApplicationConfig: &engine.ApplicationConfig{
				StartPosX:   100,
				StartPosY:   100,
				StartWidth:  1280,
				StartHeight: 720,
				Name:        "Anima Game Engine",
				LogLevel:    core.DebugLevel,
			},
			State: &gameState{},
		},
	}

	tg.FnInitialize = tg.Initialize
	tg.FnUpdate = tg.Update
	tg.FnRender = tg.Render
	tg.FnOnResize = tg.OnResize

	return tg
}

func (g *TestGame) Initialize() error {
	core.LogDebug("TestGame Initialize fn....")

	if g.SystemManager == nil {
		panic(fmt.Errorf("the engine is not yet initialized with all the system managers "))
	}

	state := g.State.(*gameState)
	state.WorldCamera = g.SystemManager.CameraSystem.GetDefault()
	state.WorldCamera.SetPosition(math.NewVec3(10.5, 5.0, 9.5))

	return nil
}

var temp_move_speed float32 = 50.0

func (g *TestGame) Update(deltaTime float64) error {
	state := g.State.(*gameState)

	// HACK: temp hack to move camera around.
	if core.InputIsKeyDown(core.KEY_A) || core.InputIsKeyDown(core.KEY_LEFT) {
		state.WorldCamera.Yaw(float32(1.0 * deltaTime))
	}

	if core.InputIsKeyDown(core.KEY_D) || core.InputIsKeyDown(core.KEY_RIGHT) {
		state.WorldCamera.Yaw(float32(-1.0 * deltaTime))
	}

	if core.InputIsKeyDown(core.KEY_UP) {
		state.WorldCamera.Yaw(float32(1.0 * deltaTime))
	}

	if core.InputIsKeyDown(core.KEY_DOWN) {
		state.WorldCamera.Yaw(float32(-1.0 * deltaTime))
	}

	if core.InputIsKeyDown(core.KEY_W) {
		state.WorldCamera.Yaw(temp_move_speed * float32(deltaTime))
	}

	if core.InputIsKeyDown(core.KEY_S) {
		state.WorldCamera.Yaw(temp_move_speed * float32(deltaTime))
	}

	if core.InputIsKeyDown(core.KEY_Q) {
		state.WorldCamera.Yaw(temp_move_speed * float32(deltaTime))
	}

	if core.InputIsKeyDown(core.KEY_E) {
		state.WorldCamera.Yaw(temp_move_speed * float32(deltaTime))
	}

	if core.InputIsKeyDown(core.KEY_SPACE) {
		state.WorldCamera.Yaw(temp_move_speed * float32(deltaTime))
	}

	if core.InputIsKeyDown(core.KEY_X) {
		state.WorldCamera.Yaw(temp_move_speed * float32(deltaTime))
	}

	// TODO: temp
	if core.InputIsKeyUp(core.KEY_P) && core.InputWasKeyDown(core.KEY_P) {
		core.LogDebug("Pos:[%.2f, %.2f, %.2f", state.WorldCamera.Position.X, state.WorldCamera.Position.Y, state.WorldCamera.Position.Z)
	}

	// RENDERER DEBUG FUNCTIONS
	if core.InputIsKeyUp(core.KEY_NUMPAD1) && core.InputWasKeyDown(core.KEY_NUMPAD1) {
		data := core.EventContext{
			Type: core.EVENT_CODE_SET_RENDER_MODE,
			Data: metadata.RENDERER_VIEW_MODE_LIGHTING,
		}
		core.EventFire(data)
	}

	if core.InputIsKeyUp(core.KEY_NUMPAD2) && core.InputWasKeyDown(core.KEY_NUMPAD2) {
		data := core.EventContext{
			Type: core.EVENT_CODE_SET_RENDER_MODE,
			Data: metadata.RENDERER_VIEW_MODE_NORMALS,
		}
		core.EventFire(data)
	}

	if core.InputIsKeyUp(core.KEY_NUMPAD0) && core.InputWasKeyDown(core.KEY_NUMPAD0) {
		data := core.EventContext{
			Type: core.EVENT_CODE_SET_RENDER_MODE,
			Data: metadata.RENDERER_VIEW_MODE_DEFAULT,
		}
		core.EventFire(data)
	}

	// TODO: temp
	// Bind a key to load up some data.
	if core.InputIsKeyUp(core.KEY_L) && core.InputWasKeyDown(core.KEY_L) {
		data := core.EventContext{}
		core.EventFire(data)
	}

	// TODO: end temp

	return nil
}

func (g *TestGame) Render(deltaTime float64) error {
	return nil
}

func (g *TestGame) OnResize(width uint32, height uint32) error {
	return nil
}
