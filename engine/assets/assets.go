package assets

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
	assets  map[string]*AssetInfo
	loaders map[metadata.ResourceType]Loader

	mutex sync.RWMutex

	done     chan struct{}
	fsnotify *fsnotify.Watcher
	isClosed bool
	events   chan fsnotify.Event
	errors   chan error
}

func NewAssetManager() (*AssetManager, error) {
	fsWatch, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	return &AssetManager{
		assets:   make(map[string]*AssetInfo),
		loaders:  make(map[metadata.ResourceType]Loader),
		fsnotify: fsWatch,
		events:   make(chan fsnotify.Event),
		errors:   make(chan error),
		done:     make(chan struct{}),
	}, nil
}

func (am *AssetManager) Initialize(assetsDir string) error {
	go am.start()

	if err := am.addRecursive(assetsDir); err != nil {
		return err
	}

	// Register loaders
	am.registerLoader(metadata.ResourceTypeShader, &loaders.ShaderLoader{})
	am.registerLoader(metadata.ResourceTypeBinary, &loaders.BinaryLoader{})
	am.registerLoader(metadata.ResourceTypeImage, &loaders.ImageLoader{})
	am.registerLoader(metadata.ResourceTypeMaterial, &loaders.MaterialLoader{})
	am.registerLoader(metadata.ResourceTypeBitmapFont, &loaders.BitmapFontLoader{
		ResourcePath: assetsDir,
	})
	am.registerLoader(metadata.ResourceTypeSystemFont, &loaders.SystemFontLoader{})

	return nil
}

// Add starts watching the named file or directory (non-recursively).
func (am *AssetManager) add(name string) error {
	if am.isClosed {
		return errors.New("rfsnotify instance already closed")
	}
	return am.fsnotify.Add(name)
}

// AddRecursive starts watching the named directory and all sub-directories.
func (am *AssetManager) addRecursive(name string) error {
	if am.isClosed {
		return errors.New("rfsnotify instance already closed")
	}
	if err := am.watchRecursive(name, false); err != nil {
		return err
	}
	return nil
}

// Remove stops watching the the named file or directory (non-recursively).
func (am *AssetManager) remove(name string) error {
	return am.fsnotify.Remove(name)
}

// RemoveRecursive stops watching the named directory and all sub-directories.
func (am *AssetManager) removeRecursive(name string) error {
	if err := am.watchRecursive(name, true); err != nil {
		return err
	}
	return nil
}

// Register loaders for each asset type
func (am *AssetManager) registerLoader(assetType metadata.ResourceType, loader Loader) {
	am.loaders[assetType] = loader
}

var imageExtensions = []string{".tga", ".png", ".jpg", ".bmp"}

// Load an asset using the appropriate loader
func (am *AssetManager) LoadAsset(filename string, resourceType metadata.ResourceType, params interface{}) (*metadata.Resource, error) {
	var asset *AssetInfo
	var path string
	switch resourceType {
	case metadata.ResourceTypeImage:
		found := false
		for i := 0; i < len(imageExtensions); i++ {
			path = fmt.Sprintf("assets/textures/%s%s", filename, imageExtensions[i])
			asset = am.assetExists(path)
			if asset != nil {
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("asset with name %s not found", filename)
		}
	case metadata.ResourceTypeShader:
		path = fmt.Sprintf("assets/shaders/%s.shadercfg", filename)
		asset = am.assetExists(path)
	case metadata.ResourceTypeBinary:
		path = fmt.Sprintf("assets/%s", filename)
		params = map[string]string{
			"name": filename,
		}
		asset = am.assetExists(path)
	case metadata.ResourceTypeMaterial:
		path = fmt.Sprintf("assets/materials/%s.amt", filename)
		asset = am.assetExists(path)
	case metadata.ResourceTypeSystemFont:
		path = fmt.Sprintf("assets/fonts/%s.fontcfg", filename)
		asset = am.assetExists(path)
	case metadata.ResourceTypeBitmapFont:
		path = fmt.Sprintf("assets/fonts/%s.fnt", filename)
		asset = am.assetExists(path)
	default:
		err := fmt.Errorf("unknown resource type")
		return nil, err
	}

	loader, loaderExists := am.loaders[asset.Type]
	if !loaderExists {
		return nil, fmt.Errorf("no loader registered for asset type: %d", asset.Type)
	}

	return loader.Load(path, resourceType, params)
}

func (am *AssetManager) assetExists(path string) *AssetInfo {
	am.mutex.RLock()
	asset, exists := am.assets[path]
	am.mutex.RUnlock()
	if !exists {
		return nil
	}
	// Load or reload asset from disk if necessary
	asset.LastLoaded = time.Now()
	am.assets[path] = asset // Update the loaded time

	return asset
}

func (am *AssetManager) UnloadAsset(asset *metadata.Resource) error {
	return nil
}

func (am *AssetManager) start() {
	for {
		select {

		case e := <-am.fsnotify.Events:
			s, err := os.Stat(e.Name)
			if err == nil && s != nil && s.IsDir() {
				if e.Op&fsnotify.Create != 0 {
					am.watchRecursive(e.Name, false)
				}
			}
			// Handle create or modify events
			if e.Op&(fsnotify.Create|fsnotify.Write) != 0 {
				am.handleFileEvent(e.Name)
			}
			//Can't stat a deleted directory, so just pretend that it's always a directory and
			//try to remove from the watch list...  we really have no clue if it's a directory or not...
			if e.Op&fsnotify.Remove != 0 {
				am.removeAsset(e.Name)
				am.fsnotify.Remove(e.Name)
			}
			am.events <- e

		case e := <-am.fsnotify.Errors:
			am.errors <- e
			core.LogError(e.Error())

		case <-am.done:
			am.fsnotify.Close()
			close(am.events)
			close(am.errors)
			return
		}
	}
}

// watchRecursive adds all directories under the given one to the watch list.
// this is probably a very racey process. What if a file is added to a folder before we get the watch added?
func (am *AssetManager) watchRecursive(path string, unWatch bool) error {
	wd, err := os.Getwd()
	if err != nil {
		return err
	}
	wd = wd + "/" // add trailing slash
	err = filepath.Walk(path, func(walkPath string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if fi.IsDir() {
			if unWatch {
				if err = am.fsnotify.Remove(walkPath); err != nil {
					return err
				}
			} else {
				am.mutex.RLock()
				if err = am.fsnotify.Add(walkPath); err != nil {
					return err
				}
				am.mutex.RUnlock()
			}
		} else {
			p := strings.TrimPrefix(walkPath, wd)
			am.handleFileEvent(p)
		}
		return nil
	})
	return err
}

// Handle the creation or modification of a file
func (am *AssetManager) handleFileEvent(path string) {
	am.mutex.Lock()
	defer am.mutex.Unlock()

	assetType := determineAssetType(path)
	if assetType == metadata.ResourceTypeNone {
		return
	}
	am.assets[path] = &AssetInfo{
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
	case ".shadercfg":
		return metadata.ResourceTypeShader
	case ".fontcfg", ".ksf":
		return metadata.ResourceTypeSystemFont
	case ".fnt", ".kbf":
		return metadata.ResourceTypeBitmapFont
	case ".spv":
		return metadata.ResourceTypeBinary
	case ".png", ".jpg", ".tga":
		return metadata.ResourceTypeImage
	case ".obj", ".ksm":
		return metadata.ResourceTypeModel
	case ".amt":
		return metadata.ResourceTypeMaterial
	default:
		return metadata.ResourceTypeNone
	}
}
