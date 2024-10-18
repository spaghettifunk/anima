package systems

import (
	"fmt"
	"sync"

	"github.com/spaghettifunk/anima/engine/core"
	"github.com/spaghettifunk/anima/engine/renderer/metadata"
	"github.com/spaghettifunk/anima/engine/resources/loaders"
)

/** @brief The configuration for the resource system */
type ResourceSystemConfig struct {
	/** @brief The maximum number of loaders that can be registered with this system. */
	MaxLoaderCount uint32
	/** @brief The relative base path for assets. */
	AssetBasePath string
}

type resourceSystemState struct {
	Config            ResourceSystemConfig
	RegisteredLoaders []loaders.ResourceLoader
}

var onceResourceSystemState sync.Once
var rsState *resourceSystemState

func NewResourceSystem(config ResourceSystemConfig) error {
	if config.MaxLoaderCount == 0 {
		err := fmt.Errorf("failed to run NewResourceSystem because config.MaxLoaderCount==0")
		core.LogFatal(err.Error())
		return err
	}

	onceResourceSystemState.Do(func() {
		rsState = &resourceSystemState{
			Config:            config,
			RegisteredLoaders: make([]loaders.ResourceLoader, config.MaxLoaderCount),
		}

		// Invalidate all loaders
		for i := uint32(0); i < config.MaxLoaderCount; i++ {
			rsState.RegisteredLoaders[i].ID = loaders.InvalidID
		}

		// NOTE: Auto-register known loader types here.
		// rsState.RegisterLoader(binary_resource_loader_create())
		// rsState.RegisterLoader(image_resource_loader_create())
		// rsState.RegisterLoader(material_resource_loader_create())
		// rsState.RegisterLoader(shader_resource_loader_create())
		// rsState.RegisterLoader(mesh_resource_loader_create())
	})

	core.LogInfo("Resource system initialized with base path '%s'.", config.AssetBasePath)

	return nil
}

func ResourceSystemShutdown() {
}

func ResourceSystemRegisterLoader(loader loaders.ResourceLoader) bool {
	count := rsState.Config.MaxLoaderCount
	// Ensure no loaders for the given type already exist
	for i := uint32(0); i < count; i++ {
		l := rsState.RegisteredLoaders[i]
		if l.ID != loaders.InvalidID {
			if l.ResourceType == loader.ResourceType {
				core.LogError("resource_system_register_loader - Loader of type %d already exists and will not be registered.", loader.ResourceType)
				return false
			} else if len(loader.CustomType) > 0 && l.CustomType == loader.CustomType {
				core.LogError("resource_system_register_loader - Loader of custom type %s already exists and will not be registered.", loader.CustomType)
				return false
			}
		}
	}
	for i := uint32(0); i < count; i++ {
		if rsState.RegisteredLoaders[i].ID == loaders.InvalidID {
			rsState.RegisteredLoaders[i] = loader
			rsState.RegisteredLoaders[i].ID = i
			core.LogDebug("Loader registered.")
			return true
		}
	}

	return false
}

func ResourceSystemLoad(name string, resourceType metadata.ResourceType, params interface{}) (*metadata.Resource, error) {
	outResource := &metadata.Resource{}
	if resourceType != metadata.ResourceTypeCustom {
		// Select loader.
		count := rsState.Config.MaxLoaderCount
		for i := uint32(0); i < count; i++ {
			l := rsState.RegisteredLoaders[i]
			if l.ID != loaders.InvalidID && l.ResourceType == resourceType {
				return l.Load(name, params)
			}
		}
	}

	outResource.LoaderID = loaders.InvalidID
	core.LogError("resource_system_load - No loader for type %d was found.", resourceType)

	return outResource, nil
}

func ResourceSystemLoadCustom(name, custom_type string, params interface{}) (*metadata.Resource, error) {
	outResource := &metadata.Resource{}
	if len(custom_type) > 0 {
		// Select loader.
		count := rsState.Config.MaxLoaderCount
		for i := uint32(0); i < count; i++ {
			l := rsState.RegisteredLoaders[i]
			if l.ID != loaders.InvalidID && l.ResourceType == metadata.ResourceTypeCustom && l.CustomType == custom_type {
				return load(name, l, params)
			}
		}
	}

	outResource.LoaderID = loaders.InvalidID
	core.LogError("resource_system_load_custom - No loader for type %s was found.", custom_type)
	return outResource, nil
}

func ResourceSystemUnload(resource *metadata.Resource) error {
	if resource != nil {
		if resource.LoaderID != loaders.InvalidID {
			l := rsState.RegisteredLoaders[resource.LoaderID]
			if l.ID != loaders.InvalidID {
				if err := l.Unload(resource); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// TODO: when do I need to use this???
func load(name string, loader loaders.ResourceLoader, params interface{}) (*metadata.Resource, error) {
	err := fmt.Errorf("function `load` not implemented")
	core.LogFatal(err.Error())
	return nil, err
}
