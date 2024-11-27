package systems

import (
	"runtime"

	"github.com/spaghettifunk/anima/engine/assets"
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
	RendererSystem   *RendererSystem
	FontSystem       *FontSystem
	AssetManager     *assets.AssetManager
}

var (
	MaxNumberOfWorkers int = runtime.NumCPU()
)

func NewSystemManager(appName string, width, height uint32, platform *platform.Platform, am *assets.AssetManager) (*SystemManager, error) {
	renderer, err := NewRendererSystem(appName, width, height, platform, am)
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

	ts, err := NewTextureSystem(&TextureSystemConfig{
		MaxTextureCount: 65536,
	}, js, am, renderer)
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
	}, ssys, ts, am, renderer)
	if err != nil {
		return nil, err
	}

	fs, err := NewFontSystem(&FontSystemConfig{
		AutoRelease:            false,
		DefaultBitmapFontCount: 1,
		DefaultSystemFontCount: 1,
		MaxSystemFontCount:     101,
		MaxBitmapFontCount:     101,
		BitmapFontConfigs: []*metadata.BitmapFontConfig{
			{
				Name:         "Ubuntu Mono 21px",
				ResourceName: "UbuntuMono21px",
				Size:         21,
			},
		},
		SystemFontConfigs: []*metadata.SystemFontConfig{
			{
				DefaultSize:  20,
				Name:         "Noto Sans",
				ResourceName: "NotoSansCJK",
			},
		},
	}, ts, ssys, am, renderer)
	if err != nil {
		return nil, err
	}

	rvs, err := NewRenderViewSystem(RenderViewSystemConfig{
		MaxViewCount: 251,
	}, renderer, ssys, cs, ms, fs)
	if err != nil {
		return nil, err
	}

	gs, err := NewGeometrySystem(&GeometrySystemConfig{
		MaxGeometryCount: 4096,
	}, ms, renderer)
	if err != nil {
		return nil, err
	}

	mls, err := NewMeshLoaderSystem(gs, am)
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
		RenderViewSystem: rvs,
		FontSystem:       fs,
		AssetManager:     am,
	}, nil
}

func (sm *SystemManager) Initialize() error {
	if err := sm.RendererSystem.Initialize(sm.ShaderSystem, sm.RenderViewSystem); err != nil {
		return err
	}
	if err := sm.TextureSystem.Initialize(); err != nil {
		return err
	}
	if err := sm.MaterialSystem.Initialize(); err != nil {
		return err
	}
	if err := sm.GeometrySystem.Initialize(); err != nil {
		return err
	}
	// if err := sm.FontSystem.Initialize(); err != nil {
	// 	return err
	// }
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

func (sm *SystemManager) Shutdown() error {
	if err := sm.RenderViewSystem.Shutdown(); err != nil {
		return err
	}
	// if err := sm.FontSystem.Shutdown(); err != nil {
	// 	return err
	// }
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
	if err := sm.CameraSystem.Shutdown(); err != nil {
		return err
	}
	if err := sm.JobSystem.Shutdown(); err != nil {
		return err
	}
	return nil
}
