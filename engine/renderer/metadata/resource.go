package metadata

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

type Skybox struct {
	Cubemap    *TextureMap
	Geometry   *Geometry
	InstanceID uint32
	/** @brief Synced to the renderer's current frame number when the material has been applied that frame. */
	RenderFrameNumber uint64
}
