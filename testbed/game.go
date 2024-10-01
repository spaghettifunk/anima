package testbed

import (
	"github.com/spaghettifunk/alaska-engine/engine"
	"github.com/spaghettifunk/alaska-engine/engine/core"
)

func NewTestGame() *engine.Game {
	return &engine.Game{
		ApplicationConfig: &engine.ApplicationConfig{
			StartPosX:   100,
			StartPosY:   100,
			StartWidth:  1280,
			StartHeight: 720,
			Name:        "Alaska Game Engine",
		},
		State:        nil,
		FnInitialize: Initialize,
		FnUpdate:     Update,
		FnRender:     Render,
		FnOnResize:   OnResize,
	}
}

func Initialize() error {
	core.LogInfo("TestGame Initialize fn....")
	return nil
}

func Update(deltaTime float32) error {
	core.LogInfo("TestGame Update fn....")
	return nil
}

func Render(deltaTime float32) error {
	core.LogInfo("TestGame Render fn....")
	return nil
}

func OnResize(width uint32, height uint32) error {
	core.LogInfo("TestGame OnResize fn....")
	return nil
}
