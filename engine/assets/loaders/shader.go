package loaders

import (
	"os"

	"github.com/spaghettifunk/anima/engine/renderer/metadata"
)

type ShaderLoader struct{}

func (sl *ShaderLoader) Load(path string, assetType metadata.ResourceType, params interface{}) (*metadata.Resource, error) {
	// Read SPIR-V binary file and return a shader object or binary data
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return &metadata.Resource{
		Name:     "",
		FullPath: path,
		DataSize: uint64(len(data)),
		Data:     data,
	}, nil // or return a struct wrapping the shader binary data
}

func (sl *ShaderLoader) Unload(*metadata.Resource) error {
	return nil
}
