package systems

import (
	"github.com/spaghettifunk/anima/engine/renderer"
)

type SystemManager struct {
	cameraSystem     *CameraSystem
	geometrySystem   *GeometrySystem
	jobSystem        *JobSystem
	materialSystem   *MaterialSystem
	meshLoaderSystem *MeshLoaderSystem
	renderViewSystem *RenderViewSystem
	shaderSystem     *ShaderSystem
	textureSystem    *TextureSystem
	resourceSystem   *ResourceSystem
}

func NewSystemManager(renderer *renderer.Renderer) (*SystemManager, error) {
	// TODO: remake the jobsystem
	js, err := NewJobSystem(1, nil)
	if err != nil {
		return nil, err
	}

	cs, err := NewCameraSystem(&CameraSystemConfig{
		MaxCameraCount: 100,
	})
	if err != nil {
		return nil, err
	}
	rs, err := NewResourceSystem(&ResourceSystemConfig{
		MaxLoaderCount: 1000,
		AssetBasePath:  "", // TODO: add correct path here
	})
	if err != nil {
		return nil, err
	}
	ts, err := NewTextureSystem(&TextureSystemConfig{
		MaxTextureCount: 1000,
	}, js, rs, renderer)
	if err != nil {
		return nil, err
	}
	ssys, err := NewShaderSystem(&ShaderSystemConfig{
		MaxShaderCount:      1000,
		MaxUniformCount:     uint8(255),
		MaxGlobalTextures:   uint8(255),
		MaxInstanceTextures: uint8(255),
	}, ts, renderer)
	if err != nil {
		return nil, err
	}
	ms, err := NewMaterialSystem(&MaterialSystemConfig{
		MaxMaterialCount: 1000,
	}, ssys, ts, rs, renderer)
	if err != nil {
		return nil, err
	}
	gs, err := NewGeometrySystem(&GeometrySystemConfig{
		MaxGeometryCount: 1000,
	}, ms, renderer)
	if err != nil {
		return nil, err
	}
	mls, err := NewMeshLoaderSystem(gs, rs)
	if err != nil {
		return nil, err
	}
	rvs, err := NewRenderViewSystem(RenderViewSystemConfig{
		MaxViewCount: 1000,
	}, renderer)
	if err != nil {
		return nil, err
	}
	return &SystemManager{
		cameraSystem:     cs,
		jobSystem:        js,
		textureSystem:    ts,
		shaderSystem:     ssys,
		materialSystem:   ms,
		geometrySystem:   gs,
		meshLoaderSystem: mls,
		resourceSystem:   rs,
		renderViewSystem: rvs,
	}, nil
}

func (sm *SystemManager) Shutdown() error {
	if err := sm.renderViewSystem.Shutdown(); err != nil {
		return err
	}
	if err := sm.meshLoaderSystem.Shutdown(); err != nil {
		return err
	}
	if err := sm.geometrySystem.Shutdown(); err != nil {
		return err
	}
	if err := sm.materialSystem.Shutdown(); err != nil {
		return err
	}
	if err := sm.shaderSystem.Shutdown(); err != nil {
		return err
	}
	if err := sm.textureSystem.Shutdown(); err != nil {
		return err
	}
	if err := sm.resourceSystem.Shutdown(); err != nil {
		return err
	}
	if err := sm.cameraSystem.Shutdown(); err != nil {
		return err
	}
	if err := sm.jobSystem.Shutdown(); err != nil {
		return err
	}
	return nil
}
