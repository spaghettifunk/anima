package metadata

import (
	"github.com/spaghettifunk/anima/engine/resources"
)

/** @brief Configuration for the shader system. */
type ShaderSystemConfig struct {
	/** @brief The maximum number of shaders held in the system. NOTE: Should be at least 512. */
	max_shader_count uint16
	/** @brief The maximum number of uniforms allowed in a single shader. */
	max_uniform_count uint8
	/** @brief The maximum number of global-scope textures allowed in a single shader. */
	max_global_textures uint8
	/** @brief The maximum number of instance-scope textures allowed in a single shader. */
	max_instance_textures uint8
}

/**
 * @brief Represents the current state of a given shader.
 */
type ShaderState int

const (
	/** @brief The shader has not yet gone through the creation process, and is unusable.*/
	SHADER_STATE_NOT_CREATED ShaderState = iota
	/** @brief The shader has gone through the creation process, but not initialization. It is unusable.*/
	SHADER_STATE_UNINITIALIZED
	/** @brief The shader is created and initialized, and is ready for use.*/
	SHADER_STATE_INITIALIZED
)

/**
 * @brief Represents a single entry in the internal uniform array.
 */
type ShaderUniform struct {
	/** @brief The offset in bytes from the beginning of the uniform set (global/instance/local) */
	offset uint64
	/**
	 * @brief The location to be used as a lookup. Typically the same as the index except for samplers,
	 * which is used to lookup texture index within the internal array at the given scope (global/instance).
	 */
	location uint16
	/** @brief Index into the internal uniform array. */
	index uint16
	/** @brief The size of the uniform, or 0 for samplers. */
	size uint16
	/** @brief The index of the descriptor set the uniform belongs to (0=global, 1=instance, INVALID_ID=local). */
	set_index uint8
	/** @brief The scope of the uniform. */
	scope resources.ShaderScope
	/** @brief The type of uniform. */
	shaderUniformType resources.ShaderUniformType
}

/**
 * @brief Represents a single shader vertex attribute.
 */
type ShaderAttribute struct {
	/** @brief The attribute name. */
	name string
	/** @brief The attribute type. */
	shaderUniformAttributeType resources.ShaderAttributeType
	/** @brief The attribute size in bytes. */
	size uint32
}

/**
 * @brief Represents a shader on the frontend.
 */
type Shader struct {
	/** @brief The shader identifier */
	ID uint32

	name string

	/**
	 * @brief The amount of bytes that are required for UBO alignment.
	 *
	 * This is used along with the UBO size to determine the ultimate
	 * stride, which is how much the UBOs are spaced out in the buffer.
	 * For example, a required alignment of 256 means that the stride
	 * must be a multiple of 256 (true for some nVidia cards).
	 */
	required_ubo_alignment uint64

	/** @brief The actual size of the global uniform buffer object. */
	global_ubo_size uint64
	/** @brief The stride of the global uniform buffer object. */
	global_ubo_stride uint64
	/**
	 * @brief The offset in bytes for the global UBO from the beginning
	 * of the uniform buffer.
	 */
	global_ubo_offset uint64

	/** @brief The actual size of the instance uniform buffer object. */
	ubo_size uint64

	/** @brief The stride of the instance uniform buffer object. */
	ubo_stride uint64

	/** @brief The total size of all push constant ranges combined. */
	push_constant_size uint64
	/** @brief The push constant stride, aligned to 4 bytes as required by Vulkan. */
	push_constant_stride uint64

	/** @brief An array of global texture map pointers. Darray */
	global_texture_maps []*resources.TextureMap

	/** @brief The number of instance textures. */
	instance_texture_count uint8

	bound_scope resources.ShaderScope

	/** @brief The identifier of the currently bound instance. */
	bound_instance_id uint32
	/** @brief The currently bound instance's ubo offset. */
	bound_ubo_offset uint32

	/** @brief The block of memory used by the uniform hashtable. */
	hashtable_block interface{}
	/** @brief A hashtable to store uniform index/locations by name. */
	uniform_lookup map[string]interface{}

	/** @brief An array of uniforms in this shader. Darray. */
	uniforms []ShaderUniform

	/** @brief An array of attributes. Darray. */
	attributes []ShaderAttribute

	/** @brief The internal state of the shader. */
	state ShaderState

	/** @brief The number of push constant ranges. */
	push_constant_range_count uint8
	/** @brief An array of push constant ranges. */
	push_constant_ranges [32]MemoryRange
	/** @brief The size of all attributes combined, a.k.a. the size of a vertex. */
	attribute_stride uint16

	/** @brief aUsed to ensure the shader's globals are only updated once per frame. */
	render_frame_number uint64

	/** @brief An opaque pointer to hold renderer API specific data. Renderer is responsible for creation and destruction of this.  */
	internal_data interface{}
}
