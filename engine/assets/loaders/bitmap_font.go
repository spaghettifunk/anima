package loaders

import (
	"fmt"
	"os"
	"unsafe"

	_ "image/png"

	"github.com/fzipp/bmfont"
	"github.com/spaghettifunk/anima/engine/renderer/metadata"
)

type BitmapFontLoader struct {
	ResourcePath string
}

type BitmapFontFileType int

const (
	BITMAP_FONT_FILE_TYPE_NOT_FOUND BitmapFontFileType = iota
	BITMAP_FONT_FILE_TYPE_KBF
	BITMAP_FONT_FILE_TYPE_FNT
)

type SupportedBitmapFontFileType struct {
	Extension  string
	BitmapType BitmapFontFileType
	IsBinary   bool
}

func (fl *BitmapFontLoader) Load(path string, assetType metadata.ResourceType, params interface{}) (*metadata.Resource, error) {
	// Generate the KBF filename.
	rd, err := fl.importFNTFile(path)
	if err != nil {
		return nil, err
	}

	res := &metadata.Resource{
		FullPath: path,
		Data:     rd,
		DataSize: uint64(unsafe.Sizeof(&metadata.BitmapFontResourceData{})),
	}

	return res, nil
}

func (fl *BitmapFontLoader) Unload(resource *metadata.Resource) error {
	if resource.Data != nil {
		data := resource.Data.(*metadata.BitmapFontResourceData)
		data.Data.Glyphs = nil
		data.Pages = nil
		data.Data.Kernings = nil
		resource.Data = nil
		resource.DataSize = 0
		resource.LoaderID = metadata.InvalidID
		resource.FullPath = ""
	}
	return nil
}

func (fl *BitmapFontLoader) importFNTFile(kbf_file_name string) (*metadata.BitmapFontResourceData, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	file := fmt.Sprintf("%s/%s", wd, kbf_file_name)
	descriptor, err := bmfont.LoadDescriptor(file)
	if err != nil {
		return nil, err
	}

	out_data := &metadata.BitmapFontResourceData{
		Data: &metadata.FontData{
			Face:       descriptor.Info.Face,
			Size:       uint32(descriptor.Info.Size),
			LineHeight: int32(descriptor.Common.LineHeight),
			Baseline:   int32(descriptor.Common.Base),
			AtlasSizeX: int32(descriptor.Common.ScaleH),
			AtlasSizeY: int32(descriptor.Common.ScaleW),
			Glyphs:     make([]*metadata.FontGlyph, len(descriptor.Chars)),
			Kernings:   make([]*metadata.FontKerning, len(descriptor.Kerning)),
		},
		Pages: make([]*metadata.BitmapFontPage, len(descriptor.Pages)),
	}

	i := 0
	for _, p := range descriptor.Pages {
		out_data.Pages[i] = &metadata.BitmapFontPage{
			ID:   int8(p.ID),
			File: p.File,
		}
		i++
	}

	i = 0
	for _, g := range descriptor.Chars {
		out_data.Data.Glyphs[i] = &metadata.FontGlyph{
			Codepoint: g.ID,
			Height:    uint16(g.Height),
			Width:     uint16(g.Width),
			X:         uint16(g.X),
			Y:         uint16(g.Y),
			XAdvance:  int16(g.XAdvance),
			XOffset:   int16(g.XOffset),
			YOffset:   int16(g.YOffset),
			PageID:    uint8(g.Page),
		}
		i++
	}

	i = 0
	for p, k := range descriptor.Kerning {
		out_data.Data.Kernings[i] = &metadata.FontKerning{
			Amount:     int16(k.Amount),
			Codepoint0: p.First,
			Codepoint1: p.Second,
		}
		i++
	}

	return out_data, nil
}
