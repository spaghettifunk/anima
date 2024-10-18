package metadata

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

/** @brief The configuration for the material system. */
type MaterialSystemConfig struct {
	/** @brief The maximum number of loaded materials. */
	MaxMaterialCount uint32
}

type MaterialReference struct {
	ReferenceCount uint64
	Handle         uint32
	AutoRelease    bool
}
