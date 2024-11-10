package loaders

import (
	"fmt"
	"io"
	"os"

	"github.com/spaghettifunk/anima/engine/renderer/metadata"
)

type BinaryLoader struct{}

func (bl *BinaryLoader) Load(path string, assetType metadata.ResourceType, params interface{}) (*metadata.Resource, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	buf, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}

	res := bytesToBytecode(buf)

	p, ok := params.(map[string]string)
	if !ok {
		return nil, fmt.Errorf("failed to cast params in binary loader")
	}

	return &metadata.Resource{
		Name:     p["name"],
		FullPath: path,
		DataSize: uint64(len(res)),
		Data:     res,
	}, nil
}

func (bl *BinaryLoader) Unload(*metadata.Resource) error {
	return nil
}

func bytesToBytecode(b []byte) []uint32 {
	byteCode := make([]uint32, len(b)/4)
	for i := 0; i < len(byteCode); i++ {
		byteIndex := i * 4
		byteCode[i] = 0
		byteCode[i] |= uint32(b[byteIndex])
		byteCode[i] |= uint32(b[byteIndex+1]) << 8
		byteCode[i] |= uint32(b[byteIndex+2]) << 16
		byteCode[i] |= uint32(b[byteIndex+3]) << 24
	}

	return byteCode
}
