package systems

import (
	"github.com/spaghettifunk/anima/engine/renderer/metadata"
	"github.com/spaghettifunk/anima/engine/resources"
)

type ShaderSystem struct{}

func shader_system_initialize(memory_requirement []uint64, memory interface{}, config metadata.ShaderSystemConfig) (*ShaderSystem, error) {
	return nil, nil
}

/**
 * @brief Shuts down the shader system.
 *
 * @param state A pointer to the system state.
 */
func (s *ShaderSystem) shader_system_shutdown() {}

/**
 * @brief Creates a new shader with the given config.
 *
 * @param config The configuration to be used when creating the shader.
 * @return True on success; otherwise false.
 */
func (s *ShaderSystem) shader_system_create(config *resources.ShaderConfig) error {
	return nil
}

/**
 * @brief Gets the identifier of a shader by name.
 *
 * @param shader_name The name of the shader.
 * @return The shader id, if found; otherwise INVALID_ID.
 */
func (s *ShaderSystem) shader_system_get_id(shader_name string) uint32 {
	return 0
}

/**
 * @brief Returns a pointer to a shader with the given identifier.
 *
 * @param shader_id The shader identifier.
 * @return A pointer to a shader, if found; otherwise 0.
 */
func (s *ShaderSystem) shader_system_get_by_id(shader_id uint32) (*metadata.Shader, error) {
	return nil, nil
}

/**
 * @brief Returns a pointer to a shader with the given name.
 *
 * @param shader_name The name to search for. Case sensitive.
 * @return A pointer to a shader, if found; otherwise 0.
 */
func (s *ShaderSystem) shader_system_get(shader_name string) (*metadata.Shader, error) {
	return nil, nil
}

/**
 * @brief Uses the shader with the given name.
 *
 * @param shader_name The name of the shader to use. Case sensitive.
 * @return True on success; otherwise false.
 */
func (s *ShaderSystem) shader_system_use(shader_name string) bool {
	return false
}

/**
 * @brief Uses the shader with the given identifier.
 *
 * @param shader_id The identifier of the shader to be used.
 * @return True on success; otherwise false.
 */
func (s *ShaderSystem) shader_system_use_by_id(shader_id uint32) bool {
	return false
}

/**
 * @brief Returns the uniform index for a uniform with the given name, if found.
 *
 * @param s A pointer to the shader to obtain the index from.
 * @param uniform_name The name of the uniform to search for.
 * @return The uniform index, if found; otherwise INVALID_ID_U16.
 */
func (s *ShaderSystem) shader_system_uniform_index(shader *metadata.Shader, uniform_name string) uint32 {
	return 0
}

/**
 * @brief Sets the value of a uniform with the given name to the supplied value.
 * NOTE: Operates against the currently-used shader.
 *
 * @param uniform_name The name of the uniform to be set.
 * @param value The value to be set.
 * @return True on success; otherwise false.
 */
func (s *ShaderSystem) shader_system_uniform_set(uniform_name string, value interface{}) bool {
	return false
}

/**
 * @brief Sets the texture of a sampler with the given name to the supplied texture.
 * NOTE: Operates against the currently-used shader.
 *
 * @param uniform_name The name of the uniform to be set.
 * @param t A pointer to the texture to be set.
 * @return True on success; otherwise false.
 */
func (s *ShaderSystem) shader_system_sampler_set(sampler_name string, texture *resources.Texture) bool {
	return false
}

/**
 * @brief Sets a uniform value by index.
 * NOTE: Operates against the currently-used shader.
 *
 * @param index The index of the uniform.
 * @param value The value of the uniform.
 * @return True on success; otherwise false.
 */
func (s *ShaderSystem) shader_system_uniform_set_by_index(index uint16, value interface{}) bool {
	return false
}

/**
 * @brief Sets a sampler value by index.
 * NOTE: Operates against the currently-used shader.
 *
 * @param index The index of the uniform.
 * @param value A pointer to the texture to be set.
 * @return True on success; otherwise false.
 */
func (s *ShaderSystem) shader_system_sampler_set_by_index(index uint16, texture *resources.Texture) bool {
	return false
}

/**
 * @brief Applies global-scoped uniforms.
 * NOTE: Operates against the currently-used shader.
 *
 * @return True on success; otherwise false.
 */
func (s *ShaderSystem) shader_system_apply_global() bool {
	return false
}

/**
 * @brief Applies instance-scoped uniforms.
 * NOTE: Operates against the currently-used shader.
 * @param needs_update Indicates if the shader needs uniform updates or just needs to be bound.
 *
 * @param needs_update Indicates if shader internals need to be updated, or just to be bound.
 * @return True on success; otherwise false.
 */
func (s *ShaderSystem) shader_system_apply_instance(needs_update bool) bool {
	return false
}

/**
 * @brief Binds the instance with the given id for use. Must be done before setting
 * instance-scoped uniforms.
 * NOTE: Operates against the currently-used shader.
 *
 * @param instance_id The identifier of the instance to bind.
 * @return True on success; otherwise false.
 */
func (s *ShaderSystem) shader_system_bind_instance(instance_id uint32) bool {
	return false
}
