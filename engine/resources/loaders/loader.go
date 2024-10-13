package loaders

import "github.com/spaghettifunk/anima/engine/resources"

const InvalidID uint32 = 99999

/** @brief An "interface" for a resource loader. All registered loaders use this. */
type ResourceLoader struct {
	/** @brief The loader identifier. */
	ID uint32
	/** @brief The loader resource type. */
	ResourceType resources.ResourceType
	/** @brief The loader custom type string, if type is set to custom. */
	CustomType string
	/** @brief A type path which is prepended for the asset type. */
	TypePath string

	ResourceLoaderInterface
}

type ResourceLoaderInterface interface {
	Load(name string, params interface{}) (*resources.Resource, error)
	Unload(resource *resources.Resource) error
}
