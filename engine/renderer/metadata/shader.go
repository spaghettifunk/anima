package metadata

import (
	"github.com/spaghettifunk/anima/engine/resources"
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
	/** @brief The Offset in bytes from the beginning of the uniform set (global/instance/local) */
	Offset uint64
	/**
	 * @brief The Location to be used as a lookup. Typically the same as the index except for samplers,
	 * which is used to lookup texture index within the internal array at the given scope (global/instance).
	 */
	Location uint16
	/** @brief Index into the internal uniform array. */
	Index uint16
	/** @brief The Size of the uniform, or 0 for samplers. */
	Size uint16
	/** @brief The index of the descriptor set the uniform belongs to (0=global, 1=instance, INVALID_ID=local). */
	SetIndex uint8
	/** @brief The Scope of the uniform. */
	Scope resources.ShaderScope
	/** @brief The type of uniform. */
	ShaderUniformType resources.ShaderUniformType
}

/**
 * @brief Represents a single shader vertex attribute.
 */
type ShaderAttribute struct {
	/** @brief The attribute Name. */
	Name string
	/** @brief The attribute type. */
	ShaderUniformAttributeType resources.ShaderAttributeType
	/** @brief The attribute Size in bytes. */
	Size uint32
}

/**
 * @brief Represents a shader on the frontend.
 */
type Shader struct {
	/** @brief The shader identifier */
	ID uint32

	Name string

	/**
	 * @brief The amount of bytes that are required for UBO alignment.
	 *
	 * This is used along with the UBO size to determine the ultimate
	 * stride, which is how much the UBOs are spaced out in the buffer.
	 * For example, a required alignment of 256 means that the stride
	 * must be a multiple of 256 (true for some nVidia cards).
	 */
	RequiredUboAlignment uint64

	/** @brief The actual size of the global uniform buffer object. */
	GlobalUboSize uint64
	/** @brief The stride of the global uniform buffer object. */
	GlobalUboStride uint64
	/**
	 * @brief The offset in bytes for the global UBO from the beginning
	 * of the uniform buffer.
	 */
	GlobalUboOffset uint64

	/** @brief The actual size of the instance uniform buffer object. */
	UboSize uint64

	/** @brief The stride of the instance uniform buffer object. */
	UboStride uint64

	/** @brief The total size of all push constant ranges combined. */
	PushConstantSize uint64
	/** @brief The push constant stride, aligned to 4 bytes as required by Vulkan. */
	PushConstantStride uint64

	/** @brief An array of global texture map pointers. Darray */
	GlobalTextureMaps []*resources.TextureMap

	/** @brief The number of instance textures. */
	InstanceTextureCount uint8

	BoundScope resources.ShaderScope

	/** @brief The identifier of the currently bound instance. */
	BoundInstanceID uint32
	/** @brief The currently bound instance's ubo offset. */
	BoundUboOffset uint32

	/** @brief A hashtable to store uniform index/locations by name. */
	UniformLookup map[string]uint16

	/** @brief An array of Uniforms in this shader. Darray. */
	Uniforms []ShaderUniform

	/** @brief An array of Attributes. Darray. */
	Attributes []ShaderAttribute

	/** @brief The internal State of the shader. */
	State ShaderState

	/** @brief The number of push constant ranges. */
	PushConstantRangeCount uint8
	/** @brief An array of push constant ranges. */
	PushConstantRanges [32]*MemoryRange
	/** @brief The size of all attributes combined, a.k.a. the size of a vertex. */
	AttributeStride uint16

	/** @brief aUsed to ensure the shader's globals are only updated once per frame. */
	RenderFrameNumber uint64

	/** @brief An opaque pointer to hold renderer API specific data. Renderer is responsible for creation and destruction of this.  */
	InternalData interface{}
}
