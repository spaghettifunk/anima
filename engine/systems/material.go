package systems

import (
	"fmt"

	"github.com/spaghettifunk/anima/engine/assets"
	"github.com/spaghettifunk/anima/engine/assets/loaders"
	"github.com/spaghettifunk/anima/engine/core"
	"github.com/spaghettifunk/anima/engine/math"
	"github.com/spaghettifunk/anima/engine/renderer/metadata"
)

/** @brief The configuration for the material system. */
type MaterialSystemConfig struct {
	/** @brief The maximum number of loaded materials. */
	MaxMaterialCount uint32
}

type MaterialSystem struct {
	Config          *MaterialSystemConfig
	DefaultMaterial *metadata.Material
	// Array of registered materials.
	RegisteredMaterials []*metadata.Material
	// Hashtable for material lookups.
	RegisteredMaterialTable map[string]*metadata.MaterialReference
	// Known locations for the material shader.
	MaterialLocations *metadata.MaterialShaderUniformLocations
	MaterialShaderID  uint32
	// Known locations for the UI shader.
	UILocations *metadata.UIShaderUniformLocations
	UIShaderID  uint32
	// sub systems
	shaderSystem  *ShaderSystem
	textureSystem *TextureSystem
	renderer      *RendererSystem
	assetManager  *assets.AssetManager
}

/**
 * @brief Initializes the material system.
 * Should be called twice; once to get the memory requirement (passing state=0), and a second
 * time passing an allocated block of memory to actually initialize the system.
 *
 * @param memory_requirement A pointer to hold the memory requirement as it is calculated.
 * @param state A block of memory to hold the state or, if gathering the memory requirement, 0.
 * @param config The configuration for this system.
 * @return True on success; otherwise false.
 */
func NewMaterialSystem(config *MaterialSystemConfig, shaderSytem *ShaderSystem, ts *TextureSystem, am *assets.AssetManager, r *RendererSystem) (*MaterialSystem, error) {
	if config.MaxMaterialCount == 0 {
		core.LogError("func NewMaterialSystem - config.MaxMaterialCount must be > 0.")
		return nil, nil
	}

	ms := &MaterialSystem{
		MaterialShaderID: loaders.InvalidID,
		DefaultMaterial: &metadata.Material{
			DiffuseMap: &metadata.TextureMap{
				Texture: &metadata.Texture{},
			},
			SpecularMap: &metadata.TextureMap{
				Texture: &metadata.Texture{},
			},
			NormalMap: &metadata.TextureMap{
				Texture: &metadata.Texture{},
			},
			DiffuseColour: math.NewVec4One(),
		},
		MaterialLocations: &metadata.MaterialShaderUniformLocations{
			View:            loaders.InvalidIDUint16,
			Projection:      loaders.InvalidIDUint16,
			DiffuseColour:   loaders.InvalidIDUint16,
			DiffuseTexture:  loaders.InvalidIDUint16,
			SpecularTexture: loaders.InvalidIDUint16,
			NormalTexture:   loaders.InvalidIDUint16,
			AmbientColour:   loaders.InvalidIDUint16,
			Shininess:       loaders.InvalidIDUint16,
			Model:           loaders.InvalidIDUint16,
			RenderMode:      loaders.InvalidIDUint16,
		},
		UIShaderID: loaders.InvalidID,
		UILocations: &metadata.UIShaderUniformLocations{
			DiffuseColour:  loaders.InvalidIDUint16,
			DiffuseTexture: loaders.InvalidIDUint16,
			View:           loaders.InvalidIDUint16,
			Projection:     loaders.InvalidIDUint16,
			Model:          loaders.InvalidIDUint16,
		},
		RegisteredMaterials:     make([]*metadata.Material, config.MaxMaterialCount),
		RegisteredMaterialTable: make(map[string]*metadata.MaterialReference),
		shaderSystem:            shaderSytem,
		textureSystem:           ts,
		assetManager:            am,
		renderer:                r,
	}

	// Fill the hashtable with invalid references to use as a default.
	invalid_ref := &metadata.MaterialReference{
		AutoRelease:    false,
		Handle:         loaders.InvalidID,
		ReferenceCount: 0,
	}

	// Invalidate all materials in the array.
	for i := uint32(0); i < config.MaxMaterialCount; i++ {
		ms.RegisteredMaterialTable[metadata.GenerateNewHash()] = invalid_ref
		ms.RegisteredMaterials[i] = &metadata.Material{
			ID:                loaders.InvalidID,
			Generation:        loaders.InvalidID,
			InternalID:        loaders.InvalidID,
			RenderFrameNumber: loaders.InvalidID,
		}
	}

	if !ms.createDefaultMaterial() {
		core.LogError("Failed to create default material. Application cannot continue.")
		return nil, nil
	}
	return ms, nil
}

/**
 * @brief Shuts down the material system.
 *
 * @param state The state block of memory.
 */
func (ms *MaterialSystem) Shutdown() error {
	// Invalidate all materials in the array.
	for i := uint32(0); i < ms.Config.MaxMaterialCount; i++ {
		if ms.RegisteredMaterials[i].ID != loaders.InvalidID {
			ms.destroyMaterial(ms.RegisteredMaterials[i])
		}
	}
	// Destroy the default material.
	ms.destroyMaterial(ms.DefaultMaterial)

	return nil
}

/**
 * @brief Attempts to acquire a material with the given name. If it has not yet been loaded,
 * this triggers it to load. If the material is not found, a pointer to the default material
 * is returned. If the material _is_ found and loaded, its reference counter is incremented.
 *
 * @param name The name of the material to find.
 * @return A pointer to the loaded material. Can be a pointer to the default material if not found.
 */
func (ms *MaterialSystem) Acquire(name string) (*metadata.Material, error) {
	// Load material configuration from resource;
	materialResource, err := ms.assetManager.LoadAsset(name, metadata.ResourceTypeMaterial, nil)
	if err != nil {
		err := fmt.Errorf("failed to load material resource, returning nullptr")
		core.LogError(err.Error())
		return nil, err
	}

	// Now acquire from loaded config.
	m := &metadata.Material{}
	if materialResource.Data != nil {
		cfg, ok := materialResource.Data.(*metadata.MaterialConfig)
		if !ok {
			err := fmt.Errorf("failed to cast to `*metadata.MaterialConfig`")
			core.LogError(err.Error())
			return nil, err
		}
		m, err = ms.AcquireFromConfig(cfg)
		if err != nil {
			core.LogError(err.Error())
			return nil, err
		}
	}

	// Clean up
	if err := ms.assetManager.UnloadAsset(materialResource); err != nil {
		return nil, err
	}

	if m != nil {
		err := fmt.Errorf("failed to load material resource, returning nullptr")
		core.LogError(err.Error())
		return nil, err
	}

	return m, nil
}

/**
 * @brief Attempts to acquire a material from the given configuration. If it has not yet been loaded,
 * this triggers it to load. If the material is not found, a pointer to the default material
 * is returned. If the material _is_ found and loaded, its reference counter is incremented.
 *
 * @param config The config of the material to load.
 * @return A pointer to the loaded material.
 */
func (ms *MaterialSystem) AcquireFromConfig(config *metadata.MaterialConfig) (*metadata.Material, error) {
	// Return default material.
	if config.Name == metadata.DefaultMaterialName {
		return ms.DefaultMaterial, nil
	}

	ref := ms.RegisteredMaterialTable[config.Name]

	// This can only be changed the first time a material is loaded.
	if ref.ReferenceCount == 0 {
		ref.AutoRelease = config.AutoRelease
	}
	ref.ReferenceCount++
	if ref.Handle == loaders.InvalidID {
		// This means no material exists here. Find a free index first.
		count := ms.Config.MaxMaterialCount
		var material *metadata.Material
		for i := uint32(0); i < count; i++ {
			if ms.RegisteredMaterials[i].ID == loaders.InvalidID {
				// A free slot has been found. Use its index as the handle.
				ref.Handle = i
				material = ms.RegisteredMaterials[i]
				break
			}
		}

		// Make sure an empty slot was actually found.
		if material == nil || ref.Handle == loaders.InvalidID {
			err := fmt.Errorf("material_system_acquire - Material system cannot hold anymore materials. Adjust configuration to allow more")
			core.LogError(err.Error())
			return nil, err
		}

		// Create new material.
		material = ms.loadMaterial(config)
		if material == nil {
			err := fmt.Errorf("failed to load material '%s'", config.Name)
			core.LogError(err.Error())
			return nil, err
		}

		// Get the uniform indices.
		shader, err := ms.shaderSystem.GetShaderByID(material.ShaderID)
		if err != nil {
			core.LogError(err.Error())
			return nil, err
		}
		// Save off the locations for known types for quick lookups.
		if ms.MaterialShaderID == loaders.InvalidID && config.ShaderName == metadata.BUILTIN_SHADER_NAME_MATERIAL {
			ms.MaterialShaderID = shader.ID
			ms.MaterialLocations.Projection = ms.shaderSystem.GetUniformIndex(shader, "projection")
			ms.MaterialLocations.View = ms.shaderSystem.GetUniformIndex(shader, "view")
			ms.MaterialLocations.AmbientColour = ms.shaderSystem.GetUniformIndex(shader, "ambient_colour")
			ms.MaterialLocations.ViewPosition = ms.shaderSystem.GetUniformIndex(shader, "view_position")
			ms.MaterialLocations.DiffuseColour = ms.shaderSystem.GetUniformIndex(shader, "diffuse_colour")
			ms.MaterialLocations.DiffuseTexture = ms.shaderSystem.GetUniformIndex(shader, "diffuse_texture")
			ms.MaterialLocations.SpecularTexture = ms.shaderSystem.GetUniformIndex(shader, "specular_texture")
			ms.MaterialLocations.NormalTexture = ms.shaderSystem.GetUniformIndex(shader, "normal_texture")
			ms.MaterialLocations.Shininess = ms.shaderSystem.GetUniformIndex(shader, "shininess")
			ms.MaterialLocations.Model = ms.shaderSystem.GetUniformIndex(shader, "model")
			ms.MaterialLocations.RenderMode = ms.shaderSystem.GetUniformIndex(shader, "mode")
		} else if ms.UIShaderID == loaders.InvalidID && config.ShaderName == metadata.BUILTIN_SHADER_NAME_UI {
			ms.UIShaderID = shader.ID
			ms.UILocations.Projection = ms.shaderSystem.GetUniformIndex(shader, "projection")
			ms.UILocations.View = ms.shaderSystem.GetUniformIndex(shader, "view")
			ms.UILocations.DiffuseColour = ms.shaderSystem.GetUniformIndex(shader, "diffuse_colour")
			ms.UILocations.DiffuseTexture = ms.shaderSystem.GetUniformIndex(shader, "diffuse_texture")
			ms.UILocations.Model = ms.shaderSystem.GetUniformIndex(shader, "model")
		}

		if material.Generation == loaders.InvalidID {
			material.Generation = 0
		} else {
			material.Generation++
		}

		// Also use the handle as the material id.
		material.ID = ref.Handle
		// KTRACE("Material '%s' does not yet exist. Created, and ref_count is now %i.", config.name, ref.ReferenceCount);
	} else {
		// KTRACE("Material '%s' already exists, ref_count increased to %i.", config.name, ref.ReferenceCount);
	}

	// Update the entry.
	ms.RegisteredMaterialTable[config.Name] = ref
	return ms.RegisteredMaterials[ref.Handle], nil
}

/**
 * @brief Releases a material with the given name. Ignores non-existant materials.
 * Decreases the reference counter by 1. If the reference counter reaches 0 and
 * AutoRelease was set to true, the material is unloaded, releasing internal resources.
 *
 * @param name The name of the material to unload.
 */
func (ms *MaterialSystem) Release(name string) {
	// Ignore release requests for the default material.
	if name == metadata.DefaultMaterialName {
		return
	}
	ref := ms.RegisteredMaterialTable[name]
	if ref != nil {
		if ref.ReferenceCount == 0 {
			core.LogWarn("Tried to release non-existent material: '%s'", name)
			return
		}
		ref.ReferenceCount--
		if ref.ReferenceCount == 0 && ref.AutoRelease {
			material := ms.RegisteredMaterials[ref.Handle]
			// Destroy/reset material.
			ms.destroyMaterial(material)
			// Reset the reference.
			ref.Handle = loaders.InvalidID
			ref.AutoRelease = false
			// KTRACE("Released material '%s'., Material unloaded because reference count=0 and AutoRelease=true.", name);
		} else {
			// KTRACE("Released material '%s', now has a reference count of '%i' (AutoRelease=%s).", name, ref.ReferenceCount, ref.AutoRelease ? "true" : "false");
		}
		// Update the entry.
		ms.RegisteredMaterialTable[name] = ref
	} else {
		core.LogError("material_system_release failed to release material '%s'", name)
	}
}

/**
 * @brief Gets a pointer to the default material. Does not reference count.
 */
func (ms *MaterialSystem) GetDefault() *metadata.Material {
	return ms.DefaultMaterial
}

/**
 * @brief Applies global-level data for the material shader id.
 *
 * @param ShaderID The identifier of the shader to apply globals for.
 * @param renderer_frame_number The renderer's current frame number.
 * @param projection A constant pointer to a projection matrix.
 * @param view A constant pointer to a view matrix.
 * @param ambient_colour The ambient colour of the scene.
 * @param view_position The camera position.
 * @param render_mode The render mode.
 * @return True on success; otherwise false.
 */
func (ms *MaterialSystem) ApplyGlobal(shaderID uint32, renderer_frame_number uint64, projection []math.Mat4, view []math.Mat4, ambient_colour []math.Vec3, view_position []math.Vec3, render_mode uint32) bool {
	shader, err := ms.shaderSystem.GetShaderByID(shaderID)
	if err != nil {
		core.LogError(err.Error())
		return false
	}
	if shader.RenderFrameNumber == renderer_frame_number {
		return true
	}
	if shaderID == ms.MaterialShaderID {
		if ok := ms.shaderSystem.SetUniformByIndex(ms.MaterialLocations.Projection, projection); !ok {
			return ms.materialFail("msState.MaterialLocations.Projection")
		}
		if ok := ms.shaderSystem.SetUniformByIndex(ms.MaterialLocations.View, view); !ok {
			return ms.materialFail("msState.MaterialLocations.View")
		}
		if ok := ms.shaderSystem.SetUniformByIndex(ms.MaterialLocations.AmbientColour, ambient_colour); !ok {
			return ms.materialFail("msState.MaterialLocations.AmbientColour")
		}
		if ok := ms.shaderSystem.SetUniformByIndex(ms.MaterialLocations.ViewPosition, view_position); !ok {
			return ms.materialFail("msState.MaterialLocations.ViewPosition")
		}
		if ok := ms.shaderSystem.SetUniformByIndex(ms.MaterialLocations.RenderMode, &render_mode); !ok {
			return ms.materialFail("msState.MaterialLocations.RenderMode")
		}
	} else if shaderID == ms.UIShaderID {
		if ok := ms.shaderSystem.SetUniformByIndex(ms.UILocations.Projection, projection); !ok {
			return ms.materialFail("msState.UILocations.Projection")
		}
		if ok := ms.shaderSystem.SetUniformByIndex(ms.UILocations.View, view); !ok {
			return ms.materialFail("msState.UILocations.View")
		}
	} else {
		core.LogError("func MaterialSystemApplyGlobal(): Unrecognized shader id '%d' ", shaderID)
		return false
	}
	ms.shaderSystem.ApplyGlobal()

	// Sync the frame number.
	shader.RenderFrameNumber = renderer_frame_number
	return true
}

/**
 * @brief Applies instance-level material data for the given material.
 *
 * @param m A pointer to the material to be applied.
 * @param needsUpdate Indicates if material internals require updating, or if they should just be bound.
 * @return True on success; otherwise false.
 */
func (ms *MaterialSystem) ApplyInstance(material *metadata.Material, needsUpdate bool) bool {
	// Apply instance-level uniforms.
	if ok := ms.shaderSystem.BindInstance(material.InternalID); !ok {
		return ms.materialFail("material.InternalID")
	}
	if needsUpdate {
		if material.ShaderID == ms.MaterialShaderID {
			// Material shader
			if ok := ms.shaderSystem.SetUniformByIndex(ms.MaterialLocations.DiffuseColour, &material.DiffuseColour); !ok {
				return ms.materialFail("msState.MaterialLocations.DiffuseColour")
			}
			if ok := ms.shaderSystem.SetUniformByIndex(ms.MaterialLocations.DiffuseTexture, &material.DiffuseMap); !ok {
				return ms.materialFail("msState.MaterialLocations.DiffuseTexture")
			}
			if ok := ms.shaderSystem.SetUniformByIndex(ms.MaterialLocations.SpecularTexture, &material.SpecularMap); !ok {
				return ms.materialFail("msState.MaterialLocations.SpecularTexture")
			}
			if ok := ms.shaderSystem.SetUniformByIndex(ms.MaterialLocations.NormalTexture, &material.NormalMap); !ok {
				return ms.materialFail("msState.MaterialLocations.NormalTexture")
			}
			if ok := ms.shaderSystem.SetUniformByIndex(ms.MaterialLocations.Shininess, &material.Shininess); !ok {
				return ms.materialFail("msState.MaterialLocations.Shininess")
			}
		} else if material.ShaderID == ms.UIShaderID {
			// UI shader
			if ok := ms.shaderSystem.SetUniformByIndex(ms.UILocations.DiffuseColour, &material.DiffuseColour); !ok {
				return ms.materialFail("msState.UILocations.DiffuseColour")
			}
			if ok := ms.shaderSystem.SetUniformByIndex(ms.UILocations.DiffuseTexture, &material.DiffuseMap); !ok {
				return ms.materialFail("msState.UILocations.DiffuseTexture")
			}
		} else {
			core.LogError("material_system_apply_instance(): Unrecognized shader id '%d' on shader '%s'.", material.ShaderID, material.Name)
			return false
		}
	}
	if ok := ms.shaderSystem.ApplyInstance(needsUpdate); !ok {
		return ms.materialFail("needsUpdate")
	}
	return true
}

/**
 * @brief Applies local-level material data (typically just model matrix).
 *
 * @param m A pointer to the material to be applied.
 * @param model A constant pointer to the model matrix to be applied.
 * @return True on success; otherwise false.
 */
func (ms *MaterialSystem) ApplyLocal(material *metadata.Material, model [][]math.Mat4) bool {
	if material.ShaderID == ms.MaterialShaderID {
		return ms.shaderSystem.SetUniformByIndex(ms.MaterialLocations.Model, model)
	} else if material.ShaderID == ms.UIShaderID {
		return ms.shaderSystem.SetUniformByIndex(ms.UILocations.Model, model)
	}
	core.LogError("Unrecognized shader id '%d'", material.ShaderID)
	return false
}

func (ms *MaterialSystem) loadMaterial(config *metadata.MaterialConfig) *metadata.Material {
	material := &metadata.Material{}

	material.Name = config.Name

	material.ShaderID = ms.shaderSystem.GetShaderID(config.ShaderName)

	// Diffuse colour
	material.DiffuseColour = config.DiffuseColour
	material.Shininess = config.Shininess

	// Diffuse map
	// TODO: Make this configurable.
	// TODO: DRY
	material.DiffuseMap.FilterMinify = metadata.TextureFilterModeLinear
	material.DiffuseMap.FilterMagnify = metadata.TextureFilterModeLinear
	material.DiffuseMap.RepeatU = metadata.TextureRepeatRepeat
	material.DiffuseMap.RepeatV = metadata.TextureRepeatRepeat
	material.DiffuseMap.RepeatW = metadata.TextureRepeatRepeat
	if !ms.renderer.TextureMapAcquireResources(material.DiffuseMap) {
		core.LogError("Unable to acquire resources for diffuse texture map.")
		return nil
	}
	if len(config.DiffuseMapName) > 0 {
		material.DiffuseMap.Use = metadata.TextureUseMapDiffuse
		t, err := ms.textureSystem.Acquire(config.DiffuseMapName, true)
		if err != nil {
			core.LogError(err.Error())
			return nil
		}
		material.DiffuseMap.Texture = t
		if material.DiffuseMap.Texture == nil {
			// Configured, but not found.
			core.LogWarn("Unable to load texture '%s' for material '%s', using default.", config.DiffuseMapName, material.Name)
			material.DiffuseMap.Texture = ms.textureSystem.GetDefaultTexture()
		}
	} else {
		// This is done when a texture is not configured, as opposed to when it is configured and not found (above).
		material.DiffuseMap.Use = metadata.TextureUseMapDiffuse
		material.DiffuseMap.Texture = ms.textureSystem.GetDefaultDiffuseTexture()
	}

	// Specular map
	// TODO: Make this configurable.
	material.SpecularMap.FilterMinify = metadata.TextureFilterModeLinear
	material.SpecularMap.FilterMagnify = metadata.TextureFilterModeLinear
	material.SpecularMap.RepeatU = metadata.TextureRepeatRepeat
	material.SpecularMap.RepeatV = metadata.TextureRepeatRepeat
	material.SpecularMap.RepeatW = metadata.TextureRepeatRepeat
	if !ms.renderer.TextureMapAcquireResources(material.SpecularMap) {
		core.LogError("Unable to acquire resources for specular texture map.")
		return nil
	}
	if len(config.SpecularMapName) > 0 {
		material.SpecularMap.Use = metadata.TextureUseMapSpecular
		t, err := ms.textureSystem.Acquire(config.SpecularMapName, true)
		if err != nil {
			core.LogError(err.Error())
			return nil
		}
		material.SpecularMap.Texture = t
		if material.SpecularMap.Texture == nil {
			core.LogWarn("Unable to load specular texture '%s' for material '%s', using default.", config.SpecularMapName, material.Name)
			material.SpecularMap.Texture = ms.textureSystem.GetDefaultSpecularTexture()
		}
	} else {
		// NOTE: Only set for clarity, as call to kzero_memory above does this already.
		material.SpecularMap.Use = metadata.TextureUseMapSpecular
		material.SpecularMap.Texture = ms.textureSystem.GetDefaultSpecularTexture()
	}

	// Normal map
	// TODO: Make this configurable.
	material.NormalMap.FilterMinify = metadata.TextureFilterModeLinear
	material.NormalMap.FilterMagnify = metadata.TextureFilterModeLinear
	material.NormalMap.RepeatU = metadata.TextureRepeatRepeat
	material.NormalMap.RepeatV = metadata.TextureRepeatRepeat
	material.NormalMap.RepeatW = metadata.TextureRepeatRepeat
	if !ms.renderer.TextureMapAcquireResources(material.NormalMap) {
		core.LogError("Unable to acquire resources for normal texture map.")
		return nil
	}
	if len(config.NormalMapName) > 0 {
		material.NormalMap.Use = metadata.TextureUseMapNormal
		t, err := ms.textureSystem.Acquire(config.NormalMapName, true)
		if err != nil {
			core.LogError(err.Error())
			return nil
		}
		material.NormalMap.Texture = t
		if material.NormalMap.Texture == nil {
			core.LogWarn("Unable to load normal texture '%s' for material '%s', using default.", config.NormalMapName, material.Name)
			material.NormalMap.Texture = ms.textureSystem.GetDefaultNormalTexture()
		}
	} else {
		// Use default
		material.NormalMap.Use = metadata.TextureUseMapNormal
		material.NormalMap.Texture = ms.textureSystem.GetDefaultNormalTexture()
	}

	// TODO: other maps

	// Send it off to the renderer to acquire resources.
	shader, err := ms.shaderSystem.GetShader(config.ShaderName)
	if err != nil {
		core.LogError("Unable to load material because its shader was not found: '%s'. This is likely a problem with the material asset.", config.ShaderName)
		return nil
	}

	// Gather a list of pointers to texture maps;
	texture_map := []*metadata.TextureMap{material.DiffuseMap, material.SpecularMap, material.NormalMap}
	material.InternalID = ms.renderer.ShaderAcquireInstanceResources(shader, texture_map)

	return material
}

func (ms *MaterialSystem) destroyMaterial(material *metadata.Material) {
	// KTRACE("Destroying material '%s'...", material.name);

	// Release texture references.
	if material.DiffuseMap.Texture != nil {
		ms.textureSystem.Release(material.DiffuseMap.Texture.Name)
	}
	if material.SpecularMap.Texture != nil {
		ms.textureSystem.Release(material.SpecularMap.Texture.Name)
	}
	if material.NormalMap.Texture != nil {
		ms.textureSystem.Release(material.NormalMap.Texture.Name)
	}

	// Release texture map resources.
	ms.renderer.TextureMapReleaseResources(material.DiffuseMap)
	ms.renderer.TextureMapReleaseResources(material.SpecularMap)
	ms.renderer.TextureMapReleaseResources(material.NormalMap)

	// Release renderer resources.
	if material.ShaderID != loaders.InvalidID && material.InternalID != loaders.InvalidID {
		shader, err := ms.shaderSystem.GetShaderByID(material.ShaderID)
		if err != nil {
			core.LogError(err.Error())
			return
		}
		if !ms.renderer.ShaderReleaseInstanceResources(shader, material.InternalID) {
			core.LogError("failed to release the shader instance resources")
		}
		material.ShaderID = loaders.InvalidID
	}

	// Zero it out, invalidate IDs.
	material.ID = loaders.InvalidID
	material.Generation = loaders.InvalidID
	material.InternalID = loaders.InvalidID
	material.RenderFrameNumber = loaders.InvalidID
}

func (ms *MaterialSystem) createDefaultMaterial() bool {
	ms.DefaultMaterial.ID = loaders.InvalidID
	ms.DefaultMaterial.Generation = loaders.InvalidID
	ms.DefaultMaterial.Name = metadata.DefaultMaterialName
	ms.DefaultMaterial.DiffuseColour = math.NewVec4Zero() // white
	ms.DefaultMaterial.DiffuseMap.Use = metadata.TextureUseMapDiffuse
	ms.DefaultMaterial.DiffuseMap.Texture = ms.textureSystem.GetDefaultTexture()

	ms.DefaultMaterial.SpecularMap.Use = metadata.TextureUseMapSpecular
	ms.DefaultMaterial.SpecularMap.Texture = ms.textureSystem.GetDefaultSpecularTexture()

	ms.DefaultMaterial.NormalMap.Use = metadata.TextureUseMapSpecular
	ms.DefaultMaterial.NormalMap.Texture = ms.textureSystem.GetDefaultNormalTexture()

	texture_maps := []*metadata.TextureMap{ms.DefaultMaterial.DiffuseMap, ms.DefaultMaterial.SpecularMap, ms.DefaultMaterial.NormalMap}

	shader, err := ms.shaderSystem.GetShader(metadata.BUILTIN_SHADER_NAME_MATERIAL)
	if err != nil {
		core.LogError(err.Error())
		return false
	}

	ms.DefaultMaterial.InternalID = ms.renderer.ShaderAcquireInstanceResources(shader, texture_maps)

	// Make sure to assign the shader id.
	ms.DefaultMaterial.ShaderID = shader.ID

	return true
}

func (ms *MaterialSystem) materialFail(expr string) bool {
	core.LogError("Failed to apply material: %s", expr)
	return false
}
