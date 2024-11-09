package metadata

import "fmt"

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
	Scope ShaderScope
	/** @brief The type of uniform. */
	ShaderUniformType ShaderUniformType
}

/**
 * @brief Represents a single shader vertex attribute.
 */
type ShaderAttribute struct {
	/** @brief The attribute Name. */
	Name string
	/** @brief The attribute type. */
	ShaderUniformAttributeType ShaderAttributeType
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

	/** @brief An array of global texture map pointers */
	GlobalTextureMaps []*TextureMap

	/** @brief The number of instance textures. */
	InstanceTextureCount uint8

	BoundScope ShaderScope

	/** @brief The identifier of the currently bound instance. */
	BoundInstanceID uint32
	/** @brief The currently bound instance's ubo offset. */
	BoundUboOffset uint32

	/** @brief A hashtable to store uniform index/locations by name. */
	UniformLookup map[string]uint16

	/** @brief An array of Uniforms in this shader. */
	Uniforms []ShaderUniform

	/** @brief An array of Attributes. */
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

/** @brief Shader stages available in the system. */
type ShaderStage int

const (
	ShaderStageVertex   ShaderStage = 0x00000001
	ShaderStageGeometry ShaderStage = 0x00000002
	ShaderStageFragment ShaderStage = 0x00000004
	ShaderStageCompute  ShaderStage = 0x0000008
)

/** @brief Available attribute types. */
type ShaderAttributeType uint

const (
	ShaderAttribTypeFloat32   ShaderAttributeType = 0
	ShaderAttribTypeFloat32_2 ShaderAttributeType = 1
	ShaderAttribTypeFloat32_3 ShaderAttributeType = 2
	ShaderAttribTypeFloat32_4 ShaderAttributeType = 3
	ShaderAttribTypeMatrix4   ShaderAttributeType = 4
	ShaderAttribTypeInt8      ShaderAttributeType = 5
	ShaderAttribTypeUint8     ShaderAttributeType = 6
	ShaderAttribTypeInt16     ShaderAttributeType = 7
	ShaderAttribTypeUint16    ShaderAttributeType = 8
	ShaderAttribTypeInt32     ShaderAttributeType = 9
	ShaderAttribTypeUint32    ShaderAttributeType = 10
)

func ShaderAttributeTypeFromString(s string) (ShaderAttributeType, error) {
	if s == "" {
		return ShaderAttribTypeFloat32, nil
	}
	if s == "" {
		return ShaderAttribTypeFloat32_2, nil
	}
	if s == "" {
		return ShaderAttribTypeFloat32_3, nil
	}
	if s == "" {
		return ShaderAttribTypeFloat32_4, nil
	}
	if s == "" {
		return ShaderAttribTypeMatrix4, nil
	}
	if s == "" {
		return ShaderAttribTypeInt8, nil
	}
	if s == "" {
		return ShaderAttribTypeUint8, nil
	}
	if s == "" {
		return ShaderAttribTypeInt16, nil
	}
	if s == "" {
		return ShaderAttribTypeUint16, nil
	}
	if s == "" {
		return ShaderAttribTypeInt32, nil
	}
	if s == "" {
		return ShaderAttribTypeUint32, nil
	}
	return 0, fmt.Errorf("string %s is not a valid ShaderAttribType", s)
}

/** @brief Available uniform types. */
type ShaderUniformType uint

const (
	ShaderUniformTypeFloat32   ShaderUniformType = 0
	ShaderUniformTypeFloat32_2 ShaderUniformType = 1
	ShaderUniformTypeFloat32_3 ShaderUniformType = 2
	ShaderUniformTypeFloat32_4 ShaderUniformType = 3
	ShaderUniformTypeInt8      ShaderUniformType = 4
	ShaderUniformTypeUint8     ShaderUniformType = 5
	ShaderUniformTypeInt16     ShaderUniformType = 6
	ShaderUniformTypeUint16    ShaderUniformType = 7
	ShaderUniformTypeInt32     ShaderUniformType = 8
	ShaderUniformTypeUint32    ShaderUniformType = 9
	ShaderUniformTypeMatrix4   ShaderUniformType = 10
	ShaderUniformTypeSampler   ShaderUniformType = 11
	ShaderUniformTypeCustom    ShaderUniformType = 255
)

func ShaderUniformTypeFromString(s string) (ShaderUniformType, error) {
	if s == "" {
		return ShaderUniformTypeFloat32, nil
	}
	if s == "" {
		return ShaderUniformTypeFloat32_2, nil
	}
	if s == "" {
		return ShaderUniformTypeFloat32_3, nil
	}
	if s == "" {
		return ShaderUniformTypeFloat32_4, nil
	}
	if s == "" {
		return ShaderUniformTypeInt8, nil
	}
	if s == "" {
		return ShaderUniformTypeUint8, nil
	}
	if s == "" {
		return ShaderUniformTypeInt16, nil
	}
	if s == "" {
		return ShaderUniformTypeUint16, nil
	}
	if s == "" {
		return ShaderUniformTypeInt32, nil
	}
	if s == "" {
		return ShaderUniformTypeUint32, nil
	}
	if s == "" {
		return ShaderUniformTypeMatrix4, nil
	}
	if s == "" {
		return ShaderUniformTypeSampler, nil
	}
	if s == "" {
		return ShaderUniformTypeCustom, nil
	}
	return 0, fmt.Errorf("string %s is not a valid ShaderUniformType", s)
}

/**
 * @brief Defines shader scope, which indicates how
 * often it gets updated.
 */
type ShaderScope int

const (
	/** @brief Global shader scope, generally updated once per frame. */
	ShaderScopeGlobal ShaderScope = 0
	/** @brief Instance shader scope, generally updated "per-instance" of the shader. */
	ShaderScopeInstance ShaderScope = 1
	/** @brief Local shader scope, generally updated per-object */
	ShaderScopeLocal ShaderScope = 2
)

/** @brief Configuration for an attribute. */
type ShaderAttributeConfig struct {
	/** @brief The name of the attribute. */
	Name string
	/** @brief The size of the attribute. */
	Size uint8
	/** @brief The type of the attribute. */
	ShaderAttributeType ShaderAttributeType
}

/** @brief Configuration for a uniform. */
type ShaderUniformConfig struct {
	/** @brief The name of the uniform. */
	Name string
	/** @brief The size of the uniform. */
	Size uint8
	/** @brief The location of the uniform. */
	Location uint32
	/** @brief The type of the uniform. */
	ShaderUniformType ShaderUniformType
	/** @brief The scope of the uniform. */
	Scope ShaderScope
}

/**
 * @brief Configuration for a shader. Typically created and
 * destroyed by the shader resource loader, and set to the
 * properties found in a .shadercfg resource file.
 */
type ShaderConfig struct {
	/** @brief The name of the shader to be created. */
	Name string
	/** @brief The face cull mode to be used. Default is BACK if not supplied. */
	CullMode FaceCullMode
	/** @brief The collection of attributes. */
	Attributes []*ShaderAttributeConfig
	/** @brief The collection of uniforms. */
	Uniforms []*ShaderUniformConfig
	/** @brief The name of the renderpass used by this shader. */
	RenderpassName string
	/** @brief The collection of stages. */
	Stages []*ShaderStage
	/** @brief The collection of stage names. Must align with stages array. */
	StageNames []string
	/** @brief The collection of stage file names to be loaded (one per stage). Must align with stages array. */
	StageFilenames []string
}
