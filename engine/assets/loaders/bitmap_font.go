package loaders

import (
	"fmt"
	"os"
	"unsafe"

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
	format_str := "%s/%s/%s%s"

	// Supported extensions. Note that these are in order of priority when looked up.
	// This is to prioritize the loading of a binary version of the bitmap font, followed by
	// importing various types of bitmap fonts to binary types, which would be loaded on the
	// next run.
	// TODO: Might be good to be able to specify an override to always import (i.e. skip
	// binary versions) for debug purposes.
	supported_filetypes := []*SupportedBitmapFontFileType{
		{
			Extension:  ".fnt",
			BitmapType: BITMAP_FONT_FILE_TYPE_FNT,
			IsBinary:   false,
		},
		// {
		// 	Extension:  ".kbf",
		// 	BitmapType: BITMAP_FONT_FILE_TYPE_KBF,
		// 	IsBinary:   true,
		// },
	}

	p, ok := params.(map[string]string)
	if !ok {
		return nil, fmt.Errorf("failed to cast params in bitmap font loader")
	}

	var full_file_path string
	bitmapType := BITMAP_FONT_FILE_TYPE_NOT_FOUND
	// Try each supported extension.
	for i := 0; i < len(supported_filetypes); i++ {
		full_file_path = fmt.Sprintf(format_str, fl.ResourcePath, "fonts", p["name"], supported_filetypes[i].Extension)
		// If the file exists, open it and stop looking.
		if _, err := os.Stat(full_file_path); err != nil {
			return nil, err
		}

		// we found the first file with matching extension
		bitmapType = supported_filetypes[i].BitmapType
		break
	}

	if bitmapType == BITMAP_FONT_FILE_TYPE_NOT_FOUND {
		err := fmt.Errorf("unable to find bitmap font of supported type called '%s'", p["name"])
		return nil, err
	}

	resource_data := &metadata.BitmapFontResourceData{
		Data: &metadata.FontData{
			FontType: metadata.FONT_TYPE_BITMAP,
		},
	}

	switch bitmapType {
	case BITMAP_FONT_FILE_TYPE_FNT:
		{
			// Generate the KBF filename.
			fnt_file_name := fmt.Sprintf("%s/%s/%s%s", fl.ResourcePath, "fonts", p["name"], ".fnt")
			rd, err := fl.importFNTFile(fnt_file_name)
			if err != nil {
				return nil, err
			}
			resource_data = rd
			break
		}
	// case BITMAP_FONT_FILE_TYPE_KBF:
	// if err := fl.readKBFFile(bfFile, resource_data); err != nil {
	// return nil, err
	// }
	case BITMAP_FONT_FILE_TYPE_NOT_FOUND:
		err := fmt.Errorf("unable to find bitmap font of supported type called '%s'", p["name"])
		return nil, err
	}

	out_resource := &metadata.Resource{
		FullPath: full_file_path,
		Data:     resource_data,
		DataSize: uint64(unsafe.Sizeof(&metadata.BitmapFontResourceData{})),
	}

	return out_resource, nil
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
	font, err := bmfont.Load(kbf_file_name)
	if err != nil {
		return nil, err
	}

	out_data := &metadata.BitmapFontResourceData{
		Data: &metadata.FontData{
			Face:       font.Descriptor.Info.Face,
			Size:       uint32(font.Descriptor.Info.Size),
			LineHeight: int32(font.Descriptor.Common.LineHeight),
			Baseline:   int32(font.Descriptor.Common.Base),
			AtlasSizeX: int32(font.Descriptor.Common.ScaleH),
			AtlasSizeY: int32(font.Descriptor.Common.ScaleW),
			Glyphs:     make([]*metadata.FontGlyph, len(font.Descriptor.Chars)),
			Kernings:   make([]*metadata.FontKerning, len(font.Descriptor.Kerning)),
		},
		Pages: make([]*metadata.BitmapFontPage, len(font.Descriptor.Pages)),
	}

	i := 0
	for _, p := range font.Descriptor.Pages {
		out_data.Pages[i].ID = int8(p.ID)
		out_data.Pages[i].File = p.File
		i++
	}

	i = 0
	for _, g := range font.Descriptor.Chars {
		out_data.Data.Glyphs[i].Codepoint = g.ID
		out_data.Data.Glyphs[i].Height = uint16(g.Height)
		out_data.Data.Glyphs[i].Width = uint16(g.Width)
		out_data.Data.Glyphs[i].X = uint16(g.X)
		out_data.Data.Glyphs[i].Y = uint16(g.Y)
		out_data.Data.Glyphs[i].XAdvance = int16(g.XAdvance)
		out_data.Data.Glyphs[i].XOffset = int16(g.XOffset)
		out_data.Data.Glyphs[i].YOffset = int16(g.YOffset)
		out_data.Data.Glyphs[i].PageID = uint8(g.Page)
		i++
	}

	i = 0
	for p, k := range font.Descriptor.Kerning {
		out_data.Data.Kernings[i].Amount = int16(k.Amount)
		out_data.Data.Kernings[i].Codepoint0 = p.First
		out_data.Data.Kernings[i].Codepoint1 = p.Second
		i++
	}

	return out_data, nil
}
