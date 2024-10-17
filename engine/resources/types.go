package resources

import "github.com/spaghettifunk/anima/engine/math"

type ResourceType int

/** @brief Pre-defined resource types. */
const (
	/** @brief Text resource type. */
	ResourceTypeText ResourceType = iota
	/** @brief Binary resource type. */
	ResourceTypeBinary
	/** @brief Image resource type. */
	ResourceTypeImage
	/** @brief Material resource type. */
	ResourceTypeMaterial
	/** @brief Shader resource type (or more accurately shader config). */
	ResourceTypeShader
	/** @brief Mesh resource type (collection of geometry configs). */
	ResourceTypeMesh
	/** @brief Bitmap font resource type. */
	ResourceTypeBitmapFont
	/** @brief System font resource type. */
	ResourceTypeSystemFont
	/** @brief Custom resource type. Used by loaders outside the core engine. */
	ResourceTypeCustom
)

/** @brief A magic number indicating the file as an anima binary file. */
const ResourceMagic int = 0xdaaaadd1

/**
 * @brief The header data for binary resource types.
 */
type ResourceHeader struct {
	/** @brief A magic number indicating the file as a kohi binary file. */
	MagicNumber uint32
	/** @brief The resource type. Maps to the enum resource_type. */
	ResourceType ResourceType
	/** @brief The format version this resource uses. */
	Version uint8
	/** @brief Reserved for future header data.. */
	Reserved uint16
}

/**
 * @brief A generic structure for a resource. All resource loaders
 * load data into these.
 */
type Resource struct {
	/** @brief The identifier of the loader which handles this resource. */
	LoaderID uint32
	/** @brief The name of the resource. */
	Name string
	/** @brief The full file path of the resource. */
	FullPath string
	/** @brief The size of the resource data in bytes. */
	DataSize uint64
	/** @brief The resource data. */
	Data interface{}
}

/**
 * @brief A structure to hold image resource data.
 */
type ImageResourceData struct {
	/** @brief The number of channels. */
	ChannelCount uint8
	/** @brief The width of the image. */
	Width uint32
	/** @brief The height of the image. */
	Height uint32
	/** @brief The pixel data of the image. */
	Pixels []uint8
}

/** @brief Parameters used when loading an image. */
type ImageResourceParams struct {
	/** @brief Indicates if the image should be flipped on the y-axis when loaded. */
	FlipY bool
}

/** @brief Determines face culling mode during rendering. */
type FaceCullMode int

const (
	/** @brief No faces are culled. */
	FaceCullModeNone FaceCullMode = 0x0
	/** @brief Only front faces are culled. */
	FaceCullModeFront FaceCullMode = 0x1
	/** @brief Only back faces are culled. */
	FaceCullModeBack FaceCullMode = 0x2
	/** @brief Both front and back faces are culled. */
	FaceCullModeFrontAndBack FaceCullMode = 0x3
)

/**
 * @brief The maximum length of a texture name.
 */
const TextureNameMaxLength int = 512

type TextureFlag int

const (
	/** @brief Indicates if the texture has transparency. */
	TextureFlagHasTransparency TextureFlag = 0x1
	/** @brief Indicates if the texture can be written (rendered) to. */
	TextureFlagIsWriteable TextureFlag = 0x2
	/** @brief Indicates if the texture was created via wrapping vs traditional creation. */
	TextureFlagIsWrapped TextureFlag = 0x4
)

/** @brief Holds bit flags for textures.. */
type TextureFlagBits uint8

/**
 * @brief Represents various types of textures.
 */
type TextureType int

const (
	/** @brief A standard two-dimensional texture. */
	TextureType2d TextureType = iota
	/** @brief A cube texture, used for cubemaps. */
	TextureTypeCube
)

/**
 * @brief Represents a texture.
 */
type Texture struct {
	/** @brief The unique texture identifier. */
	ID uint32
	/** @brief The texture type. */
	TextureType TextureType
	/** @brief The texture Width. */
	Width uint32
	/** @brief The texture Height. */
	Height uint32
	/** @brief The number of channels in the texture. */
	ChannelCount uint8
	/** @brief Holds various Flags for this texture. */
	Flags TextureFlagBits
	/** @brief The texture Generation. Incremented every time the data is reloaded. */
	Generation uint32
	/** @brief The texture Name. */
	Name string
	/** @brief The raw texture data (pixels). */
	InternalData interface{}
}

/** @brief A collection of texture uses */
type TextureUse int

const (
	/** @brief An unknown use. This is default, but should never actually be used. */
	TextureUseUnknown TextureUse = 0x00
	/** @brief The texture is used as a diffuse map. */
	TextureUseMapDiffuse TextureUse = 0x01
	/** @brief The texture is used as a specular map. */
	TextureUseMapSpecular TextureUse = 0x02
	/** @brief The texture is used as a normal map. */
	TextureUseMapNormal TextureUse = 0x03
	/** @brief The texture is used as a cube map. */
	TextureUseMapCubemap TextureUse = 0x04
)

/** @brief Represents supported texture filtering modes. */
type TextureFilter int

const (
	/** @brief Nearest-neighbor filtering. */
	TextureFilterModeNearest TextureFilter = 0x0
	/** @brief Linear (i.e. bilinear) filtering.*/
	TextureFilterModeLinear TextureFilter = 0x1
)

type TextureRepeat int

const (
	TextureRepeatRepeat         TextureRepeat = 0x1
	TextureRepeatMirroredRepeat TextureRepeat = 0x2
	TextureRepeatClampToEdge    TextureRepeat = 0x3
	TextureRepeatClampToBorder  TextureRepeat = 0x4
)

/**
 * @brief A structure which maps a texture, use and
 * other properties.
 */
type TextureMap struct {
	/** @brief A pointer to a Texture. */
	Texture *Texture
	/** @brief The Use of the texture */
	Use TextureUse
	/** @brief Texture filtering mode for minification. */
	FilterMinify TextureFilter
	/** @brief Texture filtering mode for magnification. */
	FilterMagnify TextureFilter
	/** @brief The repeat mode on the U axis (or X, or S) */
	RepeatU TextureRepeat
	/** @brief The repeat mode on the V axis (or Y, or T) */
	RepeatV TextureRepeat
	/** @brief The repeat mode on the W axis (or Z, or U) */
	RepeatW TextureRepeat
	/** @brief A pointer to internal, render API-specific data. Typically the internal sampler. */
	InternalData interface{}
}

/** @brief The maximum length of a material name. */
const MaterialNameMaxLength int = 256

/**
 * @brief Material configuration typically loaded from
 * a file or created in code to load a material from.
 */
type material_config struct {
	/** @brief The name of the material. */
	Name [MaterialNameMaxLength]string
	/** @brief The material type. */
	ShaderName string
	/** @brief Indicates if the material should be automatically released when no references to it remain. */
	AutoRelease bool
	/** @brief The diffuse colour of the material. */
	DiffuseColour math.Vec4
	/** @brief The shininess of the material. */
	Shininess float32
	/** @brief The diffuse map name. */
	DiffuseMapName [TextureNameMaxLength]string
	/** @brief The specular map name. */
	SpecularMapName [TextureNameMaxLength]string
	/** @brief The normal map name. */
	NormalMapName [TextureNameMaxLength]string
}

/**
 * @brief A material, which represents various properties
 * of a surface in the world such as texture, colour,
 * bumpiness, shininess and more.
 */
type Material struct {
	/** @brief The material id. */
	ID uint32
	/** @brief The material generation. Incremented every time the material is changed. */
	Generation uint32
	/** @brief The internal material id. Used by the renderer backend to map to internal resources. */
	InternalID uint32
	/** @brief The material name. */
	Name [MaterialNameMaxLength]string
	/** @brief The diffuse colour. */
	DiffuseColour math.Vec4
	/** @brief The diffuse texture map. */
	DiffuseMap TextureMap
	/** @brief The specular texture map. */
	SpecularMap TextureMap
	/** @brief The normal texture map. */
	NormalMap TextureMap
	/** @brief The material shininess, determines how concentrated the specular lighting is. */
	Shininess float32
	ShaderID  uint32
	/** @brief Synced to the renderer's current frame number when the material has been applied that frame. */
	RenderFrameNumber uint32
}

/** @brief The maximum length of a geometry name. */
const GeometryNameMaxLength int = 256

/**
 * @brief Represents actual geometry in the world.
 * Typically (but not always, depending on use) paired with a material.
 */
type Geometry struct {
	/** @brief The geometry identifier. */
	ID uint32
	/** @brief The internal geometry identifier, used by the renderer backend to map to internal resources. */
	InternalID uint32
	/** @brief The geometry generation. Incremented every time the geometry changes. */
	Generation uint16
	/** @brief The center of the geometry in local coordinates. */
	Center math.Vec3
	/** @brief The extents of the geometry in local coordinates. */
	Extents math.Extents3D
	/** @brief The geometry name. */
	Name [GeometryNameMaxLength]string
	/** @brief A pointer to the material associated with this geometry.. */
	Material *Material
}

type Mesh struct {
	Generation     uint8
	Geometry_count uint16
	Geometries     []Geometry
	Transform      math.Transform
}

type Skybox struct {
	Cubemap    TextureMap
	Geometry   *Geometry
	InstanceID uint32
	/** @brief Synced to the renderer's current frame number when the material has been applied that frame. */
	RenderFrameNumber uint64
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
	/** @brief The length of the name. */
	NameLength uint8
	/** @brief The name of the attribute. */
	Name string
	/** @brief The size of the attribute. */
	Size uint8
	/** @brief The type of the attribute. */
	ShaderAttributeType ShaderAttributeType
}

/** @brief Configuration for a uniform. */
type ShaderUniformConfig struct {
	/** @brief The length of the name. */
	NameLength uint8
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
	/** @brief The count of attributes. */
	AttributeCount uint8
	/** @brief The collection of attributes. Darray. */
	Attributes []*ShaderAttributeConfig
	/** @brief The count of uniforms. */
	UniformCount uint8
	/** @brief The collection of uniforms. Darray. */
	Uniforms []*ShaderUniformConfig
	/** @brief The name of the renderpass used by this shader. */
	RenderpassName string
	/** @brief The number of stages present in the shader. */
	StageCount uint8
	/** @brief The collection of stages. Darray. */
	Stages []ShaderStage
	/** @brief The collection of stage names. Must align with stages array. Darray. */
	StageNames []string
	/** @brief The collection of stage file names to be loaded (one per stage). Must align with stages array. Darray. */
	StageFilenames []string
}
