package systems

import (
	"fmt"

	"github.com/spaghettifunk/anima/engine/assets"
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
		err := fmt.Errorf("func NewMaterialSystem - config.MaxMaterialCount must be > 0")
		return nil, err
	}

	ms := &MaterialSystem{
		MaterialShaderID: metadata.InvalidID,
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
			View:            metadata.InvalidIDUint16,
			Projection:      metadata.InvalidIDUint16,
			DiffuseColour:   metadata.InvalidIDUint16,
			DiffuseTexture:  metadata.InvalidIDUint16,
			SpecularTexture: metadata.InvalidIDUint16,
			NormalTexture:   metadata.InvalidIDUint16,
			AmbientColour:   metadata.InvalidIDUint16,
			Shininess:       metadata.InvalidIDUint16,
			Model:           metadata.InvalidIDUint16,
			RenderMode:      metadata.InvalidIDUint16,
		},
		UIShaderID: metadata.InvalidID,
		UILocations: &metadata.UIShaderUniformLocations{
			DiffuseColour:  metadata.InvalidIDUint16,
			DiffuseTexture: metadata.InvalidIDUint16,
			View:           metadata.InvalidIDUint16,
			Projection:     metadata.InvalidIDUint16,
			Model:          metadata.InvalidIDUint16,
		},
		RegisteredMaterials:     make([]*metadata.Material, config.MaxMaterialCount),
		RegisteredMaterialTable: make(map[string]*metadata.MaterialReference),
		shaderSystem:            shaderSytem,
		textureSystem:           ts,
		assetManager:            am,
		renderer:                r,
		Config:                  config,
	}

	// Invalidate all materials in the array.
	for i := uint32(0); i < config.MaxMaterialCount; i++ {
		ms.RegisteredMaterials[i] = &metadata.Material{
			ID:                metadata.InvalidID,
			Generation:        metadata.InvalidID,
			InternalID:        metadata.InvalidID,
			RenderFrameNumber: metadata.InvalidID,
		}
	}
	return ms, nil
}

func (ms *MaterialSystem) Initialize() error {
	if !ms.createDefaultMaterial() {
		err := fmt.Errorf("failed to create default material. Application cannot continue")
		return err
	}
	return nil
}

/**
 * @brief Shuts down the material system.
 *
 * @param state The state block of memory.
 */
func (ms *MaterialSystem) Shutdown() error {
	// Invalidate all materials in the array.
	for i := uint32(0); i < ms.Config.MaxMaterialCount; i++ {
		if ms.RegisteredMaterials[i].ID != metadata.InvalidID {
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

	if m == nil {
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

	ref := &metadata.MaterialReference{
		Handle: metadata.InvalidID,
	}

	// This can only be changed the first time a material is loaded.
	if ref.ReferenceCount == 0 {
		ref.AutoRelease = config.AutoRelease
	}
	ref.ReferenceCount++
	if ref.Handle == metadata.InvalidID {
		// This means no material exists here. Find a free index first.
		count := ms.Config.MaxMaterialCount
		var material *metadata.Material
		for i := uint32(0); i < count; i++ {
			if ms.RegisteredMaterials[i].ID == metadata.InvalidID {
				// A free slot has been found. Use its index as the handle.
				ref.Handle = i
				material = ms.RegisteredMaterials[i]
				break
			}
		}

		// Make sure an empty slot was actually found.
		if material == nil || ref.Handle == metadata.InvalidID {
			err := fmt.Errorf("material_system_acquire - Material system cannot hold anymore materials. Adjust configuration to allow more")
			core.LogError(err.Error())
			return nil, err
		}

		// Create new material.
		material, err := ms.loadMaterial(config)
		if err != nil {
			return nil, err
		}

		// Get the uniform indices.
		shader, err := ms.shaderSystem.GetShaderByID(material.ShaderID)
		if err != nil {
			core.LogError(err.Error())
			return nil, err
		}
		// Save off the locations for known types for quick lookups.
		if ms.MaterialShaderID == metadata.InvalidID && config.ShaderName == "Shader.Builtin.Material" {
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
		} else if ms.UIShaderID == metadata.InvalidID && config.ShaderName == "Shader.Builtin.UI" {
			ms.UIShaderID = shader.ID
			ms.UILocations.Projection = ms.shaderSystem.GetUniformIndex(shader, "projection")
			ms.UILocations.View = ms.shaderSystem.GetUniformIndex(shader, "view")
			ms.UILocations.DiffuseColour = ms.shaderSystem.GetUniformIndex(shader, "diffuse_colour")
			ms.UILocations.DiffuseTexture = ms.shaderSystem.GetUniformIndex(shader, "diffuse_texture")
			ms.UILocations.Model = ms.shaderSystem.GetUniformIndex(shader, "model")
		}

		if material.Generation == metadata.InvalidID {
			material.Generation = 0
		} else {
			material.Generation++
		}

		// Also use the handle as the material id.
		material.ID = ref.Handle
		ms.RegisteredMaterials[ref.Handle] = material
		core.LogDebug("material '%s' does not yet exist. Created, and ref_count is now %d", config.Name, ref.ReferenceCount)
	} else {
		core.LogDebug("material '%s' already exists, ref_count increased to %d", config.Name, ref.ReferenceCount)
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
			ref.Handle = metadata.InvalidID
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
func (ms *MaterialSystem) ApplyGlobal(shaderID uint32, renderer_frame_number uint64, projection math.Mat4, view math.Mat4, ambient_colour math.Vec3, view_position math.Vec3, render_mode uint32) bool {
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
func (ms *MaterialSystem) ApplyLocal(material *metadata.Material, model math.Mat4) bool {
	if material.ShaderID == ms.MaterialShaderID {
		return ms.shaderSystem.SetUniformByIndex(ms.MaterialLocations.Model, model)
	} else if material.ShaderID == ms.UIShaderID {
		return ms.shaderSystem.SetUniformByIndex(ms.UILocations.Model, model)
	}
	core.LogError("Unrecognized shader id '%d'", material.ShaderID)
	return false
}

func (ms *MaterialSystem) loadMaterial(config *metadata.MaterialConfig) (*metadata.Material, error) {
	material := &metadata.Material{
		Name:          config.Name,
		ShaderID:      ms.shaderSystem.GetShaderID(config.ShaderName),
		DiffuseColour: config.DiffuseColour,
		Shininess:     config.Shininess,
		DiffuseMap: &metadata.TextureMap{
			FilterMinify:  metadata.TextureFilterModeLinear,
			FilterMagnify: metadata.TextureFilterModeLinear,
			RepeatU:       metadata.TextureRepeatRepeat,
			RepeatV:       metadata.TextureRepeatRepeat,
			RepeatW:       metadata.TextureRepeatRepeat,
		},
		SpecularMap: &metadata.TextureMap{
			FilterMinify:  metadata.TextureFilterModeLinear,
			FilterMagnify: metadata.TextureFilterModeLinear,
			RepeatU:       metadata.TextureRepeatRepeat,
			RepeatV:       metadata.TextureRepeatRepeat,
			RepeatW:       metadata.TextureRepeatRepeat,
			InternalData:  new(interface{}),
		},
		NormalMap: &metadata.TextureMap{
			FilterMinify:  metadata.TextureFilterModeLinear,
			FilterMagnify: metadata.TextureFilterModeLinear,
			RepeatU:       metadata.TextureRepeatRepeat,
			RepeatV:       metadata.TextureRepeatRepeat,
			RepeatW:       metadata.TextureRepeatRepeat,
		},
	}

	// Diffuse map
	if !ms.renderer.TextureMapAcquireResources(material.DiffuseMap) {
		err := fmt.Errorf("unable to acquire resources for diffuse texture map")
		return nil, err
	}
	if len(config.DiffuseMapName) > 0 {
		material.DiffuseMap.Use = metadata.TextureUseMapDiffuse
		t, err := ms.textureSystem.Acquire(config.DiffuseMapName, true)
		if err != nil {
			return nil, err
		}
		material.DiffuseMap.Texture = t
		if material.DiffuseMap.Texture == nil {
			// Configured, but not found.
			core.LogWarn("unable to load texture '%s' for material '%s', using default", config.DiffuseMapName, material.Name)
			material.DiffuseMap.Texture = ms.textureSystem.GetDefaultTexture()
		}
	} else {
		// This is done when a texture is not configured, as opposed to when it is configured and not found (above).
		material.DiffuseMap.Use = metadata.TextureUseMapDiffuse
		material.DiffuseMap.Texture = ms.textureSystem.GetDefaultDiffuseTexture()
	}

	// Specular map
	if !ms.renderer.TextureMapAcquireResources(material.SpecularMap) {
		err := fmt.Errorf("unable to acquire resources for specular texture map")
		return nil, err
	}
	if len(config.SpecularMapName) > 0 {
		material.SpecularMap.Use = metadata.TextureUseMapSpecular
		t, err := ms.textureSystem.Acquire(config.SpecularMapName, true)
		if err != nil {
			return nil, err
		}
		material.SpecularMap.Texture = t
		if material.SpecularMap.Texture == nil {
			core.LogWarn("unable to load specular texture '%s' for material '%s', using default", config.SpecularMapName, material.Name)
			material.SpecularMap.Texture = ms.textureSystem.GetDefaultSpecularTexture()
		}
	} else {
		// NOTE: Only set for clarity, as call to kzero_memory above does this already.
		material.SpecularMap.Use = metadata.TextureUseMapSpecular
		material.SpecularMap.Texture = ms.textureSystem.GetDefaultSpecularTexture()
	}

	// Normal map
	if !ms.renderer.TextureMapAcquireResources(material.NormalMap) {
		err := fmt.Errorf("unable to acquire resources for normal texture map")
		return nil, err
	}
	if len(config.NormalMapName) > 0 {
		material.NormalMap.Use = metadata.TextureUseMapNormal
		t, err := ms.textureSystem.Acquire(config.NormalMapName, true)
		if err != nil {
			return nil, err
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
		return nil, err
	}

	// Gather a list of pointers to texture maps;
	texture_map := []*metadata.TextureMap{material.DiffuseMap, material.SpecularMap, material.NormalMap}
	material.InternalID, err = ms.renderer.ShaderAcquireInstanceResources(shader, texture_map)
	if err != nil {
		return nil, err
	}

	return material, nil
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
	if material.ShaderID != metadata.InvalidID && material.InternalID != metadata.InvalidID {
		shader, err := ms.shaderSystem.GetShaderByID(material.ShaderID)
		if err != nil {
			core.LogError(err.Error())
			return
		}
		if !ms.renderer.ShaderReleaseInstanceResources(shader, material.InternalID) {
			core.LogError("failed to release the shader instance resources")
		}
		material.ShaderID = metadata.InvalidID
	}

	// Zero it out, invalidate IDs.
	material.ID = metadata.InvalidID
	material.Generation = metadata.InvalidID
	material.InternalID = metadata.InvalidID
	material.RenderFrameNumber = metadata.InvalidID
}

func (ms *MaterialSystem) createDefaultMaterial() bool {
	ms.DefaultMaterial.ID = metadata.InvalidID
	ms.DefaultMaterial.Generation = metadata.InvalidID
	ms.DefaultMaterial.Name = metadata.DefaultMaterialName
	ms.DefaultMaterial.DiffuseColour = math.NewVec4Zero() // white
	ms.DefaultMaterial.DiffuseMap.Use = metadata.TextureUseMapDiffuse
	ms.DefaultMaterial.DiffuseMap.Texture = ms.textureSystem.GetDefaultTexture()

	ms.DefaultMaterial.SpecularMap.Use = metadata.TextureUseMapSpecular
	ms.DefaultMaterial.SpecularMap.Texture = ms.textureSystem.GetDefaultSpecularTexture()

	ms.DefaultMaterial.NormalMap.Use = metadata.TextureUseMapSpecular
	ms.DefaultMaterial.NormalMap.Texture = ms.textureSystem.GetDefaultNormalTexture()

	texture_maps := []*metadata.TextureMap{ms.DefaultMaterial.DiffuseMap, ms.DefaultMaterial.SpecularMap, ms.DefaultMaterial.NormalMap}

	shader, err := ms.shaderSystem.GetShader("Shader.Builtin.Material")
	if err != nil {
		core.LogError(err.Error())
		return false
	}

	ms.DefaultMaterial.InternalID, err = ms.renderer.ShaderAcquireInstanceResources(shader, texture_maps)
	if err != nil {
		core.LogError(err.Error())
		return false
	}

	// Make sure to assign the shader id.
	ms.DefaultMaterial.ShaderID = shader.ID

	return true
}

func (ms *MaterialSystem) materialFail(expr string) bool {
	core.LogError("Failed to apply material: %s", expr)
	return false
}
