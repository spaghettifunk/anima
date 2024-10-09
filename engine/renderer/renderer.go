package renderer

import (
	"sync"

	"github.com/spaghettifunk/anima/engine/core"
	"github.com/spaghettifunk/anima/engine/platform"
	"github.com/spaghettifunk/anima/engine/renderer/vulkan"
)

type RendererBackend interface {
	Initialize(appName string, appWidth, appHeight uint32) error
	Shutdow() error
	Resized(width, height uint16) error
	BeginFrame(deltaTime float64) error
	EndFrame(deltaTime float64) error
}

type RendererType uint8

const (
	Vulkan RendererType = iota
	DirectX
	Metal
	OpenGL
)

type Renderer struct {
	backend RendererBackend
}

type RenderPacket struct {
	DeltaTime float64
}

var initRenderer sync.Once
var renderer *Renderer

func Initialize(appName string, appWidth, appHeight uint32, platform *platform.Platform) error {
	initRenderer.Do(func() {
		renderer = &Renderer{
			backend: vulkan.New(platform),
		}
	})
	return renderer.backend.Initialize(appName, appWidth, appHeight)
}

func Shutdown() error {
	return renderer.backend.Shutdow()
}

func BeginFrame(deltaTime float64) error {
	return renderer.backend.BeginFrame(deltaTime)
}

func EndFrame(deltaTime float64) error {
	return renderer.backend.EndFrame(deltaTime)
}

func OnResize(width, height uint16) error {
	return renderer.backend.Resized(width, height)
}

func DrawFrame(renderPacket *RenderPacket) error {
	if err := BeginFrame(renderPacket.DeltaTime); err != nil {
		core.LogError(err.Error())
		return err
	}
	if err := EndFrame(renderPacket.DeltaTime); err != nil {
		core.LogError("RendererEndFrame failed. Application shutting down...")
		return err
	}
	return nil
}
