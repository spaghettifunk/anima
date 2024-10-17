package systems

import (
	"fmt"
	"sync"

	"github.com/spaghettifunk/anima/engine/core"
	"github.com/spaghettifunk/anima/engine/renderer/components"
	"github.com/spaghettifunk/anima/engine/renderer/metadata"
	"github.com/spaghettifunk/anima/engine/resources/loaders"
)

type cameraSystemState struct {
	Config  CameraSystemConfig
	Lookup  map[string]uint16
	Cameras []*components.CameraLookup
	// A default, non-registered camera that always exists as a fallback.
	DefaultCamera *components.Camera
}

/** @brief The camera system configuration. */
type CameraSystemConfig struct {
	/**
	 * @brief NOTE: The maximum number of cameras that can be managed by
	 * the system.
	 */
	MaxCameraCount uint16
}

var onceCameraSystem sync.Once
var csState *cameraSystemState

/**
 * @brief Initializes the camera system.
 * Should be called twice; once to get the memory requirement (passing state=0), and a second
 * time passing an allocated block of memory to actually initialize the system.
 *
 * @param memory_requirement A pointer to hold the memory requirement as it is calculated.
 * @param state A block of memory to hold the state or, if gathering the memory requirement, 0.
 * @param config The configuration for this system.
 * @return True on success; otherwise false.
 */
func NewCameraSystem(config CameraSystemConfig) bool {
	if config.MaxCameraCount == 0 {
		core.LogError("CameraSystem_initialize - config.MaxCameraCount must be > 0.")
		return false
	}
	onceCameraSystem.Do(func() {
		csState = &cameraSystemState{
			Config:  config,
			Cameras: make([]*components.CameraLookup, 1),
			Lookup:  make(map[string]uint16, config.MaxCameraCount),
		}
		// Invalidate all cameras in the array.
		for i := uint16(0); i < csState.Config.MaxCameraCount; i++ {
			csState.Lookup[metadata.GenerateNewHash()] = loaders.InvalidIDUint16
			csState.Cameras[i].ID = loaders.InvalidIDUint16
			csState.Cameras[i].ReferenceCount = 0
		}
		// Setup default camera.
		csState.DefaultCamera = components.NewCamera()
	})
	return true
}

/**
 * @brief Shuts down the geometry camera.
 *
 * @param state The state block of memory.
 */
func CameraSystemShutdown() {
	csState = nil
}

/**
 * @brief Acquires a pointer to a camera by name.
 * If one is not found, a new one is created and retuned.
 * Internal reference counter is incremented.
 *
 * @param name The name of the camera to acquire.
 * @return A pointer to a camera if successful; 0 if an error occurs.
 */
func CameraSystemAcquire(name string) (*components.Camera, error) {
	if name == components.DEFAULT_CAMERA_NAME {
		return csState.DefaultCamera, nil
	}
	id, ok := csState.Lookup[name]
	if !ok {
		err := fmt.Errorf("CameraSystemAcquire failed lookup. Null returned.")
		core.LogError(err.Error())
		return nil, err
	}

	if id == loaders.InvalidIDUint16 {
		// Find free slot
		for i := uint16(0); i < csState.Config.MaxCameraCount; i++ {
			if i == loaders.InvalidIDUint16 {
				id = i
				break
			}
		}
		if id == loaders.InvalidIDUint16 {
			err := fmt.Errorf("CameraSystemAcquire failed to acquire new slot. Adjust camera system config to allow more. Null returned.")
			core.LogError(err.Error())
			return nil, err
		}

		// Create/register the new camera.
		core.LogDebug("Creating new camera named '%s'...")
		csState.Cameras[id].Camera = components.NewCamera()
		csState.Cameras[id].ID = id

		// Update the hashtable.
		csState.Lookup[name] = id
	}
	csState.Cameras[id].ReferenceCount++
	return csState.Cameras[id].Camera, nil
}

/**
 * @brief Releases a camera with the given name. Intenral reference
 * counter is decremented. If this reaches 0, the camera is reset,
 * and the reference is usable by a new camera.
 *
 * @param name The name of the camera to release.
 */
func CameraSystemRelease(name string) {
	if name == components.DEFAULT_CAMERA_NAME {
		core.LogDebug("Cannot release default camera. Nothing was done.")
		return
	}
	id, ok := csState.Lookup[name]
	if !ok {
		core.LogWarn("CameraSystemRelease failed lookup. Nothing was done.")
	}
	if id != loaders.InvalidIDUint16 {
		// Decrement the reference count, and reset the camera if the counter reaches 0.
		csState.Cameras[id].ReferenceCount--
		if csState.Cameras[id].ReferenceCount < 1 {
			csState.Cameras[id].Camera.Reset()
			csState.Cameras[id].ID = loaders.InvalidIDUint16
			csState.Lookup[name] = csState.Cameras[id].ID
		}
	}
}

/**
 * @brief Gets a pointer to the default camera.
 *
 * @return A pointer to the default camera.
 */
func CameraSystemGetDefault() *components.Camera {
	return csState.DefaultCamera
}
