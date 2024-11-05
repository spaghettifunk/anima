package assets

import "github.com/spaghettifunk/anima/engine/renderer/metadata"

type Loader interface {
	Load(path string, assetType metadata.ResourceType, params interface{}) (*metadata.Resource, error) // `interface{}` here allows loaders to return various asset types
	Unload(*metadata.Resource) error
}
