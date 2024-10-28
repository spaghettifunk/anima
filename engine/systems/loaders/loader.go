package loaders

import "github.com/spaghettifunk/anima/engine/renderer/metadata"

const (
	InvalidIDUint64 uint64 = 18446744073709551615
	InvalidID       uint32 = 4294967295
	InvalidIDUint16 uint16 = 65535
	InvalidIDUint8  uint8  = 255
)

/** @brief An "interface" for a resource loader. All registered loaders use this. */
type ResourceLoader struct {
	/** @brief The loader identifier. */
	ID uint32
	/** @brief The loader resource type. */
	ResourceType metadata.ResourceType
	/** @brief The loader custom type string, if type is set to custom. */
	CustomType string
	/** @brief A type path which is prepended for the asset type. */
	TypePath string

	ResourceLoaderInterface
}

type ResourceLoaderInterface interface {
	Load(name string, params interface{}) (*metadata.Resource, error)
	Unload(resource *metadata.Resource) error
}
