package loaders

/*
#cgo CFLAGS: -I../vendors
#define STB_IMAGE_IMPLEMENTATION
#include "../vendors/stb_image.h"
*/
import "C"
import (
	"fmt"
	"unsafe"

	"github.com/spaghettifunk/anima/engine/renderer/metadata"
)

type ImageLoader struct{}

// loadImage loads an image from a specified path.
func stbLoadImage(path string, flip bool) ([]uint8, int, int, int) {
	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))

	flipY := 0
	if flip {
		flipY = 1
	}

	C.stbi_set_flip_vertically_on_load_thread(C.int(flipY))

	var width, height, channels C.int
	data := C.stbi_load(cPath, &width, &height, &channels, 0)
	if data == nil {
		fmt.Println("Failed to load image")
		return nil, 0, 0, 0
	}
	defer C.stbi_image_free(unsafe.Pointer(data))

	size := int(width) * int(height) * int(channels)
	goData := C.GoBytes(unsafe.Pointer(data), C.int(size))

	return goData, int(width), int(height), int(channels)
}

func (il *ImageLoader) Load(path string, assetType metadata.ResourceType, params interface{}) (*metadata.Resource, error) {
	typedParams := params.(*metadata.ImageResourceParams)

	goData, width, height, channels := stbLoadImage(path, typedParams.FlipY)

	return &metadata.Resource{
		Name:     "image",
		FullPath: path,
		DataSize: uint64(len(goData)),
		Data: &metadata.ImageResourceData{
			ChannelCount: uint8(channels),
			Width:        uint32(width),
			Height:       uint32(height),
			Pixels:       goData,
		},
	}, nil // Return the decoded image object
}

func (il *ImageLoader) Unload(*metadata.Resource) error {
	return nil
}
