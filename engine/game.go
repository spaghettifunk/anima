package engine

import "github.com/spaghettifunk/anima/engine/systems"

type Game struct {
	ApplicationConfig *ApplicationConfig
	SystemManager     *systems.SystemManager
	State             interface{}
	FnInitialize      Initialize
	FnUpdate          Update
	FnRender          Render
	FnOnResize        OnResize
}

type Initialize func() error
type Update func(deltaTime float64) error
type Render func(deltaTime float64) error
type OnResize func(width uint32, height uint32) error
