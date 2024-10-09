package testbed

import (
	"github.com/spaghettifunk/anima/engine"
	"github.com/spaghettifunk/anima/engine/core"
)

func NewTestGame() *engine.Game {
	return &engine.Game{
		ApplicationConfig: &engine.ApplicationConfig{
			StartPosX:   100,
			StartPosY:   100,
			StartWidth:  1280,
			StartHeight: 720,
			Name:        "Anima Game Engine",
		},
		State:        nil,
		FnInitialize: Initialize,
		FnUpdate:     Update,
		FnRender:     Render,
		FnOnResize:   OnResize,
	}
}

func Initialize() error {
	core.LogDebug("TestGame Initialize fn....")
	return nil
}

func Update(deltaTime float64) error {
	core.LogDebug("TestGame Update fn....")
	return nil
}

func Render(deltaTime float64) error {
	core.LogDebug("TestGame Render fn....")
	return nil
}

func OnResize(width uint32, height uint32) error {
	core.LogDebug("TestGame OnResize fn....")
	return nil
}
