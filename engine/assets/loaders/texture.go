package loaders

import (
	"image"
	"os"

	"github.com/spaghettifunk/anima/engine/renderer/metadata"
)

type TextureLoader struct{}

func (tl *TextureLoader) Load(path string, assetType metadata.ResourceType, params interface{}) (*metadata.Resource, error) {
	// Open and decode the texture image file
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return nil, err
	}

	img, _, err := image.Decode(file) // Decodes the image (e.g., PNG, JPEG)
	if err != nil {
		return nil, err
	}
	return &metadata.Resource{
		Name:     "",
		FullPath: path,
		DataSize: uint64(info.Size()),
		Data:     img,
	}, nil // Return the decoded image object
}

func (tl *TextureLoader) Unload(*metadata.Resource) error {
	return nil
}
