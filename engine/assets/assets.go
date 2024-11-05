package assets

import (
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/spaghettifunk/anima/engine/assets/loaders"
	"github.com/spaghettifunk/anima/engine/core"
	"github.com/spaghettifunk/anima/engine/renderer/metadata"
)

type AssetInfo struct {
	Path       string
	Type       metadata.ResourceType
	LastLoaded time.Time
}

type AssetManager struct {
	assets   map[string]AssetInfo
	loaders  map[metadata.ResourceType]Loader
	mutex    sync.RWMutex
	watcher  *fsnotify.Watcher
	watchDir string
}

func NewAssetManager() *AssetManager {
	return &AssetManager{
		assets:  make(map[string]AssetInfo),
		loaders: make(map[metadata.ResourceType]Loader),
	}
}

func (am *AssetManager) Initialize(assetsDir string) error {
	go func() {
		if err := am.startWatching(assetsDir); err != nil {
			core.LogError(err.Error())
		}
	}()

	// Register loaders
	am.registerLoader(metadata.ResourceTypeShader, &loaders.ShaderLoader{})
	am.registerLoader(metadata.ResourceTypeImage, &loaders.TextureLoader{})

	return nil
}

// Register loaders for each asset type
func (am *AssetManager) registerLoader(assetType metadata.ResourceType, loader Loader) {
	am.loaders[assetType] = loader
}

// Load an asset using the appropriate loader
func (am *AssetManager) LoadAsset(path string, resourceType metadata.ResourceType, params interface{}) (*metadata.Resource, error) {
	am.mutex.RLock()
	asset, exists := am.assets[path]
	am.mutex.RUnlock()
	if !exists {
		return nil, fmt.Errorf("asset not found: %s", path)
	}
	// Load or reload asset from disk if necessary
	asset.LastLoaded = time.Now()
	am.assets[path] = asset // Update the loaded time

	loader, loaderExists := am.loaders[asset.Type]
	if !loaderExists {
		return nil, fmt.Errorf("no loader registered for asset type: %d", asset.Type)
	}

	return loader.Load(path, resourceType, params)
}

func (am *AssetManager) UnloadAsset(asset *metadata.Resource) error {
	return nil
}

// Initialize the file watcher and start watching in a separate goroutine
func (am *AssetManager) startWatching(dir string) error {
	am.watchDir = dir
	var err error
	am.watcher, err = fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	// Start the watcher in a separate goroutine
	go am.watchFiles()

	// Add directory to watcher
	err = am.watcher.Add(dir)
	if err != nil {
		return err
	}

	return nil
}

// Watch for file changes and update the asset index
func (am *AssetManager) watchFiles() {
	defer am.watcher.Close()

	for {
		select {
		case event, ok := <-am.watcher.Events:
			if !ok {
				return
			}
			// Handle create or modify events
			if event.Op&(fsnotify.Create|fsnotify.Write) != 0 {
				am.handleFileEvent(event.Name)
			}
			// Handle remove events
			if event.Op&fsnotify.Remove != 0 {
				am.removeAsset(event.Name)
			}
		case err, ok := <-am.watcher.Errors:
			if !ok {
				return
			}
			core.LogError(err.Error())
		}
	}
}

// Handle the creation or modification of a file
func (am *AssetManager) handleFileEvent(path string) {
	am.mutex.Lock()
	defer am.mutex.Unlock()

	assetType := determineAssetType(path)
	am.assets[path] = AssetInfo{
		Path:       path,
		Type:       assetType,
		LastLoaded: time.Now(),
	}
}

// Remove the asset from the index if it was deleted
func (am *AssetManager) removeAsset(path string) {
	am.mutex.Lock()
	defer am.mutex.Unlock()

	delete(am.assets, path)
}

func determineAssetType(path string) metadata.ResourceType {
	switch filepath.Ext(path) {
	case ".spv":
		return metadata.ResourceTypeShader
	case ".png", ".jpg":
		return metadata.ResourceTypeImage
	default:
		return metadata.ResourceTypeCustom
	}
}
