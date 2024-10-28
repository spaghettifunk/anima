package metadata

import "github.com/spaghettifunk/anima/engine/math"

/** @brief The name of the default material. */
const DefaultMaterialName string = "default"

type MaterialShaderUniformLocations struct {
	Projection      uint16
	View            uint16
	AmbientColour   uint16
	ViewPosition    uint16
	Shininess       uint16
	DiffuseColour   uint16
	DiffuseTexture  uint16
	SpecularTexture uint16
	NormalTexture   uint16
	Model           uint16
	RenderMode      uint16
}

type UIShaderUniformLocations struct {
	Projection     uint16
	View           uint16
	DiffuseColour  uint16
	DiffuseTexture uint16
	Model          uint16
}

type MaterialReference struct {
	ReferenceCount uint64
	Handle         uint32
	AutoRelease    bool
}

/**
 * @brief Material configuration typically loaded from
 * a file or created in code to load a material from.
 */
type MaterialConfig struct {
	/** @brief The name of the material. */
	Name string
	/** @brief The material type. */
	ShaderName string
	/** @brief Indicates if the material should be automatically released when no references to it remain. */
	AutoRelease bool
	/** @brief The diffuse colour of the material. */
	DiffuseColour math.Vec4
	/** @brief The shininess of the material. */
	Shininess float32
	/** @brief The diffuse map name. */
	DiffuseMapName string
	/** @brief The specular map name. */
	SpecularMapName string
	/** @brief The normal map name. */
	NormalMapName string
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
	Name string
	/** @brief The diffuse colour. */
	DiffuseColour math.Vec4
	/** @brief The diffuse texture map. */
	DiffuseMap *TextureMap
	/** @brief The specular texture map. */
	SpecularMap *TextureMap
	/** @brief The normal texture map. */
	NormalMap *TextureMap
	/** @brief The material shininess, determines how concentrated the specular lighting is. */
	Shininess float32
	ShaderID  uint32
	/** @brief Synced to the renderer's current frame number when the material has been applied that frame. */
	RenderFrameNumber uint32
}
