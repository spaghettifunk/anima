package loaders

import "github.com/spaghettifunk/anima/engine/renderer/metadata"

type ShaderLoader struct {
}

func (sl *ShaderLoader) Load(name string, params interface{}) (*metadata.Resource, error) {
	return nil, nil
}

func (sl *ShaderLoader) Unload(resource *metadata.Resource) error {
	return nil
}
