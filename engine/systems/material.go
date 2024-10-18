package systems

import (
	"fmt"
	"sync"

	"github.com/spaghettifunk/anima/engine/core"
	"github.com/spaghettifunk/anima/engine/math"
	"github.com/spaghettifunk/anima/engine/renderer"
	"github.com/spaghettifunk/anima/engine/renderer/metadata"
	"github.com/spaghettifunk/anima/engine/resources"
	"github.com/spaghettifunk/anima/engine/resources/loaders"
)

type MaterialSystemState struct {
	Config metadata.MaterialSystemConfig

	DefaultMaterial *resources.Material

	// Array of registered materials.
	RegisteredMaterials []*resources.Material

	// Hashtable for material lookups.
	RegisteredMaterialTable map[string]*metadata.MaterialReference

	// Known locations for the material shader.
	MaterialLocations *metadata.MaterialShaderUniformLocations
	MaterialShaderID  uint32

	// Known locations for the UI shader.
	UILocations *metadata.UIShaderUniformLocations
	UIShaderID  uint32
}

var onceMaterialSystem sync.Once
var msState *MaterialSystemState

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
func NewMaterialSystem(config *metadata.MaterialSystemConfig) bool {
	if config.MaxMaterialCount == 0 {
		core.LogError("func NewMaterialSystem - config.MaxMaterialCount must be > 0.")
		return false
	}

	defaultMaterialCreated := false

	onceMaterialSystem.Do(func() {
		msState = &MaterialSystemState{
			MaterialShaderID: loaders.InvalidID,
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
			RegisteredMaterials:     make([]*resources.Material, config.MaxMaterialCount),
			RegisteredMaterialTable: make(map[string]*metadata.MaterialReference),
		}

		// Fill the hashtable with invalid references to use as a default.
		invalid_ref := &metadata.MaterialReference{
			AutoRelease:    false,
			Handle:         loaders.InvalidID,
			ReferenceCount: 0,
		}

		// Invalidate all materials in the array.
		for i := uint32(0); i < config.MaxMaterialCount; i++ {
			msState.RegisteredMaterialTable[metadata.GenerateNewHash()] = invalid_ref
			msState.RegisteredMaterials[i].ID = loaders.InvalidID
			msState.RegisteredMaterials[i].Generation = loaders.InvalidID
			msState.RegisteredMaterials[i].InternalID = loaders.InvalidID
			msState.RegisteredMaterials[i].RenderFrameNumber = loaders.InvalidID
		}

		if !msState.createDefaultMaterial() {
			core.LogError("Failed to create default material. Application cannot continue.")
			defaultMaterialCreated = false
		}
		defaultMaterialCreated = true
	})

	return defaultMaterialCreated
}

/**
 * @brief Shuts down the material system.
 *
 * @param state The state block of memory.
 */
func MaterialSystemShutdown() {
	// Invalidate all materials in the array.
	for i := uint32(0); i < msState.Config.MaxMaterialCount; i++ {
		if msState.RegisteredMaterials[i].ID != loaders.InvalidID {
			msState.destroyMaterial(msState.RegisteredMaterials[i])
		}
	}
	// Destroy the default material.
	msState.destroyMaterial(msState.DefaultMaterial)
	msState = nil
}

/**
 * @brief Attempts to acquire a material with the given name. If it has not yet been loaded,
 * this triggers it to load. If the material is not found, a pointer to the default material
 * is returned. If the material _is_ found and loaded, its reference counter is incremented.
 *
 * @param name The name of the material to find.
 * @return A pointer to the loaded material. Can be a pointer to the default material if not found.
 */
func MaterialSystemAcquire(name string) (*resources.Material, error) {
	// Load material configuration from resource;
	materialResource, err := ResourceSystemLoad(name, resources.ResourceTypeMaterial, 0)
	if err != nil {
		err := fmt.Errorf("failed to load material resource, returning nullptr")
		core.LogError(err.Error())
		return nil, err
	}

	// Now acquire from loaded config.
	m := &resources.Material{}
	if materialResource.Data != nil {
		cfg, ok := materialResource.Data.(*resources.MaterialConfig)
		if !ok {
			err := fmt.Errorf("failed to cast to `*resources.MaterialConfig`")
			core.LogError(err.Error())
			return nil, err
		}
		m, err = MaterialSystemAcquireFromConfig(cfg)
		if err != nil {
			core.LogError(err.Error())
			return nil, err
		}
	}

	// Clean up
	ResourceSystemUnload(materialResource)

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
func MaterialSystemAcquireFromConfig(config *resources.MaterialConfig) (*resources.Material, error) {
	// Return default material.
	if config.Name == metadata.DefaultMaterialName {
		return msState.DefaultMaterial, nil
	}

	ref := msState.RegisteredMaterialTable[config.Name]

	// This can only be changed the first time a material is loaded.
	if ref.ReferenceCount == 0 {
		ref.AutoRelease = config.AutoRelease
	}
	ref.ReferenceCount++
	if ref.Handle == loaders.InvalidID {
		// This means no material exists here. Find a free index first.
		count := msState.Config.MaxMaterialCount
		var material *resources.Material
		for i := uint32(0); i < count; i++ {
			if msState.RegisteredMaterials[i].ID == loaders.InvalidID {
				// A free slot has been found. Use its index as the handle.
				ref.Handle = i
				material = msState.RegisteredMaterials[i]
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
		material = msState.loadMaterial(config)
		if material == nil {
			err := fmt.Errorf("failed to load material '%s'", config.Name)
			core.LogError(err.Error())
			return nil, err
		}

		// Get the uniform indices.
		shader, err := ShaderSystemGetShaderByID(material.ShaderID)
		if err != nil {
			core.LogError(err.Error())
			return nil, err
		}
		// Save off the locations for known types for quick lookups.
		if msState.MaterialShaderID == loaders.InvalidID && config.ShaderName == metadata.BUILTIN_SHADER_NAME_MATERIAL {
			msState.MaterialShaderID = shader.ID
			msState.MaterialLocations.Projection = ShaderSystemGetUniformIndex(shader, "projection")
			msState.MaterialLocations.View = ShaderSystemGetUniformIndex(shader, "view")
			msState.MaterialLocations.AmbientColour = ShaderSystemGetUniformIndex(shader, "ambient_colour")
			msState.MaterialLocations.ViewPosition = ShaderSystemGetUniformIndex(shader, "view_position")
			msState.MaterialLocations.DiffuseColour = ShaderSystemGetUniformIndex(shader, "diffuse_colour")
			msState.MaterialLocations.DiffuseTexture = ShaderSystemGetUniformIndex(shader, "diffuse_texture")
			msState.MaterialLocations.SpecularTexture = ShaderSystemGetUniformIndex(shader, "specular_texture")
			msState.MaterialLocations.NormalTexture = ShaderSystemGetUniformIndex(shader, "normal_texture")
			msState.MaterialLocations.Shininess = ShaderSystemGetUniformIndex(shader, "shininess")
			msState.MaterialLocations.Model = ShaderSystemGetUniformIndex(shader, "model")
			msState.MaterialLocations.RenderMode = ShaderSystemGetUniformIndex(shader, "mode")
		} else if msState.UIShaderID == loaders.InvalidID && config.ShaderName == metadata.BUILTIN_SHADER_NAME_UI {
			msState.UIShaderID = shader.ID
			msState.UILocations.Projection = ShaderSystemGetUniformIndex(shader, "projection")
			msState.UILocations.View = ShaderSystemGetUniformIndex(shader, "view")
			msState.UILocations.DiffuseColour = ShaderSystemGetUniformIndex(shader, "diffuse_colour")
			msState.UILocations.DiffuseTexture = ShaderSystemGetUniformIndex(shader, "diffuse_texture")
			msState.UILocations.Model = ShaderSystemGetUniformIndex(shader, "model")
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
	msState.RegisteredMaterialTable[config.Name] = ref
	return msState.RegisteredMaterials[ref.Handle], nil
}

/**
 * @brief Releases a material with the given name. Ignores non-existant materials.
 * Decreases the reference counter by 1. If the reference counter reaches 0 and
 * AutoRelease was set to true, the material is unloaded, releasing internal resources.
 *
 * @param name The name of the material to unload.
 */
func MaterialSystemRelease(name string) {
	// Ignore release requests for the default material.
	if name == metadata.DefaultMaterialName {
		return
	}
	ref := msState.RegisteredMaterialTable[name]
	if ref != nil {
		if ref.ReferenceCount == 0 {
			core.LogWarn("Tried to release non-existent material: '%s'", name)
			return
		}
		ref.ReferenceCount--
		if ref.ReferenceCount == 0 && ref.AutoRelease {
			material := msState.RegisteredMaterials[ref.Handle]
			// Destroy/reset material.
			msState.destroyMaterial(material)
			// Reset the reference.
			ref.Handle = loaders.InvalidID
			ref.AutoRelease = false
			// KTRACE("Released material '%s'., Material unloaded because reference count=0 and AutoRelease=true.", name);
		} else {
			// KTRACE("Released material '%s', now has a reference count of '%i' (AutoRelease=%s).", name, ref.ReferenceCount, ref.AutoRelease ? "true" : "false");
		}
		// Update the entry.
		msState.RegisteredMaterialTable[name] = ref
	} else {
		core.LogError("material_system_release failed to release material '%s'", name)
	}
}

/**
 * @brief Gets a pointer to the default material. Does not reference count.
 */
func MaterialSystemGetDefault() *resources.Material {
	return msState.DefaultMaterial
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
func MaterialSystemApplyGlobal(shaderID uint32, renderer_frame_number uint64, projection []math.Mat4, view []math.Mat4, ambient_colour []math.Vec3, view_position []math.Vec3, render_mode uint32) bool {
	shader, err := ShaderSystemGetShaderByID(shaderID)
	if err != nil {
		core.LogError(err.Error())
		return false
	}
	if shader.RenderFrameNumber == renderer_frame_number {
		return true
	}
	if shaderID == msState.MaterialShaderID {
		if ok := ShaderSystemSetUniformByIndex(msState.MaterialLocations.Projection, projection); !ok {
			return msState.materialFail("msState.MaterialLocations.Projection")
		}
		if ok := ShaderSystemSetUniformByIndex(msState.MaterialLocations.View, view); !ok {
			return msState.materialFail("msState.MaterialLocations.View")
		}
		if ok := ShaderSystemSetUniformByIndex(msState.MaterialLocations.AmbientColour, ambient_colour); !ok {
			return msState.materialFail("msState.MaterialLocations.AmbientColour")
		}
		if ok := ShaderSystemSetUniformByIndex(msState.MaterialLocations.ViewPosition, view_position); !ok {
			return msState.materialFail("msState.MaterialLocations.ViewPosition")
		}
		if ok := ShaderSystemSetUniformByIndex(msState.MaterialLocations.RenderMode, &render_mode); !ok {
			return msState.materialFail("msState.MaterialLocations.RenderMode")
		}
	} else if shaderID == msState.UIShaderID {
		if ok := ShaderSystemSetUniformByIndex(msState.UILocations.Projection, projection); !ok {
			return msState.materialFail("msState.UILocations.Projection")
		}
		if ok := ShaderSystemSetUniformByIndex(msState.UILocations.View, view); !ok {
			return msState.materialFail("msState.UILocations.View")
		}
	} else {
		core.LogError("func MaterialSystemApplyGlobal(): Unrecognized shader id '%d' ", shaderID)
		return false
	}
	ShaderSystemApplyGlobal()

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
func MaterialSystemApplyInstance(material *resources.Material, needsUpdate bool) bool {
	// Apply instance-level uniforms.
	if ok := ShaderSystemBindInstance(material.InternalID); !ok {
		return msState.materialFail("material.InternalID")
	}
	if needsUpdate {
		if material.ShaderID == msState.MaterialShaderID {
			// Material shader
			if ok := ShaderSystemSetUniformByIndex(msState.MaterialLocations.DiffuseColour, &material.DiffuseColour); !ok {
				return msState.materialFail("msState.MaterialLocations.DiffuseColour")
			}
			if ok := ShaderSystemSetUniformByIndex(msState.MaterialLocations.DiffuseTexture, &material.DiffuseMap); !ok {
				return msState.materialFail("msState.MaterialLocations.DiffuseTexture")
			}
			if ok := ShaderSystemSetUniformByIndex(msState.MaterialLocations.SpecularTexture, &material.SpecularMap); !ok {
				return msState.materialFail("msState.MaterialLocations.SpecularTexture")
			}
			if ok := ShaderSystemSetUniformByIndex(msState.MaterialLocations.NormalTexture, &material.NormalMap); !ok {
				return msState.materialFail("msState.MaterialLocations.NormalTexture")
			}
			if ok := ShaderSystemSetUniformByIndex(msState.MaterialLocations.Shininess, &material.Shininess); !ok {
				return msState.materialFail("msState.MaterialLocations.Shininess")
			}
		} else if material.ShaderID == msState.UIShaderID {
			// UI shader
			if ok := ShaderSystemSetUniformByIndex(msState.UILocations.DiffuseColour, &material.DiffuseColour); !ok {
				return msState.materialFail("msState.UILocations.DiffuseColour")
			}
			if ok := ShaderSystemSetUniformByIndex(msState.UILocations.DiffuseTexture, &material.DiffuseMap); !ok {
				return msState.materialFail("msState.UILocations.DiffuseTexture")
			}
		} else {
			core.LogError("material_system_apply_instance(): Unrecognized shader id '%d' on shader '%s'.", material.ShaderID, material.Name)
			return false
		}
	}
	if ok := ShaderSystemApplyInstance(needsUpdate); !ok {
		return msState.materialFail("needsUpdate")
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
func MaterialSystemApplyLocal(material *resources.Material, model [][]math.Mat4) bool {
	if material.ShaderID == msState.MaterialShaderID {
		return ShaderSystemSetUniformByIndex(msState.MaterialLocations.Model, model)
	} else if material.ShaderID == msState.UIShaderID {
		return ShaderSystemSetUniformByIndex(msState.UILocations.Model, model)
	}
	core.LogError("Unrecognized shader id '%d'", material.ShaderID)
	return false
}

func (ms *MaterialSystemState) loadMaterial(config *resources.MaterialConfig) *resources.Material {
	material := &resources.Material{}

	material.Name = config.Name

	material.ShaderID = ShaderSystemGetShaderID(config.ShaderName)

	// Diffuse colour
	material.DiffuseColour = config.DiffuseColour
	material.Shininess = config.Shininess

	// Diffuse map
	// TODO: Make this configurable.
	// TODO: DRY
	material.DiffuseMap.FilterMinify = resources.TextureFilterModeLinear
	material.DiffuseMap.FilterMagnify = resources.TextureFilterModeLinear
	material.DiffuseMap.RepeatU = resources.TextureRepeatRepeat
	material.DiffuseMap.RepeatV = resources.TextureRepeatRepeat
	material.DiffuseMap.RepeatW = resources.TextureRepeatRepeat
	if !renderer.TextureMapAcquireResources(material.DiffuseMap) {
		core.LogError("Unable to acquire resources for diffuse texture map.")
		return nil
	}
	if len(config.DiffuseMapName) > 0 {
		material.DiffuseMap.Use = resources.TextureUseMapDiffuse
		t, err := TextureSystemAcquire(config.DiffuseMapName, true)
		if err != nil {
			core.LogError(err.Error())
			return nil
		}
		material.DiffuseMap.Texture = t
		if material.DiffuseMap.Texture == nil {
			// Configured, but not found.
			core.LogWarn("Unable to load texture '%s' for material '%s', using default.", config.DiffuseMapName, material.Name)
			material.DiffuseMap.Texture = TextureSystemGetDefaultTexture()
		}
	} else {
		// This is done when a texture is not configured, as opposed to when it is configured and not found (above).
		material.DiffuseMap.Use = resources.TextureUseMapDiffuse
		material.DiffuseMap.Texture = TextureSystemGetDefaultDiffuseTexture()
	}

	// Specular map
	// TODO: Make this configurable.
	material.SpecularMap.FilterMinify = resources.TextureFilterModeLinear
	material.SpecularMap.FilterMagnify = resources.TextureFilterModeLinear
	material.SpecularMap.RepeatU = resources.TextureRepeatRepeat
	material.SpecularMap.RepeatV = resources.TextureRepeatRepeat
	material.SpecularMap.RepeatW = resources.TextureRepeatRepeat
	if !renderer.TextureMapAcquireResources(material.SpecularMap) {
		core.LogError("Unable to acquire resources for specular texture map.")
		return nil
	}
	if len(config.SpecularMapName) > 0 {
		material.SpecularMap.Use = resources.TextureUseMapSpecular
		t, err := TextureSystemAcquire(config.SpecularMapName, true)
		if err != nil {
			core.LogError(err.Error())
			return nil
		}
		material.SpecularMap.Texture = t
		if material.SpecularMap.Texture == nil {
			core.LogWarn("Unable to load specular texture '%s' for material '%s', using default.", config.SpecularMapName, material.Name)
			material.SpecularMap.Texture = TextureSystemGetDefaultSpecularTexture()
		}
	} else {
		// NOTE: Only set for clarity, as call to kzero_memory above does this already.
		material.SpecularMap.Use = resources.TextureUseMapSpecular
		material.SpecularMap.Texture = TextureSystemGetDefaultSpecularTexture()
	}

	// Normal map
	// TODO: Make this configurable.
	material.NormalMap.FilterMinify = resources.TextureFilterModeLinear
	material.NormalMap.FilterMagnify = resources.TextureFilterModeLinear
	material.NormalMap.RepeatU = resources.TextureRepeatRepeat
	material.NormalMap.RepeatV = resources.TextureRepeatRepeat
	material.NormalMap.RepeatW = resources.TextureRepeatRepeat
	if !renderer.TextureMapAcquireResources(material.NormalMap) {
		core.LogError("Unable to acquire resources for normal texture map.")
		return nil
	}
	if len(config.NormalMapName) > 0 {
		material.NormalMap.Use = resources.TextureUseMapNormal
		t, err := TextureSystemAcquire(config.NormalMapName, true)
		if err != nil {
			core.LogError(err.Error())
			return nil
		}
		material.NormalMap.Texture = t
		if material.NormalMap.Texture == nil {
			core.LogWarn("Unable to load normal texture '%s' for material '%s', using default.", config.NormalMapName, material.Name)
			material.NormalMap.Texture = TextureSystemGetDefaultNormalTexture()
		}
	} else {
		// Use default
		material.NormalMap.Use = resources.TextureUseMapNormal
		material.NormalMap.Texture = TextureSystemGetDefaultNormalTexture()
	}

	// TODO: other maps

	// Send it off to the renderer to acquire resources.
	shader, err := ShaderSystemGetShader(config.ShaderName)
	if err != nil {
		core.LogError("Unable to load material because its shader was not found: '%s'. This is likely a problem with the material asset.", config.ShaderName)
		return nil
	}

	// Gather a list of pointers to texture maps;
	texture_map := []*resources.TextureMap{material.DiffuseMap, material.SpecularMap, material.NormalMap}
	material.InternalID = renderer.ShaderAcquireInstanceResources(shader, texture_map)

	return material
}

func (ms *MaterialSystemState) destroyMaterial(material *resources.Material) {
	// KTRACE("Destroying material '%s'...", material.name);

	// Release texture references.
	if material.DiffuseMap.Texture != nil {
		TextureSystemRelease(material.DiffuseMap.Texture.Name)
	}
	if material.SpecularMap.Texture != nil {
		TextureSystemRelease(material.SpecularMap.Texture.Name)
	}
	if material.NormalMap.Texture != nil {
		TextureSystemRelease(material.NormalMap.Texture.Name)
	}

	// Release texture map resources.
	renderer.TextureMapReleaseResources(material.DiffuseMap)
	renderer.TextureMapReleaseResources(material.SpecularMap)
	renderer.TextureMapReleaseResources(material.NormalMap)

	// Release renderer resources.
	if material.ShaderID != loaders.InvalidID && material.InternalID != loaders.InvalidID {
		shader, err := ShaderSystemGetShaderByID(material.ShaderID)
		if err != nil {
			core.LogError(err.Error())
			return
		}
		if !renderer.ShaderReleaseInstanceResources(shader, material.InternalID) {
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

func (ms *MaterialSystemState) createDefaultMaterial() bool {
	ms.DefaultMaterial.ID = loaders.InvalidID
	ms.DefaultMaterial.Generation = loaders.InvalidID
	ms.DefaultMaterial.Name = metadata.DefaultMaterialName
	ms.DefaultMaterial.DiffuseColour = math.NewVec4Zero() // white
	ms.DefaultMaterial.DiffuseMap.Use = resources.TextureUseMapDiffuse
	ms.DefaultMaterial.DiffuseMap.Texture = TextureSystemGetDefaultTexture()

	ms.DefaultMaterial.SpecularMap.Use = resources.TextureUseMapSpecular
	ms.DefaultMaterial.SpecularMap.Texture = TextureSystemGetDefaultSpecularTexture()

	ms.DefaultMaterial.NormalMap.Use = resources.TextureUseMapSpecular
	ms.DefaultMaterial.NormalMap.Texture = TextureSystemGetDefaultNormalTexture()

	texture_maps := []*resources.TextureMap{ms.DefaultMaterial.DiffuseMap, ms.DefaultMaterial.SpecularMap, ms.DefaultMaterial.NormalMap}

	shader, err := ShaderSystemGetShader(metadata.BUILTIN_SHADER_NAME_MATERIAL)
	if err != nil {
		core.LogError(err.Error())
		return false
	}

	ms.DefaultMaterial.InternalID = renderer.ShaderAcquireInstanceResources(shader, texture_maps)

	// Make sure to assign the shader id.
	ms.DefaultMaterial.ShaderID = shader.ID

	return true
}

func (ms *MaterialSystemState) materialFail(expr string) bool {
	core.LogError("Failed to apply material: %s", expr)
	return false
}
