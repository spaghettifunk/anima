package metadata

/** @brief The texture system configuration */
type TextureSystemConfig struct {
	/** @brief The maximum number of textures that can be loaded at once. */
	MaxTextureCount uint32
}

const (
	/** @brief The default texture name. */
	DEFAULT_TEXTURE_NAME string = "default"
	/** @brief The default diffuse texture name. */
	DEFAULT_DIFFUSE_TEXTURE_NAME string = "default_DIFF"
	/** @brief The default specular texture name. */
	DEFAULT_SPECULAR_TEXTURE_NAME string = "default_SPEC"
	/** @brief The default normal texture name. */
	DEFAULT_NORMAL_TEXTURE_NAME string = "default_NORM"
)

type TextureReference struct {
	ReferenceCount uint64
	Handle         uint32
	AutoRelease    bool
}

// Also used as result_data from job.
type TextureLoadParams struct {
	ResourceName      string
	OutTexture        *Texture
	TempTexture       *Texture
	CurrentGeneration uint32
	ImageResource     *Resource
}

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
