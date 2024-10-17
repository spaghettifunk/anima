package loaders

import "github.com/spaghettifunk/anima/engine/resources"

type ShaderLoader struct {
}

func (sl *ShaderLoader) Load(name string, params interface{}) (*resources.Resource, error) {
	return nil, nil
}

func (sl *ShaderLoader) Unload(resource *resources.Resource) error {
	return nil
}
