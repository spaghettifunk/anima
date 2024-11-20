package systems

/*
#cgo CFLAGS: -I../vendors
#define STB_TRUETYPE_IMPLEMENTATION
#include "../vendors/stb_truetype.h"
*/
import "C"
import "github.com/spaghettifunk/anima/engine/renderer/metadata"

type BitmapFontInternalData struct {
	LoadedResource *metadata.Resource
	// Casted pointer to resource data for convenience.
	ResourceData *metadata.BitmapFontResourceData
}

type SystemFontVariantData struct {
	// darray
	Codepoints []int32
	Scale      float32
}

type BitmapFontLookup struct {
	ID             uint16
	ReferenceCount uint16
	Font           *BitmapFontInternalData
}

type SystemFontLookup struct {
	ID             uint16
	ReferenceCount uint16
	SizeVariants   []*metadata.FontData
	// A copy of all this is kept for each for convenience.
	BinarySize uint64
	Face       string
	FontBinary interface{}
	Offset     int32
	Index      int32
	Info       C.stbtt_fontinfo
}

type FontSystem struct {
	Config           *metadata.FontSystemConfig
	BitmapFontLookup map[string]string
	SystemFontLookup map[string]string
	BitmapFonts      []*BitmapFontLookup
	SystemFonts      []*SystemFontLookup
}
