package systems

import (
	"fmt"

	"github.com/spaghettifunk/anima/engine/core"
	"github.com/spaghettifunk/anima/engine/renderer/metadata"
	"github.com/spaghettifunk/anima/engine/systems/loaders"
)

/** @brief The texture system configuration */
type TextureSystemConfig struct {
	/** @brief The maximum number of textures that can be loaded at once. */
	MaxTextureCount uint32
}

type TextureSystem struct {
	Config                 *TextureSystemConfig
	DefaultTexture         *metadata.Texture
	DefaultDiffuseTexture  *metadata.Texture
	DefaultSpecularTexture *metadata.Texture
	DefaultNormalTexture   *metadata.Texture
	// Array of registered textures.
	RegisteredTextures []*metadata.Texture
	// Hashtable for texture lookups.
	RegisteredTextureTable map[string]*metadata.TextureReference
	// sub systems
	jobSystem      *JobSystem
	resourceSystem *ResourceSystem
	renderer       *RendererSystem
}

/**
 * @brief Initializes the texture system.
 * Should be called twice; once to get the memory requirement (passing state=0), and a second
 * time passing an allocated block of memory to actually initialize the system.
 *
 * @param state A block of memory to hold the state or, if gathering the memory requirement, 0.
 * @param config The configuration for this system.
 * @return True on success; otherwise false.
 */
func NewTextureSystem(config *TextureSystemConfig, js *JobSystem, rs *ResourceSystem, r *RendererSystem) (*TextureSystem, error) {
	if config.MaxTextureCount == 0 {
		err := fmt.Errorf("func NewTextureSystem - config.MaxTextureCount must be > 0")
		core.LogFatal(err.Error())
		return nil, err
	}

	ts := &TextureSystem{
		Config:                 config,
		RegisteredTextures:     make([]*metadata.Texture, config.MaxTextureCount),
		RegisteredTextureTable: make(map[string]*metadata.TextureReference),
		DefaultTexture:         &metadata.Texture{},
		DefaultDiffuseTexture:  &metadata.Texture{},
		DefaultSpecularTexture: &metadata.Texture{},
		DefaultNormalTexture:   &metadata.Texture{},
		jobSystem:              js,
		resourceSystem:         rs,
		renderer:               r,
	}

	// Invalidate all textures in the array.
	for i := uint32(0); i < config.MaxTextureCount; i++ {
		ts.RegisteredTextureTable[metadata.GenerateNewHash()] = &metadata.TextureReference{
			AutoRelease:    false,
			Handle:         loaders.InvalidID, // Primary reason for needing default values.
			ReferenceCount: 0,
		}
		ts.RegisteredTextures[i] = &metadata.Texture{
			ID:         loaders.InvalidID,
			Generation: loaders.InvalidID,
		}
	}

	// Create default textures for use in the system.
	ts.CreateDefaultTextures()

	return ts, nil
}

/**
 * @brief Shuts down the texture system.
 *
 * @param state The state block of memory for this system.
 */
func (ts *TextureSystem) Shutdown() error {
	// Destroy all loaded textures.
	for i := uint32(0); i < ts.Config.MaxTextureCount; i++ {
		t := ts.RegisteredTextures[i]
		if t.Generation != loaders.InvalidID {
			ts.renderer.TextureDestroy(t)
		}
	}
	ts.DestroyDefaultTextures()
	return nil
}

/**
 * @brief Attempts to acquire a texture with the given name. If it has not yet been loaded,
 * this triggers it to load. If the texture is not found, a pointer to the default texture
 * is returned. If the texture _is_ found and loaded, its reference counter is incremented.
 *
 * @param name The name of the texture to find.
 * @param autoRelease Indicates if the texture should auto-release when its reference count is 0.
 * Only takes effect the first time the texture is acquired.
 * @return A pointer to the loaded texture. Can be a pointer to the default texture if not found.
 */
func (ts *TextureSystem) Acquire(name string, autoRelease bool) (*metadata.Texture, error) {
	// Return default texture, but warn about it since this should be returned via get_default_texture();
	// TODO: Check against other default texture names?
	if name == metadata.DEFAULT_TEXTURE_NAME {
		core.LogWarn("func texture system Acquire called for default texture. Use texture_system_get_default_texture for texture 'default'")
		return ts.DefaultTexture, nil
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
 * @param name The name of the texture to find. Used as a base string for actual texture names.
 * @param autoRelease Indicates if the texture should auto-release when its reference count is 0.
 * Only takes effect the first time the texture is acquired.
 * @return A pointer to the loaded texture. Can be a pointer to the default texture if not found.
 */
func (ts *TextureSystem) AcquireCube(name string, autoRelease bool) (*metadata.Texture, error) {
	// Return default texture, but warn about it since this should be returned via get_default_texture();
	// TODO: Check against other default texture names?
	if name == metadata.DEFAULT_TEXTURE_NAME {
		core.LogWarn("func texture system AcquireCube called for default texture. Use texture_system_get_default_texture for texture 'default'")
		return ts.DefaultTexture, nil
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

/**
 * @brief Attempts to acquire a writeable texture with the given name. This does not point to
 * nor attempt to load a texture file. Does also increment the reference counter.
 * NOTE: Writeable textures are not auto-released.
 *
 * @param name The name of the texture to acquire.
 * @param width The texture width in pixels.
 * @param height The texture height in pixels.
 * @param channelCount The number of channels in the texture (typically 4 for RGBA)
 * @param hasTransparency Indicates if the texture will have transparency.
 * @return A pointer to the generated texture.
 */
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
	texture.Generation = loaders.InvalidID

	texture.Flags |= metadata.TextureFlagBits(metadata.TextureFlagHasTransparency)
	if hasTransparency {
		texture.Flags |= 0
	}

	texture.Flags |= metadata.TextureFlagBits(metadata.TextureFlagIsWriteable)
	texture.InternalData = nil

	ts.renderer.TextureCreateWriteable(texture)

	return texture, nil
}

/**
 * @brief Releases a texture with the given name. Ignores non-existant textures.
 * Decreases the reference counter by 1. If the reference counter reaches 0 and
 * autoRelease was set to true, the texture is unloaded, releasing internal resources.
 *
 * @param name The name of the texture to unload.
 */
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

/**
 * @brief Wraps the provided internal data in a texture structure using the parameters
 * provided. This is best used for when the renderer system creates internal resources
 * and they should be passed off to the texture system. Can be looked up by name via
 * the acquire methods.
 * NOTE: Wrapped textures are not auto-released.
 *
 * @param name The name of the texture.
 * @param width The texture width in pixels.
 * @param height The texture height in pixels.
 * @param channelCount The number of channels in the texture (typically 4 for RGBA)
 * @param hasTransparency Indicates if the texture will have transparency.
 * @param isWriteable Indicates if the texture is writeable.
 * @param internalData A pointer to the internal data to be set on the texture.
 * @param registerTexture Indicates if the texture should be registered with the system.
 * @return A pointer to the wrapped texture.
 */
func (ts *TextureSystem) WrapInternal(name string, width, height uint32, channelCount uint8, hasTransparency, isWriteable, registerTexture bool, internalData interface{}) (*metadata.Texture, error) {
	id := loaders.InvalidID
	var texture *metadata.Texture
	if registerTexture {
		// NOTE: Wrapped textures are never auto-released because it means that thier
		// resources are created and managed somewhere within the renderer internals.
		id, ok := ts.ProcessTextureReference(name, metadata.TextureType2d, 1, false, true)
		if !ok {
			err := fmt.Errorf("func texture system WrapInternal failed to obtain a new texture id")
			core.LogError(err.Error())
			return nil, err
		}
		texture = ts.RegisteredTextures[id]
	} else {
		texture = &metadata.Texture{}
	}

	texture.ID = id
	texture.TextureType = metadata.TextureType2d
	texture.Name = name
	texture.Width = width
	texture.Height = height
	texture.ChannelCount = channelCount
	texture.Generation = loaders.InvalidID
	texture.InternalData = internalData

	texture.Flags |= 0
	if hasTransparency {
		texture.Flags |= metadata.TextureFlagBits(metadata.TextureFlagHasTransparency)
	}
	texture.Flags |= 0
	if isWriteable {
		texture.Flags |= metadata.TextureFlagBits(metadata.TextureFlagIsWriteable)
	}
	texture.Flags |= metadata.TextureFlagBits(metadata.TextureFlagIsWrapped)

	return texture, nil
}

/**
 * @brief Sets the internal data of a texture. Useful for replacing internal data from within the
 * renderer for wrapped textures, for example.
 *
 * @param t A pointer to the texture to be updated.
 * @param internalData A pointer to the internal data to be set.
 * @return True on success; otherwise false.
 */
func (ts *TextureSystem) SetInternal(texture *metadata.Texture, internalData interface{}) bool {
	if texture != nil {
		texture.InternalData = internalData
		texture.Generation++
		return true
	}
	return false
}

/**
 * @brief Resizes the given texture. May only be done on writeable textures.
 * Potentially regenerates internal data, if configured to do so.
 *
 * @param t A pointer to the texture to be resized.
 * @param width The new width in pixels.
 * @param height The new height in pixels.
 * @param regenerateInternalData Indicates if the internal data should be regenerated.
 * @return True on success; otherwise false.
 */
func (ts *TextureSystem) Resize(texture *metadata.Texture, width, height uint32, regenerateInternalData bool) bool {
	if texture != nil {
		if (texture.Flags & metadata.TextureFlagBits(metadata.TextureFlagIsWriteable)) == 0 {
			core.LogWarn("texture_system_resize should not be called on textures that are not writeable.")
			return false
		}
		texture.Width = width
		texture.Height = height
		// Only allow this for writeable textures that are not wrapped.
		// Wrapped textures can call texture_system_set_internal then call
		// this function to get the above parameter updates and a generation
		// update.
		if (texture.Flags&metadata.TextureFlagBits(metadata.TextureFlagIsWrapped) == 0) && regenerateInternalData {
			// Regenerate internals for the new size.
			ts.renderer.TextureResize(texture, width, height)
			return false
		}
		texture.Generation++
		return true
	}
	return false
}

/**
 * @brief Writes the given data to the provided texture. May only be used on
 * writeable textures.
 *
 * @param t A pointer to the texture to be written to.
 * @param offset The offset in bytes from the beginning of the data to be written.
 * @param size The number of bytes to be written.
 * @param data A pointer to the data to be written.
 * @return True on success; otherwise false.
 */
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

/**
 * @brief Gets a pointer to the default texture. No reference counting is
 * done for default textures.
 */
func (ts *TextureSystem) GetDefaultTexture() *metadata.Texture {
	return ts.DefaultTexture
}

/**
 * @brief Gets a pointer to the default diffuse texture. No reference counting is
 * done for default textures.
 */
func (ts *TextureSystem) GetDefaultDiffuseTexture() *metadata.Texture {
	return ts.DefaultDiffuseTexture
}

/**
 * @brief Gets a pointer to the default specular texture. No reference counting is
 * done for default textures.
 */
func (ts *TextureSystem) GetDefaultSpecularTexture() *metadata.Texture {
	return ts.DefaultSpecularTexture
}

/**
 * @brief Gets a pointer to the default normal texture. No reference counting is
 * done for default textures.
 */
func (ts *TextureSystem) GetDefaultNormalTexture() *metadata.Texture {
	return ts.DefaultNormalTexture
}

func (ts *TextureSystem) CreateDefaultTextures() bool {
	// NOTE: Create default texture, a 256x256 blue/white checkerboard pattern.
	// This is done in code to eliminate asset dependencies.
	// KTRACE("Creating default texture...");
	texDimension := uint32(256)
	channels := uint32(4)
	pixelCount := uint32(texDimension * texDimension)

	pixels := make([]uint8, pixelCount*channels)

	// Each pixel.
	for row := uint32(0); row < texDimension; row++ {
		for col := uint32(0); col < texDimension; col++ {
			index := uint32((row * texDimension) + col)
			index_bpp := uint32(index * channels)
			if row%2 != 0 {
				if col%2 != 0 {
					pixels[index_bpp+0] = 0
					pixels[index_bpp+1] = 0
				}
			} else {
				if col%2 == 0 {
					pixels[index_bpp+0] = 0
					pixels[index_bpp+1] = 0
				}
			}
		}
	}

	ts.DefaultTexture.Name = metadata.DEFAULT_TEXTURE_NAME

	ts.DefaultTexture.Width = texDimension
	ts.DefaultTexture.Height = texDimension
	ts.DefaultTexture.ChannelCount = 4
	ts.DefaultTexture.Generation = loaders.InvalidID
	ts.DefaultTexture.Flags = 0
	ts.DefaultTexture.TextureType = metadata.TextureType2d
	ts.renderer.TextureCreate(pixels, ts.DefaultTexture)
	// Manually set the texture generation to invalid since this is a default texture.
	ts.DefaultTexture.Generation = loaders.InvalidID

	// Diffuse texture.
	// KTRACE("Creating default diffuse texture...");
	diffPixels := make([]uint8, 16*16*4)
	// Default diffuse map is all white.

	ts.DefaultDiffuseTexture.Name = metadata.DEFAULT_DIFFUSE_TEXTURE_NAME
	ts.DefaultDiffuseTexture.Width = 16
	ts.DefaultDiffuseTexture.Height = 16
	ts.DefaultDiffuseTexture.ChannelCount = 4
	ts.DefaultDiffuseTexture.Generation = loaders.InvalidID
	ts.DefaultDiffuseTexture.Flags = 0
	ts.DefaultDiffuseTexture.TextureType = metadata.TextureType2d
	ts.renderer.TextureCreate(diffPixels, ts.DefaultDiffuseTexture)

	// Manually set the texture generation to invalid since this is a default texture.
	ts.DefaultDiffuseTexture.Generation = loaders.InvalidID

	// Specular texture.
	// KTRACE("Creating default specular texture...");
	specPixels := make([]uint8, 16*16*4)
	// Default spec map is black (no specular)

	ts.DefaultSpecularTexture.Name = metadata.DEFAULT_SPECULAR_TEXTURE_NAME
	ts.DefaultSpecularTexture.Width = 16
	ts.DefaultSpecularTexture.Height = 16
	ts.DefaultSpecularTexture.ChannelCount = 4
	ts.DefaultSpecularTexture.Generation = loaders.InvalidID
	ts.DefaultSpecularTexture.Flags = 0
	ts.DefaultSpecularTexture.TextureType = metadata.TextureType2d

	ts.renderer.TextureCreate(specPixels, ts.DefaultSpecularTexture)
	// Manually set the texture generation to invalid since this is a default texture.
	ts.DefaultSpecularTexture.Generation = loaders.InvalidID

	// Normal texture.
	// KTRACE("Creating default normal texture...");
	normalPixels := make([]uint8, 16*16*4) // w * h * channels

	// Each pixel.
	for row := 0; row < 16; row++ {
		for col := 0; col < 16; col++ {
			index := uint32((row * 16) + col)
			index_bpp := index * channels
			// Set blue, z-axis by default and alpha.
			normalPixels[index_bpp+0] = 128
			normalPixels[index_bpp+1] = 128
			normalPixels[index_bpp+2] = 255
			normalPixels[index_bpp+3] = 255
		}
	}

	ts.DefaultNormalTexture.Name = metadata.DEFAULT_NORMAL_TEXTURE_NAME
	ts.DefaultNormalTexture.Width = 16
	ts.DefaultNormalTexture.Height = 16
	ts.DefaultNormalTexture.ChannelCount = 4
	ts.DefaultNormalTexture.Generation = loaders.InvalidID
	ts.DefaultNormalTexture.Flags = 0
	ts.DefaultNormalTexture.TextureType = metadata.TextureType2d
	ts.renderer.TextureCreate(normalPixels, ts.DefaultNormalTexture)

	// Manually set the texture generation to invalid since this is a default texture.
	ts.DefaultNormalTexture.Generation = loaders.InvalidID

	return true
}

func (ts *TextureSystem) DestroyDefaultTextures() {
	ts.DestroyTexture(ts.DefaultTexture)
	ts.DestroyTexture(ts.DefaultDiffuseTexture)
	ts.DestroyTexture(ts.DefaultSpecularTexture)
	ts.DestroyTexture(ts.DefaultNormalTexture)
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

		imgResource, err := ts.resourceSystem.Load(textureNames[i], metadata.ResourceTypeImage, params)
		if err != nil {
			core.LogError("func LoadCubeTextures - Failed to load image resource for texture '%s'", textureNames[i])
			return false
		}

		resourceData, ok := imgResource.Data.(*metadata.ImageResourceData)
		if !ok {
			core.LogError("failed to type cast img_resource.Data to `*metadata.ImageResourceData`")
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
		ts.resourceSystem.Unload(imgResource)
	}

	// Acquire internal texture resources and upload to GPU.
	ts.renderer.TextureCreate(pixels, texture)
	pixels = nil

	return true
}

func (ts *TextureSystem) DestroyTexture(texture *metadata.Texture) {
	// Clean up backend resources.
	ts.renderer.TextureDestroy(texture)

	texture.ID = loaders.InvalidID
	texture.Generation = loaders.InvalidID
}

func (ts *TextureSystem) ProcessTextureReference(name string, textureType metadata.TextureType, referenceDiff int8, autoRelease, skipLoad bool) (uint32, bool) {
	outTextureID := loaders.InvalidID

	ref, ok := ts.RegisteredTextureTable[name]
	if ok {
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

		// Take a copy of the name since it would be wiped out if destroyed,
		// (as passed in name is generally a pointer to the actual texture's name).
		name_copy := name

		// If decrementing, this means a release.
		if referenceDiff < 0 {
			// Check if the reference count has reached 0. If it has, and the reference
			// is set to auto-release, destroy the texture.
			if ref.ReferenceCount == 0 && ref.AutoRelease {
				t := ts.RegisteredTextures[ref.Handle]

				// Destroy/reset texture.
				ts.DestroyTexture(t)

				// Reset the reference.
				ref.Handle = loaders.InvalidID
				ref.AutoRelease = false
				// KTRACE("Released texture '%s'., Texture unloaded because reference count=0 and AutoRelease=true.", name_copy);
			} else {
				// KTRACE("Released texture '%s', now has a reference count of '%i' (AutoRelease=%s).", name_copy, ref.reference_count, ref.AutoRelease ? "true" : "false");
			}

		} else {
			// Incrementing. Check if the handle is new or not.
			if ref.Handle == loaders.InvalidID {
				// This means no texture exists here. Find a free index first.
				count := ts.Config.MaxTextureCount

				for i := uint32(0); i < count; i++ {
					if ts.RegisteredTextures[i].ID == loaders.InvalidID {
						// A free slot has been found. Use its index as the handle.
						ref.Handle = i
						outTextureID = i
						break
					}
				}

				// An empty slot was not found, bleat about it and boot out.
				if outTextureID == loaders.InvalidID {
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
								outTextureID = loaders.InvalidID
								core.LogError("Failed to load cube texture '%s'.", name)
								return 0, false
							}
						} else {
							if !ts.LoadTexture(name, t) {
								outTextureID = loaders.InvalidID
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
		ts.RegisteredTextureTable[name_copy] = ref

		return outTextureID, true
	}

	// NOTE: This would only happen in the event something went wrong with the state.
	core.LogError("process_texture_reference failed to acquire id for name '%s'. loaders.InvalidID returned.", name)
	return 0, false
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
		ts.renderer.TextureDestroy(old)

		if textureParams.CurrentGeneration == loaders.InvalidID {
			textureParams.OutTexture.Generation = 0
		} else {
			textureParams.OutTexture.Generation = textureParams.CurrentGeneration + 1
		}

		core.LogDebug("Successfully loaded texture '%s'.", textureParams.ResourceName)

		// Clean up data.
		ts.resourceSystem.Unload(textureParams.ImageResource)
		if textureParams.ResourceName != "" {
			textureParams.ResourceName = ""
		}
	}
}

func (ts *TextureSystem) TextureLoadJobFail(paramsChan <-chan interface{}) {
	if params, ok := <-paramsChan; ok {
		textureParams := params.(*metadata.TextureLoadParams)
		core.LogError("Failed to load texture '%s'.", textureParams.ResourceName)
		ts.resourceSystem.Unload(textureParams.ImageResource)
	}
}

func (ts *TextureSystem) TextureLoadJobStart(params interface{}, resultChan chan<- interface{}) error {
	loadParams := params.(*metadata.TextureLoadParams)

	resource_params := &metadata.ImageResourceParams{
		FlipY: true,
	}

	result, err := ts.resourceSystem.Load(loadParams.ResourceName, metadata.ResourceTypeImage, resource_params)
	if err != nil {
		core.LogError(err.Error())
		resultChan <- loadParams
		return err
	}
	loadParams.ImageResource = result

	resourceData := loadParams.ImageResource.Data.(*metadata.ImageResourceData)

	// Use a temporary texture to load into.
	loadParams.TempTexture.Width = resourceData.Width
	loadParams.TempTexture.Height = resourceData.Height
	loadParams.TempTexture.ChannelCount = resourceData.ChannelCount

	loadParams.CurrentGeneration = loadParams.OutTexture.Generation
	loadParams.OutTexture.Generation = loaders.InvalidID

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

	// Take a copy of the name.
	loadParams.TempTexture.Name = loadParams.ResourceName
	loadParams.TempTexture.Generation = loaders.InvalidID
	loadParams.TempTexture.Flags |= 0
	if hasTransparency {
		loadParams.TempTexture.Flags |= metadata.TextureFlagBits(metadata.TextureFlagHasTransparency)
	}

	resultChan <- loadParams

	return nil
}
