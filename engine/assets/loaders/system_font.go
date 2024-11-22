package loaders

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"unsafe"

	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"

	"github.com/spaghettifunk/anima/engine/renderer/metadata"
)

type SystemFontLoader struct{}

func (fl *SystemFontLoader) Load(path string, assetType metadata.ResourceType, params interface{}) (*metadata.Resource, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	rd := &metadata.SystemFontResourceData{
		Fonts: []*metadata.SystemFontFace{},
	}
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip comments and empty lines
		if len(line) == 0 || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse the file and face keys
		if strings.HasPrefix(line, "file=") {
			filename := strings.TrimPrefix(line, "file=")
			fullPath := fmt.Sprintf("%s/assets/fonts/%s", wd, filename)
			// Read the font data.
			fontBytes, err := os.ReadFile(fullPath)
			if err != nil {
				return nil, err
			}
			f, err := opentype.ParseCollection(fontBytes)
			if err != nil {
				return nil, err
			}
			rd.FontBinary = f
			rd.BinarySize = uint64(unsafe.Sizeof(&sfnt.Collection{}))
		} else if strings.HasPrefix(line, "face=") {
			face := strings.TrimPrefix(line, "face=")
			rd.Fonts = append(rd.Fonts, &metadata.SystemFontFace{
				Name: face,
			})
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	res := &metadata.Resource{
		FullPath: path,
		Data:     rd,
		DataSize: uint64(unsafe.Sizeof(&metadata.SystemFontResourceData{})),
	}

	return res, nil
}

func (fl *SystemFontLoader) Unload(*metadata.Resource) error {
	return nil
}
