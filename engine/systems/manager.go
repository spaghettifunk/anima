package systems

import (
	"fmt"
	"os"
	"runtime"

	"github.com/spaghettifunk/anima/engine/platform"
	"github.com/spaghettifunk/anima/engine/renderer/metadata"
)

type SystemManager struct {
	CameraSystem     *CameraSystem
	GeometrySystem   *GeometrySystem
	JobSystem        *JobSystem
	MaterialSystem   *MaterialSystem
	MeshLoaderSystem *MeshLoaderSystem
	RenderViewSystem *RenderViewSystem
	ShaderSystem     *ShaderSystem
	TextureSystem    *TextureSystem
	ResourceSystem   *ResourceSystem
	RendererSystem   *RendererSystem
}

var (
	MaxNumberOfWorkers int = runtime.NumCPU()
)

func NewSystemManager(appName string, width, height uint32, platform *platform.Platform) (*SystemManager, error) {
	renderer, err := NewRendererSystem(appName, width, height, platform)
	if err != nil {
		return nil, err
	}

	js, err := NewJobSystem(MaxNumberOfWorkers, 25)
	if err != nil {
		return nil, err
	}

	cs, err := NewCameraSystem(&CameraSystemConfig{
		MaxCameraCount: 61,
	})
	if err != nil {
		return nil, err
	}
	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	rs, err := NewResourceSystem(&ResourceSystemConfig{
		MaxLoaderCount: 32,
		AssetBasePath:  fmt.Sprintf("%s/assets", wd),
	})
	if err != nil {
		return nil, err
	}
	ts, err := NewTextureSystem(&TextureSystemConfig{
		MaxTextureCount: 65536,
	}, js, rs, renderer)
	if err != nil {
		return nil, err
	}
	ssys, err := NewShaderSystem(&ShaderSystemConfig{
		MaxShaderCount:      1024,
		MaxUniformCount:     uint8(128),
		MaxGlobalTextures:   uint8(31),
		MaxInstanceTextures: uint8(31),
	}, ts, renderer)
	if err != nil {
		return nil, err
	}
	ms, err := NewMaterialSystem(&MaterialSystemConfig{
		MaxMaterialCount: 4096,
	}, ssys, ts, rs, renderer)
	if err != nil {
		return nil, err
	}
	gs, err := NewGeometrySystem(&GeometrySystemConfig{
		MaxGeometryCount: 4096,
	}, ms, renderer)
	if err != nil {
		return nil, err
	}
	mls, err := NewMeshLoaderSystem(gs, rs)
	if err != nil {
		return nil, err
	}
	rvs, err := NewRenderViewSystem(RenderViewSystemConfig{
		MaxViewCount: 251,
	}, renderer)
	if err != nil {
		return nil, err
	}
	return &SystemManager{
		RendererSystem:   renderer,
		CameraSystem:     cs,
		JobSystem:        js,
		TextureSystem:    ts,
		ShaderSystem:     ssys,
		MaterialSystem:   ms,
		GeometrySystem:   gs,
		MeshLoaderSystem: mls,
		ResourceSystem:   rs,
		RenderViewSystem: rvs,
	}, nil
}

func (sm *SystemManager) Initialize() error {
	if err := sm.RendererSystem.Initialize(sm.ResourceSystem, sm.ShaderSystem); err != nil {
		return err
	}
	return nil
}

func (sm *SystemManager) DrawFrame(renderPacket *metadata.RenderPacket) error {
	if err := sm.RendererSystem.DrawFrame(renderPacket, sm.RenderViewSystem); err != nil {
		return err
	}
	return nil
}

func (sm *SystemManager) OnResize(width, height uint16) error {
	if err := sm.RendererSystem.OnResize(width, height); err != nil {
		return err
	}
	return nil
}

func (sm *SystemManager) RenderViewCreate(config *metadata.RenderViewConfig) error {
	if !sm.RenderViewSystem.Create(config) {
		err := fmt.Errorf("failed to create the renderview with name `%s`", config.Name)
		return err
	}
	return nil
}

func (sm *SystemManager) Shutdown() error {
	if err := sm.RenderViewSystem.Shutdown(); err != nil {
		return err
	}
	if err := sm.MeshLoaderSystem.Shutdown(); err != nil {
		return err
	}
	if err := sm.GeometrySystem.Shutdown(); err != nil {
		return err
	}
	if err := sm.MaterialSystem.Shutdown(); err != nil {
		return err
	}
	if err := sm.ShaderSystem.Shutdown(); err != nil {
		return err
	}
	if err := sm.TextureSystem.Shutdown(); err != nil {
		return err
	}
	if err := sm.ResourceSystem.Shutdown(); err != nil {
		return err
	}
	if err := sm.CameraSystem.Shutdown(); err != nil {
		return err
	}
	if err := sm.JobSystem.Shutdown(); err != nil {
		return err
	}
	return nil
}
