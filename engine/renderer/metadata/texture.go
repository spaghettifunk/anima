package metadata

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

type DefaultTexture struct {
	DefaultTexture         *Texture
	TexturePixels          []uint8
	DefaultDiffuseTexture  *Texture
	DiffuseTexturePixels   []uint8
	DefaultSpecularTexture *Texture
	SpecularTexturePixels  []uint8
	DefaultNormalTexture   *Texture
	NormalTexturePixels    []uint8
}

func NewDefaultTexture() *DefaultTexture {
	return &DefaultTexture{
		DefaultTexture:         &Texture{},
		DefaultDiffuseTexture:  &Texture{},
		DefaultSpecularTexture: &Texture{},
		DefaultNormalTexture:   &Texture{},
	}
}

// CreateSkeletonTexture misses the call to the actual renderer to properly generate the texture
// this method creates the shell of the objects that will hold the texture
func (ts *DefaultTexture) CreateSkeletonTextures() bool {
	// NOTE: Create default texture, a 256x256 blue/white checkerboard pattern.
	// This is done in code to eliminate asset dependencies.
	// KTRACE("Creating default texture...");
	texDimension := uint32(256)
	channels := uint32(4)
	pixelCount := uint32(texDimension * texDimension)

	pixels := make([]uint8, pixelCount*channels)

	// Each pixel.
	for row := uint32(0); row < texDimension; row++ {
		for col := uint32(0); col < texDimension; col++ {
			index := uint32((row * texDimension) + col)
			index_bpp := uint32(index * channels)
			if row%2 != 0 {
				if col%2 != 0 {
					pixels[index_bpp+0] = 0
					pixels[index_bpp+1] = 0
				}
			} else {
				if col%2 == 0 {
					pixels[index_bpp+0] = 0
					pixels[index_bpp+1] = 0
				}
			}
		}
	}

	ts.DefaultTexture.Name = DEFAULT_TEXTURE_NAME

	ts.DefaultTexture.Width = texDimension
	ts.DefaultTexture.Height = texDimension
	ts.DefaultTexture.ChannelCount = 4
	ts.DefaultTexture.Generation = InvalidID
	ts.DefaultTexture.Flags = 0
	ts.DefaultTexture.TextureType = TextureType2d
	ts.TexturePixels = pixels

	// ts.renderer.TextureCreate(pixels, ts.DefaultTexture)

	// Manually set the texture generation to invalid since this is a default texture.
	ts.DefaultTexture.Generation = InvalidID

	// Diffuse texture.
	// KTRACE("Creating default diffuse texture...");
	diffPixels := make([]uint8, 16*16*4)
	// Default diffuse map is all white.

	ts.DefaultDiffuseTexture.Name = DEFAULT_DIFFUSE_TEXTURE_NAME
	ts.DefaultDiffuseTexture.Width = 16
	ts.DefaultDiffuseTexture.Height = 16
	ts.DefaultDiffuseTexture.ChannelCount = 4
	ts.DefaultDiffuseTexture.Generation = InvalidID
	ts.DefaultDiffuseTexture.Flags = 0
	ts.DefaultDiffuseTexture.TextureType = TextureType2d
	ts.DiffuseTexturePixels = diffPixels

	// ts.renderer.TextureCreate(diffPixels, ts.DefaultDiffuseTexture)

	// Manually set the texture generation to invalid since this is a default texture.
	ts.DefaultDiffuseTexture.Generation = InvalidID

	// Specular texture.
	// KTRACE("Creating default specular texture...");
	specPixels := make([]uint8, 16*16*4)
	// Default spec map is black (no specular)

	ts.DefaultSpecularTexture.Name = DEFAULT_SPECULAR_TEXTURE_NAME
	ts.DefaultSpecularTexture.Width = 16
	ts.DefaultSpecularTexture.Height = 16
	ts.DefaultSpecularTexture.ChannelCount = 4
	ts.DefaultSpecularTexture.Generation = InvalidID
	ts.DefaultSpecularTexture.Flags = 0
	ts.DefaultSpecularTexture.TextureType = TextureType2d
	ts.SpecularTexturePixels = specPixels

	// ts.renderer.TextureCreate(specPixels, ts.DefaultSpecularTexture)

	// Manually set the texture generation to invalid since this is a default texture.
	ts.DefaultSpecularTexture.Generation = InvalidID

	// Normal texture.
	// KTRACE("Creating default normal texture...");
	normalPixels := make([]uint8, 16*16*4) // w * h * channels

	// Each pixel.
	for row := 0; row < 16; row++ {
		for col := 0; col < 16; col++ {
			index := uint32((row * 16) + col)
			index_bpp := index * channels
			// Set blue, z-axis by default and alpha.
			normalPixels[index_bpp+0] = 128
			normalPixels[index_bpp+1] = 128
			normalPixels[index_bpp+2] = 255
			normalPixels[index_bpp+3] = 255
		}
	}

	ts.DefaultNormalTexture.Name = DEFAULT_NORMAL_TEXTURE_NAME
	ts.DefaultNormalTexture.Width = 16
	ts.DefaultNormalTexture.Height = 16
	ts.DefaultNormalTexture.ChannelCount = 4
	ts.DefaultNormalTexture.Generation = InvalidID
	ts.DefaultNormalTexture.Flags = 0
	ts.DefaultNormalTexture.TextureType = TextureType2d
	ts.NormalTexturePixels = normalPixels

	// ts.renderer.TextureCreate(normalPixels, ts.DefaultNormalTexture)

	// Manually set the texture generation to invalid since this is a default texture.
	ts.DefaultNormalTexture.Generation = InvalidID

	return true
}

func (ts *DefaultTexture) DestroyDefaultTextures() {
	ts.DestroySkeletonTexture(ts.DefaultTexture)
	ts.DestroySkeletonTexture(ts.DefaultDiffuseTexture)
	ts.DestroySkeletonTexture(ts.DefaultSpecularTexture)
	ts.DestroySkeletonTexture(ts.DefaultNormalTexture)
}

func (ts *DefaultTexture) DestroySkeletonTexture(texture *Texture) {
	// Clean up backend resources.
	// ts.renderer.TextureDestroy(texture)

	texture.ID = InvalidID
	texture.Generation = InvalidID
}
