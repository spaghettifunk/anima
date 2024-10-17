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
