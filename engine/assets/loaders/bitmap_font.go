package loaders

import "github.com/spaghettifunk/anima/engine/renderer/metadata"

type BitmapFontLoader struct{}

func (fl *BitmapFontLoader) Load(path string, assetType metadata.ResourceType, params interface{}) (*metadata.Resource, error) {
	return nil, nil
}

func (fl *BitmapFontLoader) Unload(*metadata.Resource) error {
	return nil
}

