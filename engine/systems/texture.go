package systems

import (
	"fmt"

	"github.com/spaghettifunk/anima/engine/assets"
	"github.com/spaghettifunk/anima/engine/core"
	"github.com/spaghettifunk/anima/engine/renderer/metadata"
)

type TextureSystemConfig struct {
	/** @brief The maximum number of textures that can be loaded at once. */
	MaxTextureCount uint32
}

type TextureSystem struct {
	Config         *TextureSystemConfig
	DefaultTexture *metadata.DefaultTexture
	// Array of registered textures.
	RegisteredTextures []*metadata.Texture
	// Hashtable for texture lookups.
	RegisteredTextureTable map[string]*metadata.TextureReference
	// sub systems
	jobSystem    *JobSystem
	assetManager *assets.AssetManager
	renderer     *RendererSystem
}

func NewTextureSystem(config *TextureSystemConfig, js *JobSystem, am *assets.AssetManager, r *RendererSystem) (*TextureSystem, error) {
	if config.MaxTextureCount == 0 {
		err := fmt.Errorf("func NewTextureSystem - config.MaxTextureCount must be > 0")
		core.LogFatal(err.Error())
		return nil, err
	}

	ts := &TextureSystem{
		Config:                 config,
		RegisteredTextures:     make([]*metadata.Texture, config.MaxTextureCount),
		RegisteredTextureTable: make(map[string]*metadata.TextureReference),
		DefaultTexture:         metadata.NewDefaultTexture(),
		jobSystem:              js,
		assetManager:           am,
		renderer:               r,
	}

	// Invalidate all textures in the array.
	for i := uint32(0); i < config.MaxTextureCount; i++ {
		ts.RegisteredTextures[i] = &metadata.Texture{
			ID:         metadata.InvalidID,
			Generation: metadata.InvalidID,
		}
	}

	// Create default textures for use in the system.
	ts.DefaultTexture.CreateSkeletonTextures()

	return ts, nil
}

func (ts *TextureSystem) Initialize() error {
	ts.renderer.TextureCreate(ts.DefaultTexture.TexturePixels, ts.DefaultTexture.DefaultTexture)
	ts.renderer.TextureCreate(ts.DefaultTexture.DiffuseTexturePixels, ts.DefaultTexture.DefaultDiffuseTexture)
	ts.renderer.TextureCreate(ts.DefaultTexture.SpecularTexturePixels, ts.DefaultTexture.DefaultSpecularTexture)
	ts.renderer.TextureCreate(ts.DefaultTexture.NormalTexturePixels, ts.DefaultTexture.DefaultNormalTexture)

	return nil
}

func (ts *TextureSystem) Shutdown() error {
	// Destroy all loaded textures.
	for i := uint32(0); i < ts.Config.MaxTextureCount; i++ {
		t := ts.RegisteredTextures[i]
		if t.Generation != metadata.InvalidID {
			if err := ts.renderer.TextureDestroy(t); err != nil {
				return err
			}
		}
	}
	if err := ts.renderer.TextureDestroy(ts.DefaultTexture.DefaultTexture); err != nil {
		return err
	}
	if err := ts.renderer.TextureDestroy(ts.DefaultTexture.DefaultDiffuseTexture); err != nil {
		return err
	}
	if err := ts.renderer.TextureDestroy(ts.DefaultTexture.DefaultSpecularTexture); err != nil {
		return err
	}
	if err := ts.renderer.TextureDestroy(ts.DefaultTexture.DefaultNormalTexture); err != nil {
		return err
	}

	return nil
}

func (ts *TextureSystem) Aquire(name string, autoRelease bool) (*metadata.Texture, error) {
	// Return default texture, but warn about it since this should be returned via get_default_texture();
	// TODO: Check against other default texture names?
	if name == metadata.DEFAULT_TEXTURE_NAME {
		core.LogWarn("func texture system Acquire called for default texture. Use texture_system_get_default_texture for texture 'default'")
		return ts.DefaultTexture.DefaultTexture, nil
	}
	// NOTE: Increments reference count, or creates new entry.
	id, ok := ts.ProcessTextureReference(name, metadata.TextureType2d, 1, autoRelease, false)
	if !ok {
		err := fmt.Errorf("func texture system Acquire failed to obtain a new texture id")
		core.LogError(err.Error())
		return nil, err
	}
	return ts.RegisteredTextures[id], nil
}

/**
 * @brief Attempts to acquire a cubemap texture with the given name. If it has not yet been loaded,
 * this triggers it to load. If the texture is not found, a pointer to the default texture
 * is returned. If the texture _is_ found and loaded, its reference counter is incremented.
 * Requires textures with name as the base, one for each side of a cube, in the following order:
 * - name_f Front
 * - name_b Back
 * - name_u Up
 * - name_d Down
 * - name_r Right
 * - name_l Left
 *
 * For example, "skybox_f.png", "skybox_b.png", etc. where name is "skybox".
 *
 */
func (ts *TextureSystem) AquireCube(name string, autoRelease bool) (*metadata.Texture, error) {
	// Return default texture, but warn about it since this should be returned via get_default_texture();
	// TODO: Check against other default texture names?
	if name == metadata.DEFAULT_TEXTURE_NAME {
		core.LogWarn("func texture system AcquireCube called for default texture. Use texture_system_get_default_texture for texture 'default'")
		return ts.DefaultTexture.DefaultTexture, nil
	}
	// NOTE: Increments reference count, or creates new entry.
	id, ok := ts.ProcessTextureReference(name, metadata.TextureTypeCube, 1, autoRelease, false)
	if !ok {
		err := fmt.Errorf("func texture system AcquireCube failed to obtain a new texture id")
		core.LogError(err.Error())
		return nil, err
	}
	return ts.RegisteredTextures[id], nil
}

func (ts *TextureSystem) AquireWriteable(name string, width, height uint32, channelCount uint8, hasTransparency bool) (*metadata.Texture, error) {
	// NOTE: Wrapped textures are never auto-released because it means that thier
	// resources are created and managed somewhere within the renderer internals.
	id, ok := ts.ProcessTextureReference(name, metadata.TextureType2d, 1, false, true)
	if !ok {
		err := fmt.Errorf("func texture system Acquire failed to obtain a new texture id")
		core.LogError(err.Error())
		return nil, err
	}

	texture := ts.RegisteredTextures[id]
	texture.ID = id
	texture.TextureType = metadata.TextureType2d
	texture.Name = name
	texture.Width = width
	texture.Height = height
	texture.ChannelCount = channelCount
	texture.Generation = metadata.InvalidID

	texture.Flags |= metadata.TextureFlagBits(metadata.TextureFlagHasTransparency)
	if hasTransparency {
		texture.Flags |= 0
	}

	texture.Flags |= metadata.TextureFlagBits(metadata.TextureFlagIsWriteable)
	texture.InternalData = nil

	ts.renderer.TextureCreateWriteable(texture)

	return texture, nil
}

func (ts *TextureSystem) Release(name string) {
	// Ignore release requests for the default texture.
	// TODO: Check against other default texture names as well?
	if name == metadata.DEFAULT_TEXTURE_NAME {
		return
	}
	// NOTE: Decrement the reference count.
	id, ok := ts.ProcessTextureReference(name, metadata.TextureType2d, -1, false, false)
	if !ok {
		core.LogError("texture_system_release failed to release texture '%s' properly.", name)
	}
	core.LogDebug("texture ID `%d` released", id)
}

func (ts *TextureSystem) SetInternal(texture *metadata.Texture, internalData interface{}) bool {
	if texture != nil {
		texture.InternalData = internalData
		texture.Generation++
		return true
	}
	return false
}

func (ts *TextureSystem) WriteData(texture *metadata.Texture, offset, size uint32, data interface{}) bool {
	if texture != nil {
		// Type assertion to []uint8
		pixels, ok := data.([]uint8)
		if !ok {
			core.LogError("Failed to cast to []uint8")
			return false
		}
		ts.renderer.TextureWriteData(texture, offset, size, pixels)
		return true
	}
	return false
}

func (ts *TextureSystem) GetDefaultTexture() *metadata.Texture {
	return ts.DefaultTexture.DefaultTexture
}

func (ts *TextureSystem) GetDefaultDiffuseTexture() *metadata.Texture {
	return ts.DefaultTexture.DefaultDiffuseTexture
}

func (ts *TextureSystem) GetDefaultSpecularTexture() *metadata.Texture {
	return ts.DefaultTexture.DefaultSpecularTexture
}

func (ts *TextureSystem) GetDefaultNormalTexture() *metadata.Texture {
	return ts.DefaultTexture.DefaultNormalTexture
}

func (ts *TextureSystem) LoadTexture(textureName string, texture *metadata.Texture) bool {
	ts.jobSystem.Submit(metadata.JobTask{
		JobType:  metadata.JOB_TYPE_GENERAL,
		Priority: metadata.JOB_PRIORITY_NORMAL,
		InputParams: []interface{}{
			// Kick off a texture loading job. Only handles loading from disk
			// to CPU. GPU upload is handled after completion of this job.
			&metadata.TextureLoadParams{
				ResourceName:      textureName,
				OutTexture:        texture,
				ImageResource:     &metadata.Resource{},
				CurrentGeneration: texture.Generation,
				TempTexture:       &metadata.Texture{},
			},
		},
		OnStart:    ts.TextureLoadJobStart,
		OnComplete: ts.TextureLoadJobSuccess,
		OnFailure:  ts.TextureLoadJobFail,
	})

	return true
}

func (ts *TextureSystem) LoadCubeTextures(name string, textureNames []string, texture *metadata.Texture) bool {
	pixels := make([]uint8, 0)
	// image_size := uint64(0)
	for i := 0; i < len(textureNames); i++ {
		params := &metadata.ImageResourceParams{
			FlipY: false,
		}

		imgResource, err := ts.assetManager.LoadAsset(textureNames[i], metadata.ResourceTypeImage, params)
		if err != nil {
			core.LogError("func LoadCubeTextures - Failed to load image resource for texture '%s'", textureNames[i])
			return false
		}

		resourceData, ok := imgResource.Data.(*metadata.ImageResourceData)
		if !ok {
			core.LogError("failed to type cast imgResource.Data to `*metadata.ImageResourceData`")
			return false
		}

		if len(pixels) == 0 {
			texture.Width = resourceData.Width
			texture.Height = resourceData.Height
			texture.ChannelCount = resourceData.ChannelCount
			texture.Flags = 0
			texture.Generation = 0
			texture.Name = name

			image_size := texture.Width * texture.Height * uint32(texture.ChannelCount)
			// NOTE: no need for transparency in cube maps, so not checking for it.

			pixels = make([]uint8, image_size*6)
		} else {
			// Verify all textures are the same size.
			if texture.Width != resourceData.Width || texture.Height != resourceData.Height || texture.ChannelCount != resourceData.ChannelCount {
				core.LogError("load_cube_textures - All textures must be the same resolution and bit depth.")
				pixels = nil
				return false
			}
		}

		// Copy to the relevant portion of the array.
		// kcopy_memory(pixels+image_size*i, resource_data.pixels, image_size)

		// Clean up data.
		ts.assetManager.UnloadAsset(imgResource)
	}

	// Acquire internal texture resources and upload to GPU.
	ts.renderer.TextureCreate(pixels, texture)
	pixels = nil

	return true
}

func (ts *TextureSystem) DestroyTexture(texture *metadata.Texture) error {
	// Clean up backend resources.
	if err := ts.renderer.TextureDestroy(texture); err != nil {
		return err
	}

	texture.ID = metadata.InvalidID
	texture.Generation = metadata.InvalidID
	return nil
}

func (ts *TextureSystem) ProcessTextureReference(name string, textureType metadata.TextureType, referenceDiff int8, autoRelease, skipLoad bool) (uint32, bool) {
	outTextureID := metadata.InvalidID

	ts.RegisteredTextureTable[name] = &metadata.TextureReference{
		Handle: metadata.InvalidID,
	}
	ref := ts.RegisteredTextureTable[name]

	// If the reference count starts off at zero, one of two things can be
	// true. If incrementing references, this means the entry is new. If
	// decrementing, then the texture doesn't exist _if_ not auto-releasing.
	if ref.ReferenceCount == 0 && referenceDiff > 0 {
		if referenceDiff > 0 {
			// This can only be changed the first time a texture is loaded.
			ref.AutoRelease = autoRelease
		} else {
			if ref.AutoRelease {
				core.LogWarn("Tried to release non-existent texture: '%s'", name)
				return 0, false
			} else {
				core.LogWarn("Tried to release a texture where autorelease=false, but references was already 0.")
				// Still count this as a success, but warn about it.
				return 0, true
			}
		}
	}

	ref.ReferenceCount += uint64(referenceDiff)

	// If decrementing, this means a release.
	if referenceDiff < 0 {
		// Check if the reference count has reached 0. If it has, and the reference
		// is set to auto-release, destroy the texture.
		if ref.ReferenceCount == 0 && ref.AutoRelease {
			t := ts.RegisteredTextures[ref.Handle]

			// Destroy/reset texture.
			ts.DestroyTexture(t)

			// Reset the reference.
			ref.Handle = metadata.InvalidID
			ref.AutoRelease = false
			// KTRACE("Released texture '%s'., Texture unloaded because reference count=0 and AutoRelease=true.", name_copy);
		} else {
			// KTRACE("Released texture '%s', now has a reference count of '%i' (AutoRelease=%s).", name_copy, ref.reference_count, ref.AutoRelease ? "true" : "false");
		}

	} else {
		// Incrementing. Check if the handle is new or not.
		if ref.Handle == metadata.InvalidID {
			// This means no texture exists here. Find a free index first.
			count := ts.Config.MaxTextureCount

			for i := uint32(0); i < count; i++ {
				if ts.RegisteredTextures[i].ID == metadata.InvalidID {
					// A free slot has been found. Use its index as the handle.
					ref.Handle = i
					outTextureID = i
					break
				}
			}

			// An empty slot was not found, bleat about it and boot out.
			if outTextureID == metadata.InvalidID {
				core.LogError("process_texture_reference - Texture system cannot hold anymore textures. Adjust configuration to allow more.")
				return 0, false
			} else {
				t := ts.RegisteredTextures[ref.Handle]
				t.TextureType = textureType
				// Create new texture.
				if skipLoad {
					// KTRACE("Load skipped for texture '%s'. This is expected behaviour.");
				} else {
					if textureType == metadata.TextureTypeCube {
						texture_names := make([]string, 6)

						// +X,-X,+Y,-Y,+Z,-Z in _cubemap_ space, which is LH y-down
						texture_names[0] = fmt.Sprintf("%s_r", name) // Right texture
						texture_names[1] = fmt.Sprintf("%s_l", name) // Left texture
						texture_names[2] = fmt.Sprintf("%s_u", name) // Up texture
						texture_names[3] = fmt.Sprintf("%s_d", name) // Down texture
						texture_names[4] = fmt.Sprintf("%s_f", name) // Front texture
						texture_names[5] = fmt.Sprintf("%s_b", name) // Back texture

						if !ts.LoadCubeTextures(name, texture_names, t) {
							outTextureID = metadata.InvalidID
							core.LogError("Failed to load cube texture '%s'.", name)
							return 0, false
						}
					} else {
						if !ts.LoadTexture(name, t) {
							outTextureID = metadata.InvalidID
							core.LogError("Failed to load texture '%s'.", name)
							return 0, false
						}
					}
					t.ID = ref.Handle
				}
				// KTRACE("Texture '%s' does not yet exist. Created, and ref_count is now %i.", name, ref.reference_count);
			}
		} else {
			outTextureID = ref.Handle
			// KTRACE("Texture '%s' already exists, ref_count increased to %i.", name, ref.reference_count);
		}
	}

	// Either way, update the entry.
	ts.RegisteredTextureTable[name] = ref

	return outTextureID, true
}

func (ts *TextureSystem) TextureLoadJobSuccess(paramsChan <-chan interface{}) {
	if params, ok := <-paramsChan; ok {
		textureParams, ok := params.(*metadata.TextureLoadParams)
		if !ok {
			core.LogError("params are not of type *TextureLoadParams")
			return
		}

		// This also handles the GPU upload. Can't be jobified until the renderer is multithreaded.
		resourceData := textureParams.ImageResource.Data.(*metadata.ImageResourceData)

		// Acquire internal texture resources and upload to GPU. Can't be jobified until the renderer is multithreaded.
		ts.renderer.TextureCreate(resourceData.Pixels, textureParams.TempTexture)

		// Take a copy of the old texture.
		old := textureParams.OutTexture

		// Assign the temp texture to the pointer.
		textureParams.OutTexture = textureParams.TempTexture

		// Destroy the old texture.
		if err := ts.renderer.TextureDestroy(old); err != nil {
			core.LogError(err.Error())
		}

		if textureParams.CurrentGeneration == metadata.InvalidID {
			textureParams.OutTexture.Generation = 0
		} else {
			textureParams.OutTexture.Generation = textureParams.CurrentGeneration + 1
		}

		core.LogDebug("Successfully loaded texture '%s'.", textureParams.ResourceName)

		// Clean up data.
		ts.assetManager.UnloadAsset(textureParams.ImageResource)
		if textureParams.ResourceName != "" {
			textureParams.ResourceName = ""
		}
	}
}

func (ts *TextureSystem) TextureLoadJobFail(paramsChan <-chan interface{}) {
	if params, ok := <-paramsChan; ok {
		textureParams := params.(*metadata.TextureLoadParams)
		core.LogError("Failed to load texture '%s'.", textureParams.ResourceName)
		ts.assetManager.UnloadAsset(textureParams.ImageResource)
	}
}

func (ts *TextureSystem) TextureLoadJobStart(params interface{}, resultChan chan<- interface{}) error {
	tmpParams := params.([]interface{})

	loadParams := tmpParams[0].(*metadata.TextureLoadParams)

	resource_params := &metadata.ImageResourceParams{
		FlipY: true,
	}

	result, err := ts.assetManager.LoadAsset(loadParams.ResourceName, metadata.ResourceTypeImage, resource_params)
	if err != nil {
		core.LogError(err.Error())
		resultChan <- loadParams
		return err
	}

	core.LogDebug("texture asset load successful")

	loadParams.ImageResource = result

	resourceData := loadParams.ImageResource.Data.(*metadata.ImageResourceData)

	// Use a temporary texture to load into.
	loadParams.TempTexture.Width = resourceData.Width
	loadParams.TempTexture.Height = resourceData.Height
	loadParams.TempTexture.ChannelCount = resourceData.ChannelCount

	loadParams.CurrentGeneration = loadParams.OutTexture.Generation
	loadParams.OutTexture.Generation = metadata.InvalidID

	total_size := loadParams.TempTexture.Width * loadParams.TempTexture.Height * uint32(loadParams.TempTexture.ChannelCount)
	// Check for transparency
	hasTransparency := false
	for i := uint32(0); i < total_size; i += uint32(loadParams.TempTexture.ChannelCount) {
		a := resourceData.Pixels[i+3]
		if a < 255 {
			hasTransparency = true
			break
		}
	}

	core.LogDebug("texture load pixel successful")

	// Take a copy of the name.
	loadParams.TempTexture.Name = loadParams.ResourceName
	loadParams.TempTexture.Generation = metadata.InvalidID
	loadParams.TempTexture.Flags |= 0
	if hasTransparency {
		loadParams.TempTexture.Flags |= metadata.TextureFlagBits(metadata.TextureFlagHasTransparency)
	}

	core.LogDebug("texture load successful")

	resultChan <- loadParams

	return nil
}
