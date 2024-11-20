package loaders

import "github.com/spaghettifunk/anima/engine/renderer/metadata"

type SystemFontLoader struct{}

func (fl *SystemFontLoader) Load(path string, assetType metadata.ResourceType, params interface{}) (*metadata.Resource, error) {
	return nil, nil
}

func (fl *SystemFontLoader) Unload(*metadata.Resource) error {
	return nil
}
