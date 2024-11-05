package loaders

import (
	"os"

	"github.com/spaghettifunk/anima/engine/renderer/metadata"
)

type ModelLoader struct{}

func (ml *ModelLoader) Load(path string, assetType metadata.ResourceType, params interface{}) (*metadata.Resource, error) {
	// Read and parse the model file (e.g., OBJ, FBX)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	// Implement parsing based on file format and return a model struct
	model := ml.parseModelData(data)
	return model, nil
}

func (ml *ModelLoader) parseModelData(data []byte) *metadata.Resource {
	return nil
}

func (ml *ModelLoader) Unload(*metadata.Resource) error {
	return nil
}
