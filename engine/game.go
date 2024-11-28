package engine

import (
	"github.com/spaghettifunk/anima/engine/renderer/metadata"
	"github.com/spaghettifunk/anima/engine/systems"
)

type Game struct {
	ApplicationConfig *ApplicationConfig
	SystemManager     *systems.SystemManager
	State             interface{}
	FnBoot            Boot
	FnInitialize      Initialize
	FnUpdate          Update
	FnRender          Render
	FnOnResize        OnResize
	FnShutdown        Shutdown
}

type Boot func() error
type Initialize func() error
type Update func(deltaTime float64) error
type Render func(packer *metadata.RenderPacket, deltaTime float64) error
type OnResize func(width uint32, height uint32) error
type Shutdown func() error
