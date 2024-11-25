package systems

import (
	"fmt"
	"math"

	"github.com/spaghettifunk/anima/engine/core"
	"github.com/spaghettifunk/anima/engine/renderer/metadata"
)

/** @brief Configuration for the shader system. */
type ShaderSystemConfig struct {
	/** @brief The maximum number of shaders held in the system. NOTE: Should be at least 512. */
	MaxShaderCount uint16
	/** @brief The maximum number of uniforms allowed in a single shader. */
	MaxUniformCount uint8
	/** @brief The maximum number of global-scope textures allowed in a single shader. */
	MaxGlobalTextures uint8
	/** @brief The maximum number of instance-scope textures allowed in a single shader. */
	MaxInstanceTextures uint8
}

type ShaderSystem struct {
	// This system's configuration.
	Config *ShaderSystemConfig
	// A lookup table for shader name->id
	Lookup map[string]uint32
	// The identifier for the currently bound shader.
	CurrentShaderID uint32
	// A collection of created shaders.
	Shaders []*metadata.Shader
	// sub systems
	textureSystem *TextureSystem
	renderer      *RendererSystem
}

func NewShaderSystem(config *ShaderSystemConfig, ts *TextureSystem, r *RendererSystem) (*ShaderSystem, error) {
	// Verify configuration.
	if config.MaxShaderCount < 512 {
		if config.MaxShaderCount == 0 {
			err := fmt.Errorf("NewShaderSystem - config.MaxShaderCount must be greater than 0")
			core.LogError(err.Error())
			return nil, err
		} else {
			// This is to help avoid hashtable collisions.
			core.LogWarn("NewShaderSystem - config.MaxShaderCount is recommended to be at least 512.")
		}
	}

	// Setup the state pointer, memory block, shader array, then create the hashtable.
	shaderSystem := &ShaderSystem{
		Config:          config,
		Shaders:         make([]*metadata.Shader, config.MaxShaderCount),
		CurrentShaderID: metadata.InvalidID,
		Lookup:          make(map[string]uint32),
		textureSystem:   ts,
		renderer:        r,
	}

	// Invalidate all shader ids.
	for i := uint16(0); i < config.MaxShaderCount; i++ {
		shaderSystem.Shaders[i] = &metadata.Shader{
			ID:                metadata.InvalidID,
			RenderFrameNumber: metadata.InvalidIDUint64,
		}
	}

	return shaderSystem, nil
}

func (shaderSystem *ShaderSystem) Initialize() error {
	return nil
}

/**
 * @brief Shuts down the shader system.
 *
 * @param state A pointer to the system state.
 */
func (shaderSystem *ShaderSystem) Shutdown() error {
	// Destroy any shaders still in existence.
	for i := uint16(0); i < shaderSystem.Config.MaxShaderCount; i++ {
		sh := shaderSystem.Shaders[i]
		if sh.ID != metadata.InvalidID {
			if err := shaderSystem.shaderDestroy(sh); err != nil {
				core.LogError(err.Error())
				return err
			}
		}
	}
	return nil
}

/**
 * @brief Creates a new shader with the given config.
 *
 * @param config The configuration to be used when creating the shader.
 * @return True on success; otherwise false.
 */
func (shaderSystem *ShaderSystem) CreateShader(pass *metadata.RenderPass, config *metadata.ShaderConfig, initialize bool) (*metadata.Shader, error) {
	id := shaderSystem.newShaderID()

	shader := shaderSystem.Shaders[id]
	shader.ID = id

	if shader.ID == metadata.InvalidID {
		err := fmt.Errorf("unable to find free slot to create new shader. Aborting")
		core.LogError(err.Error())
		return nil, err
	}

	shader.State = metadata.SHADER_STATE_NOT_CREATED
	shader.Name = config.Name
	shader.PushConstantRangeCount = 0
	shader.BoundInstanceID = metadata.InvalidID
	shader.AttributeStride = 0
	shader.UniformLookup = make(map[string]uint16)

	// Setup arrays
	shader.GlobalTextureMaps = make([]*metadata.TextureMap, shaderSystem.Config.MaxGlobalTextures)
	shader.Uniforms = []metadata.ShaderUniform{}
	shader.Attributes = []metadata.ShaderAttribute{}

	// A running total of the actual global uniform buffer object size.
	shader.GlobalUboSize = 0
	// A running total of the actual instance uniform buffer object size.
	shader.UboSize = 0
	// NOTE: UBO alignment requirement set in renderer backend.

	// This is hard-coded because the Vulkan spec only guarantees that a _minimum_ 128 bytes of space are available,
	// and it's up to the driver to determine how much is available. Therefore, to avoid complexity, only the
	// lowest common denominator of 128B will be used.
	shader.PushConstantStride = 128
	shader.PushConstantSize = 0

	// Process flags.
	shader.Flags = 0
	if config.DepthTest {
		shader.Flags |= metadata.ShaderFlagBits(metadata.SHADER_FLAG_DEPTH_TEST)
	}
	if config.DepthTest {
		shader.Flags |= metadata.ShaderFlagBits(metadata.SHADER_FLAG_DEPTH_WRITE)
	}

	if !shaderSystem.renderer.ShaderCreate(shader, config, pass, uint8(len(config.Stages)), config.StageFilenames, config.Stages) {
		err := fmt.Errorf("shader was not created")
		core.LogError(err.Error())
		return nil, err
	}

	// Ready to be initialized.
	shader.State = metadata.SHADER_STATE_UNINITIALIZED

	// Process attributes
	for i := 0; i < len(config.Attributes); i++ {
		shaderSystem.addAttribute(shader, config.Attributes[i])
	}

	// Process uniforms
	for i := 0; i < len(config.Uniforms); i++ {
		_, ok := shader.UniformLookup[config.Uniforms[i].Name]
		if !ok {
			shader.UniformLookup[config.Uniforms[i].Name] = metadata.InvalidIDUint16
		}
		if config.Uniforms[i].ShaderUniformType == metadata.ShaderUniformTypeSampler {
			shaderSystem.addSampler(shader, config.Uniforms[i])
		} else {
			shaderSystem.addUniform(shader, config.Uniforms[i])
		}
	}

	// Initialize the shader.
	if initialize {
		if err := shaderSystem.renderer.ShaderInitialize(shader); err != nil {
			core.LogError("func ShaderInitialize: initialization failed for shader '%s'", config.Name)
			// NOTE: initialize automatically destroys the shader if it fails.
			return nil, err
		}
	}

	// At this point, creation is successful, so store the shader id in the hashtable
	// so this can be looked up by name later.
	shaderSystem.Lookup[config.Name] = shader.ID

	return shader, nil
}

/**
 * @brief Gets the identifier of a shader by name.
 *
 * @param shaderName The name of the shader.
 * @return The shader id, if found; otherwise INVALID_ID.
 */
func (shaderSystem *ShaderSystem) GetShaderID(shaderName string) uint32 {
	return shaderSystem.getShaderID(shaderName)
}

/**
 * @brief Returns a pointer to a shader with the given identifier.
 *
 * @param shaderID The shader identifier.
 * @return A pointer to a shader, if found; otherwise 0.
 */
func (shaderSystem *ShaderSystem) GetShaderByID(shaderID uint32) (*metadata.Shader, error) {
	if shaderID >= uint32(shaderSystem.Config.MaxShaderCount) || shaderSystem.Shaders[shaderID].ID == metadata.InvalidID {
		return nil, fmt.Errorf("shader with ID `%d` not found", shaderID)
	}
	return shaderSystem.Shaders[shaderID], nil
}

/**
 * @brief Returns a pointer to a shader with the given name.
 *
 * @param shaderName The name to search for. Case sensitive.
 * @return A pointer to a shader, if found; otherwise 0.
 */
func (shaderSystem *ShaderSystem) GetShader(shaderName string) (*metadata.Shader, error) {
	shader_id := shaderSystem.getShaderID(shaderName)
	if shader_id != metadata.InvalidID {
		return shaderSystem.GetShaderByID(shader_id)
	}
	return nil, fmt.Errorf("shader with name `%s` not found", shaderName)
}

/**
 * @brief Uses the shader with the given name.
 *
 * @param shaderName The name of the shader to use. Case sensitive.
 * @return True on success; otherwise false.
 */
func (shaderSystem *ShaderSystem) UseShader(shaderName string) error {
	next_shader_id := shaderSystem.getShaderID(shaderName)
	if next_shader_id == metadata.InvalidID {
		return fmt.Errorf("next shader ID is invalid")
	}
	return shaderSystem.useByID(next_shader_id)
}

/**
 * @brief Uses the shader with the given identifier.
 *
 * @param shaderID The identifier of the shader to be used.
 * @return True on success; otherwise false.
 */
func (shaderSystem *ShaderSystem) UseShaderByID(shaderID uint32) bool {
	return false
}

/**
 * @brief Returns the uniform index for a uniform with the given name, if found.
 *
 * @param s A pointer to the shader to obtain the index from.
 * @param uniformName The name of the uniform to search for.
 * @return The uniform index, if found; otherwise INVALID_ID_U16.
 */
func (shaderSystem *ShaderSystem) GetUniformIndex(shader *metadata.Shader, uniformName string) uint16 {
	if shader.ID == metadata.InvalidID {
		core.LogError("func GetUniformIndex called with invalid shader.")
		return metadata.InvalidIDUint16
	}
	index := shader.UniformLookup[uniformName]
	if index == metadata.InvalidIDUint16 {
		core.LogError("Shader '%s' does not have a registered uniform named '%s'", shader.Name, uniformName)
		return metadata.InvalidIDUint16
	}
	return shader.Uniforms[index].Index
}

/**
 * @brief Sets the value of a uniform with the given name to the supplied value.
 * NOTE: Operates against the currently-used shader.
 *
 * @param uniformName The name of the uniform to be set.
 * @param value The value to be set.
 * @return True on success; otherwise false.
 */
func (shaderSystem *ShaderSystem) SetUniform(uniformName string, value interface{}) error {
	if shaderSystem.CurrentShaderID == metadata.InvalidID {
		err := fmt.Errorf("func SetUniform called without a shader in use.")
		return err
	}
	shader := shaderSystem.Shaders[shaderSystem.CurrentShaderID]
	index := shaderSystem.GetUniformIndex(shader, uniformName)
	return shaderSystem.SetUniformByIndex(index, value)
}

/**
 * @brief Sets the texture of a sampler with the given name to the supplied texture.
 * NOTE: Operates against the currently-used shader.
 *
 * @param uniformName The name of the uniform to be set.
 * @param t A pointer to the texture to be set.
 * @return True on success; otherwise false.
 */
func (shaderSystem *ShaderSystem) SetTextureSampler(samplerName string, texture *metadata.Texture) error {
	return shaderSystem.SetUniform(samplerName, texture)
}

/**
 * @brief Sets a uniform value by index.
 * NOTE: Operates against the currently-used shader.
 *
 * @param index The index of the uniform.
 * @param value The value of the uniform.
 * @return True on success; otherwise false.
 */
func (shaderSystem *ShaderSystem) SetUniformByIndex(index uint16, value interface{}) error {
	shader := shaderSystem.Shaders[shaderSystem.CurrentShaderID]
	uniform := shader.Uniforms[index]
	if shader.BoundScope != uniform.Scope {
		if uniform.Scope == metadata.ShaderScopeGlobal {
			shaderSystem.renderer.ShaderBindGlobals(shader)
		} else if uniform.Scope == metadata.ShaderScopeInstance {
			shaderSystem.renderer.ShaderBindInstance(shader, shader.BoundInstanceID)
		} else {
			// NOTE: Nothing to do here for locals, just set the uniform.
		}
		shader.BoundScope = uniform.Scope
	}
	return shaderSystem.renderer.ShaderSetUniform(shader, uniform, value)
}

func (shaderSystem *ShaderSystem) SetSampler(samplerName string, texture *metadata.Texture) error {
	return shaderSystem.SetUniform(samplerName, texture)
}

/**
 * @brief Sets a sampler value by index.
 * NOTE: Operates against the currently-used shader.
 *
 * @param index The index of the uniform.
 * @param value A pointer to the texture to be set.
 * @return True on success; otherwise false.
 */
func (shaderSystem *ShaderSystem) SetSamplerByIndex(index uint16, texture *metadata.Texture) error {
	return shaderSystem.SetUniformByIndex(index, texture)
}

/**
 * @brief Applies global-scoped uniforms.
 * NOTE: Operates against the currently-used shader.
 *
 * @return True on success; otherwise false.
 */
func (shaderSystem *ShaderSystem) ApplyGlobal() error {
	return shaderSystem.renderer.ShaderApplyGlobals(shaderSystem.Shaders[shaderSystem.CurrentShaderID])
}

/**
 * @brief Applies instance-scoped uniforms.
 * NOTE: Operates against the currently-used shader.
 * @param needsUpdate Indicates if the shader needs uniform updates or just needs to be bound.
 *
 * @param needsUpdate Indicates if shader internals need to be updated, or just to be bound.
 * @return True on success; otherwise false.
 */
func (shaderSystem *ShaderSystem) ApplyInstance(needsUpdate bool) error {
	return shaderSystem.renderer.ShaderApplyInstance(shaderSystem.Shaders[shaderSystem.CurrentShaderID], needsUpdate)
}

/**
 * @brief Binds the instance with the given id for use. Must be done before setting
 * instance-scoped uniforms.
 * NOTE: Operates against the currently-used shader.
 *
 * @param instanceID The identifier of the instance to bind.
 * @return True on success; otherwise false.
 */
func (shaderSystem *ShaderSystem) BindInstance(instanceID uint32) bool {
	shader := shaderSystem.Shaders[shaderSystem.CurrentShaderID]
	shader.BoundInstanceID = instanceID
	return shaderSystem.renderer.ShaderBindInstance(shader, instanceID)
}

func (s *ShaderSystem) addAttribute(shader *metadata.Shader, config *metadata.ShaderAttributeConfig) bool {
	size := uint32(0)
	switch config.ShaderAttributeType {
	case metadata.ShaderAttribTypeInt8, metadata.ShaderAttribTypeUint8:
		size = 1
	case metadata.ShaderAttribTypeInt16, metadata.ShaderAttribTypeUint16:
		size = 2
	case metadata.ShaderAttribTypeFloat32, metadata.ShaderAttribTypeInt32, metadata.ShaderAttribTypeUint32:
		size = 4
	case metadata.ShaderAttribTypeFloat32_2:
		size = 8
	case metadata.ShaderAttribTypeFloat32_3:
		size = 12
	case metadata.ShaderAttribTypeFloat32_4:
		size = 16
	default:
		core.LogError("unrecognized type %d, defaulting to size of 4. This probably is not what is desired", size)
		size = 4
	}

	shader.AttributeStride += uint16(size)

	// Create/push the attribute.
	attrib := metadata.ShaderAttribute{
		Name:                       config.Name,
		Size:                       size,
		ShaderUniformAttributeType: config.ShaderAttributeType,
	}
	shader.Attributes = append(shader.Attributes, attrib)

	return true
}

func (shaderSystem *ShaderSystem) addSampler(shader *metadata.Shader, config *metadata.ShaderUniformConfig) error {
	// Samples can't be used for push constants.
	if config.Scope == metadata.ShaderScopeLocal {
		err := fmt.Errorf("add_sampler cannot add a sampler at local scope.")
		return err
	}

	// Verify the name is valid and unique.
	if !shaderSystem.uniformNameValid(shader, config.Name) || !shaderSystem.shaderUniformAddStateValid(shader) {
		err := fmt.Errorf("failed to verify uniform name")
		return err
	}

	// If global, push into the global list.
	location := uint32(0)
	if config.Scope == metadata.ShaderScopeGlobal {
		global_texture_count := len(shader.GlobalTextureMaps)
		if global_texture_count+1 > int(shaderSystem.Config.MaxGlobalTextures) {
			err := fmt.Errorf("Shader global texture count `%d` exceeds max of `%d`", global_texture_count, shaderSystem.Config.MaxGlobalTextures)
			return err
		}
		location = uint32(global_texture_count)

		// NOTE: creating a default texture map to be used here. Can always be updated later.
		default_map := &metadata.TextureMap{
			FilterMagnify: metadata.TextureFilterModeLinear,
			FilterMinify:  metadata.TextureFilterModeLinear,
			RepeatU:       metadata.TextureRepeatRepeat,
			RepeatV:       metadata.TextureRepeatRepeat,
			RepeatW:       metadata.TextureRepeatRepeat,
			Use:           metadata.TextureUseUnknown,
		}
		if err := shaderSystem.renderer.TextureMapAcquireResources(default_map); err != nil {
			core.LogError("Failed to acquire resources for global texture map during shader creation.")
			return err
		}

		// Allocate a pointer assign the texture, and push into global texture maps.
		// NOTE: This allocation is only done for global texture maps.
		textureMap := default_map
		textureMap.Texture = shaderSystem.textureSystem.GetDefaultTexture()

		shader.GlobalTextureMaps = append(shader.GlobalTextureMaps, textureMap)
	} else {
		// Otherwise, it's instance-level, so keep count of how many need to be added during the resource acquisition.
		if shader.InstanceTextureCount+1 > shaderSystem.Config.MaxInstanceTextures {
			err := fmt.Errorf("Shader instance texture count `%d` exceeds max of `%d`", shader.InstanceTextureCount, shaderSystem.Config.MaxInstanceTextures)
			return err
		}
		location = uint32(shader.InstanceTextureCount)
		shader.InstanceTextureCount++
	}

	// Treat it like a uniform. NOTE: In the case of samplers, out_location is used to determine the
	// hashtable entry's 'location' field value directly, and is then set to the index of the uniform array.
	// This allows location lookups for samplers as if they were uniforms as well (since technically they are).
	// TODO: might need to store this elsewhere
	if !shaderSystem.uniformAdd(shader, config.Name, 0, config.ShaderUniformType, config.Scope, location, true) {
		err := fmt.Errorf("unable to add sampler uniform")
		return err
	}

	return nil
}

func (shaderSystem *ShaderSystem) addUniform(shader *metadata.Shader, config *metadata.ShaderUniformConfig) bool {
	if !shaderSystem.shaderUniformAddStateValid(shader) || !shaderSystem.uniformNameValid(shader, config.Name) {
		return false
	}
	return shaderSystem.uniformAdd(shader, config.Name, uint32(config.Size), config.ShaderUniformType, config.Scope, 0, false)
}

func (shaderSystem *ShaderSystem) getShaderID(shader_name string) uint32 {
	id, ok := shaderSystem.Lookup[shader_name]
	if !ok {
		core.LogError("There is no shader registered named '%s'.", shader_name)
		return metadata.InvalidID
	}
	return id
}

func (s *ShaderSystem) newShaderID() uint32 {
	for i := uint32(0); i < uint32(s.Config.MaxShaderCount); i++ {
		if s.Shaders[i].ID == metadata.InvalidID {
			return i
		}
	}
	return metadata.InvalidID
}

func (shaderSystem *ShaderSystem) uniformAdd(shader *metadata.Shader, uniform_name string, size uint32, shader_uniform_type metadata.ShaderUniformType, scope metadata.ShaderScope, set_location uint32, is_sampler bool) bool {
	uniform_count := len(shader.Uniforms)
	if uniform_count+1 > int(shaderSystem.Config.MaxUniformCount) {
		core.LogError("A shader can only accept a combined maximum of %d uniforms and samplers at global, instance and local scopes.", shaderSystem.Config.MaxUniformCount)
		return false
	}
	entry := metadata.ShaderUniform{
		Index:             uint16(uniform_count), // Index is saved to the hashtable for lookups.
		Scope:             scope,
		ShaderUniformType: shader_uniform_type,
	}

	is_global := (scope == metadata.ShaderScopeGlobal)
	if is_sampler {
		// Just use the passed in location
		entry.Location = uint16(set_location)
	} else {
		entry.Location = entry.Index
	}

	if scope != metadata.ShaderScopeLocal {
		entry.SetIndex = uint8(scope)
		entry.Offset = 0
		if !is_sampler {
			if is_global {
				entry.Offset = shader.GlobalUboSize
			} else {
				entry.Offset = shader.UboSize
			}
		}
		entry.Size = 0
		if !is_sampler {
			entry.Size = uint16(size)
		}
	} else {
		// Push a new aligned range (align to 4, as required by Vulkan spec)
		entry.SetIndex = metadata.InvalidIDUint8
		r := metadata.GetAlignedRange(shader.PushConstantSize, uint64(size), 4)
		// utilize the aligned offset/range
		entry.Offset = r.Offset
		entry.Size = uint16(r.Size)

		// Track in configuration for use in initialization.
		if len(shader.PushConstantRanges) == 0 {
			shader.PushConstantRanges = make([]*metadata.MemoryRange, int(math.Max(1, float64(shader.PushConstantRangeCount))))
		}
		shader.PushConstantRanges[shader.PushConstantRangeCount] = r
		shader.PushConstantRangeCount++

		// Increase the push constant's size by the total value.
		shader.PushConstantSize += r.Size
	}

	shader.UniformLookup[uniform_name] = entry.Index
	shader.Uniforms = append(shader.Uniforms, entry)

	if !is_sampler {
		if entry.Scope == metadata.ShaderScopeGlobal {
			shader.GlobalUboSize += uint64(entry.Size)
		} else if entry.Scope == metadata.ShaderScopeInstance {
			shader.UboSize += uint64(entry.Size)
		}
	}

	return true
}

func (shaderSystem *ShaderSystem) uniformNameValid(shader *metadata.Shader, uniform_name string) bool {
	if uniform_name == "" {
		core.LogError("Uniform name must exist.")
		return false
	}
	if location, ok := shader.UniformLookup[uniform_name]; !ok && location != metadata.InvalidIDUint16 {
		core.LogError("A uniform by the name '%s' already exists on shader '%s'.", uniform_name, shader.Name)
		return false
	}
	return true
}

func (shaderSystem *ShaderSystem) shaderUniformAddStateValid(shader *metadata.Shader) bool {
	if shader.State != metadata.SHADER_STATE_UNINITIALIZED {
		core.LogError("Uniforms may only be added to shaders before initialization.")
		return false
	}
	return true
}

func (shaderSystem *ShaderSystem) useByID(shaderID uint32) error {
	// Only perform the use if the shader id is different.
	if shaderSystem.CurrentShaderID != shaderID {
		nextShader, err := shaderSystem.GetShaderByID(shaderID)
		if err != nil {
			return err
		}
		shaderSystem.CurrentShaderID = shaderID
		if err := shaderSystem.renderer.ShaderUse(nextShader); err != nil {
			core.LogError("Failed to use shader '%s'.", nextShader.Name)
			return err
		}
		if err := shaderSystem.renderer.ShaderBindGlobals(nextShader); err != nil {
			core.LogError("Failed to bind globals for shader '%s'.", nextShader.Name)
			return err
		}
	}
	return nil
}

func (shaderSystem *ShaderSystem) shaderDestroy(shader *metadata.Shader) error {
	shaderSystem.renderer.ShaderDestroy(shader)
	// Set it to be unusable right away.
	shader.State = metadata.SHADER_STATE_NOT_CREATED
	for i := 0; i < len(shader.GlobalTextureMaps); i++ {
		shader.GlobalTextureMaps[i] = nil
	}
	shader.GlobalTextureMaps = make([]*metadata.TextureMap, 1)
	return nil
}
