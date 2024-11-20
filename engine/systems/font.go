package systems

/*
#cgo CFLAGS: -I../vendors
#define STB_TRUETYPE_IMPLEMENTATION
#include "../vendors/stb_truetype.h"
*/
import "C"
import (
	"fmt"
	"unsafe"

	"github.com/spaghettifunk/anima/engine/assets"
	"github.com/spaghettifunk/anima/engine/core"
	"github.com/spaghettifunk/anima/engine/renderer/metadata"
)

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

type FontSystemConfig struct {
	DefaultSystemFontCount uint8
	SystemFontConfigs      []*metadata.SystemFontConfig
	DefaultBitmapFontCount uint8
	BitmapFontConfigs      []*metadata.BitmapFontConfig
	MaxSystemFontCount     uint8
	MaxBitmapFontCount     uint8
	AutoRelease            bool
}

type FontSystem struct {
	Config           *FontSystemConfig
	BitmapFontLookup map[string]uint16
	SystemFontLookup map[string]uint16
	BitmapFonts      []*BitmapFontLookup
	SystemFonts      []*SystemFontLookup
	// subsystems
	textureSystem  *TextureSystem
	assetManager   *assets.AssetManager
	rendererSystem *RendererSystem
}

func NewFontSystem(config *FontSystemConfig, ts *TextureSystem, am *assets.AssetManager, r *RendererSystem) (*FontSystem, error) {
	fs := &FontSystem{
		Config:           config,
		BitmapFontLookup: make(map[string]uint16),
		SystemFontLookup: make(map[string]uint16),
		BitmapFonts:      make([]*BitmapFontLookup, config.MaxSystemFontCount),
		SystemFonts:      make([]*SystemFontLookup, config.MaxSystemFontCount),
		textureSystem:    ts,
		assetManager:     am,
		rendererSystem:   r,
	}

	return fs, nil
}

func (fs *FontSystem) Initialize() error {
	if fs.Config.MaxBitmapFontCount == 0 || fs.Config.MaxSystemFontCount == 0 {
		err := fmt.Errorf("font_system_initialize - config.max_bitmap_font_count and config.max_system_font_count must be > 0")
		return err
	}

	// Invalidate all entries in both arrays.
	count := fs.Config.MaxBitmapFontCount
	for i := 0; i < int(count); i++ {
		fs.BitmapFonts[i].ID = metadata.InvalidIDUint16
		fs.BitmapFonts[i].ReferenceCount = 0
	}
	count = fs.Config.MaxSystemFontCount
	for i := 0; i < int(count); i++ {
		fs.SystemFonts[i].ID = metadata.InvalidIDUint16
		fs.SystemFonts[i].ReferenceCount = 0
	}

	// Load up any default fonts.
	// Bitmap fonts.
	for i := 0; i < int(fs.Config.DefaultBitmapFontCount); i++ {
		if err := fs.LoadBitmapFont(fs.Config.BitmapFontConfigs[i]); err != nil {
			core.LogError("failed to load bitmap font: %s", fs.Config.BitmapFontConfigs[i].Name)
			return err
		}
	}
	// System fonts.
	for i := 0; i < int(fs.Config.DefaultSystemFontCount); i++ {
		if err := fs.LoadSystemFont(fs.Config.SystemFontConfigs[i]); err != nil {
			core.LogError("failed to load system font: %s", fs.Config.SystemFontConfigs[i].Name)
			return err
		}
	}

	return nil
}

func (fs *FontSystem) Shutdown() {
	// Cleanup bitmap fonts.
	for i := uint16(0); i < uint16(fs.Config.MaxBitmapFontCount); i++ {
		if fs.BitmapFonts[i].ID != metadata.InvalidIDUint16 {
			data := fs.BitmapFonts[i].Font.ResourceData.Data
			fs.CleanupFontData(data)
			fs.BitmapFonts[i].ID = metadata.InvalidIDUint16
		}
	}

	// Cleanup system fonts.
	for i := uint16(0); i < uint16(fs.Config.MaxSystemFontCount); i++ {
		if fs.SystemFonts[i].ID != metadata.InvalidIDUint16 {
			// Cleanup each variant.
			variant_count := len(fs.SystemFonts[i].SizeVariants)
			for j := 0; j < variant_count; j++ {
				data := fs.SystemFonts[i].SizeVariants[j]
				fs.CleanupFontData(data)
			}
			fs.BitmapFonts[i].ID = metadata.InvalidIDUint16
			fs.SystemFonts[i].SizeVariants = nil
		}
	}
}

func (fs *FontSystem) Release(text *metadata.UIText) error {
	// TODO: Lookup font by name in appropriate hashtable.
	return nil
}

func (fs *FontSystem) CleanupFontData(font *metadata.FontData) {
	// Release the texture map resources.
	fs.rendererSystem.TextureMapReleaseResources(font.Atlas)

	// If a bitmap font, release the reference to the texture.
	if font.FontType == metadata.FONT_TYPE_BITMAP && font.Atlas.Texture != nil {
		fs.textureSystem.Release(font.Atlas.Texture.Name)
	}
	font.Atlas.Texture = nil
}

func (fs *FontSystem) LoadBitmapFont(config *metadata.BitmapFontConfig) error {
	id, ok := fs.BitmapFontLookup[config.Name]
	if ok && id != metadata.InvalidIDUint16 {
		core.LogWarn("a font named '%s' already exists and will not be loaded again", config.Name)
		// Not a hard error, return success since it already exists and can be used.
		return nil
	}

	// Get a new id
	for i := uint16(0); i < uint16(fs.Config.MaxBitmapFontCount); i++ {
		if fs.BitmapFonts[i].ID == metadata.InvalidIDUint16 {
			id = i
			break
		}
	}
	if id == metadata.InvalidIDUint16 {
		err := fmt.Errorf("no space left to allocate a new bitmap font. Increase maximum number allowed in font system config")
		return err
	}

	// Obtain the lookup.
	lookup := fs.BitmapFonts[id]

	res, err := fs.assetManager.LoadAsset(config.ResourceName, metadata.ResourceTypeBitmapFont, nil)
	if err != nil {
		core.LogError("failed to load bitmap font")
		return err
	}
	lookup.Font.LoadedResource = res

	// Keep a casted pointer to the resource data for convenience.
	lookup.Font.ResourceData = lookup.Font.LoadedResource.Data.(*metadata.BitmapFontResourceData)

	// Acquire the texture.
	// TODO: only accounts for one page at the moment.
	text, err := fs.textureSystem.Aquire(lookup.Font.ResourceData.Pages[0].File, true)
	if err != nil {
		return err
	}
	lookup.Font.ResourceData.Data.Atlas.Texture = text

	if err := fs.SetupFontData(lookup.Font.ResourceData.Data); err != nil {
		return err
	}

	// Set the entry id here last before updating the hashtable.
	fs.BitmapFontLookup[config.Name] = id
	lookup.ID = id

	return nil
}

func (fs *FontSystem) LoadSystemFont(config *metadata.SystemFontConfig) error {
	// For system fonts, they can actually contain multiple fonts. For this reason,
	// a copy of the resource's data will be held in each resulting variant, and the
	// resource itself will be released.
	res, err := fs.assetManager.LoadAsset(config.ResourceName, metadata.ResourceTypeSystemFont, nil)
	if err != nil {
		core.LogError("failed to load system font")
		return err
	}

	// Keep a casted pointer to the resource data for convenience.
	resource_data := res.Data.(*metadata.SystemFontResourceData)

	// Loop through the faces and create one lookup for each, as well as a default size
	// variant for each lookup.
	font_face_count := uint32(len(resource_data.Fonts))
	for i := uint32(0); i < font_face_count; i++ {
		face := resource_data.Fonts[i]

		// Make sure a font with this name doesn't already exist.
		id, ok := fs.SystemFontLookup[face.Name]
		if ok && id != metadata.InvalidIDUint16 {
			core.LogWarn("a font named '%s' already exists and will not be loaded again.", config.Name)
			// Not a hard error, return success since it already exists and can be used.
			return nil
		}

		// Get a new id
		for j := uint16(0); j < uint16(fs.Config.MaxSystemFontCount); j++ {
			if fs.SystemFonts[j].ID == metadata.InvalidIDUint16 {
				id = j
				break
			}
		}
		if id == metadata.InvalidIDUint16 {
			err := fmt.Errorf("no space left to allocate a new font. Increase maximum number allowed in font system config")
			return err
		}

		// Obtain the lookup.
		lookup := fs.SystemFonts[id]
		lookup.BinarySize = resource_data.BinarySize
		lookup.FontBinary = resource_data.FontBinary
		lookup.Face = face.Name
		lookup.Index = int32(i)
		// To hold the size variants.
		lookup.SizeVariants = []*metadata.FontData{}

		// The offset
		data := []byte(lookup.FontBinary.([]byte))
		cData := (*C.uchar)(unsafe.Pointer(&data[0]))
		cI := C.int(i)

		offset := C.stbtt_GetFontOffsetForIndex(cData, cI)
		lookup.Offset = int32(offset)

		result := C.stbtt_InitFont(&lookup.Info, cData, offset)
		if result == 0 {
			// Zero indicates failure.
			err := fmt.Errorf("failed to init system font %s at index %d", res.FullPath, i)
			return err
		}

		// Create a default size variant.
		variant, err := fs.CreateSystemFontVariant(lookup, config.DefaultSize, face.Name)
		if err != nil {
			core.LogError("failed to create variant: %s, index %d", face.Name, i)
			core.LogError(err.Error())
			continue
		}

		// Also perform setup for the variant
		if err := fs.SetupFontData(variant); err != nil {
			core.LogError("failed to setup font data")
			core.LogError(err.Error())
			continue
		}

		// Add to the lookup's size variants.
		lookup.SizeVariants = append(lookup.SizeVariants, variant)

		// Set the entry id here last before updating the hashtable.
		lookup.ID = id
		fs.SystemFontLookup[face.Name] = id
	}

	return nil
}

func (fs *FontSystem) SetupFontData(font *metadata.FontData) error {
	// Create map resources
	font.Atlas.FilterMagnify = metadata.TextureFilterModeLinear
	font.Atlas.FilterMinify = metadata.TextureFilterModeLinear
	font.Atlas.RepeatU = metadata.TextureRepeatClampToEdge
	font.Atlas.RepeatV = metadata.TextureRepeatClampToEdge
	font.Atlas.RepeatW = metadata.TextureRepeatClampToEdge
	font.Atlas.Use = metadata.TextureUseMapDiffuse

	if !fs.rendererSystem.TextureMapAcquireResources(font.Atlas) {
		err := fmt.Errorf("unable to acquire resources for font Atlas texture map")
		return err
	}

	// Check for a tab glyph, as there may not always be one exported. If there is, store its
	// x_advance and just use that. If there is not, then create one based off spacex4
	if font.TabXAdvance == 0 {
		for i := 0; i < len(font.Glyphs); i++ {
			if font.Glyphs[i].Codepoint == '\t' {
				font.TabXAdvance = float32(font.Glyphs[i].XAdvance)
				break
			}
		}
		// If still not found, use space x 4.
		if font.TabXAdvance == 0 {
			for i := 0; i < len(font.Glyphs); i++ {
				// Search for space
				if font.Glyphs[i].Codepoint == ' ' {
					font.TabXAdvance = float32(uint16(font.Glyphs[i].XAdvance) * 4)
					break
				}
			}
			if font.TabXAdvance == 0 {
				// If _still_ not there, then a space wasn't present either, so just
				// hardcode something, in this case font size * 4.
				font.TabXAdvance = float32(font.Size * 4)
			}
		}
	}
	return nil
}

func (fs *FontSystem) CreateSystemFontVariant(lookup *SystemFontLookup, size uint16, font_name string) (*metadata.FontData, error) {
	out_variant := &metadata.FontData{
		AtlasSizeX:       1024,
		AtlasSizeY:       1024,
		Size:             uint32(size),
		FontType:         metadata.FONT_TYPE_SYSTEM,
		Face:             font_name,
		InternalDataSize: uint32(unsafe.Sizeof(SystemFontVariantData{})),
		InternalData:     SystemFontVariantData{},
	}

	internal_data := out_variant.InternalData.(SystemFontVariantData)

	// Push default codepoints (ascii 32-127) always, plus a -1 for unknown.
	internal_data.Codepoints = make([]int32, 96)
	internal_data.Codepoints = append(internal_data.Codepoints, -1) // push invalid char
	for i := 0; i < 95; i++ {
		internal_data.Codepoints[i+1] = int32(i + 32)
	}

	// Create texture.
	font_tex_name := fmt.Sprintf("__system_text_atlas_%s_i%d_sz%d__", font_name, lookup.Index, size)
	text, err := fs.textureSystem.AquireWriteable(font_tex_name, uint32(out_variant.AtlasSizeX), uint32(out_variant.AtlasSizeY), 4, true)
	if err != nil {
		return nil, err
	}
	out_variant.Atlas.Texture = text

	// Obtain some metrics
	scale := C.stbtt_ScaleForPixelHeight(&lookup.Info, C.float(size))
	internal_data.Scale = float32(scale)

	var ascent, descent, line_gap C.int
	C.stbtt_GetFontVMetrics(&lookup.Info, &ascent, &descent, &line_gap)
	out_variant.LineHeight = (int32(ascent) - int32(descent) + int32(line_gap)) * int32(internal_data.Scale)

	if err := fs.RebuildSystemFontVariantAtlas(lookup, out_variant); err != nil {
		return nil, err
	}

	return out_variant, nil
}

func (fs *FontSystem) RebuildSystemFontVariantAtlas(lookup *SystemFontLookup, variant *metadata.FontData) error {
	internal_data := variant.InternalData.(*SystemFontVariantData)

	pack_image_size := variant.AtlasSizeX * variant.AtlasSizeY * int32(unsafe.Sizeof(uint8(1)))
	pixels := make([]uint8, pack_image_size)
	codepoint_count := len(internal_data.Codepoints)
	packed_chars := make([]*C.stbtt_packedchar, codepoint_count)

	// Begin packing all known characters into the atlas. This
	// creates a single-channel image with rendered glyphs at the
	// given size.
	cData := (*C.uchar)(unsafe.Pointer(&pixels[0]))
	var context C.stbtt_pack_context
	if C.stbtt_PackBegin(&context, cData, C.int(variant.AtlasSizeX), C.int(variant.AtlasSizeY), 0, 1, nil) == 0 {
		err := fmt.Errorf("stbtt_PackBegin failed")
		return err
	}

	// Fit all codepoints into a single range for packing.
	pack_range := C.stbtt_pack_range{
		first_unicode_codepoint_in_range: 0,
		font_size:                        C.float(variant.Size),
		num_chars:                        C.int(codepoint_count),
		chardata_for_range:               packed_chars[0],
		array_of_unicode_codepoints:      (*C.int)(unsafe.Pointer(&internal_data.Codepoints[0])),
	}
	data := []byte(lookup.FontBinary.([]byte))
	cData = (*C.uchar)(unsafe.Pointer(&data[0]))
	if C.stbtt_PackFontRanges(&context, cData, C.int(lookup.Index), &pack_range, 1) == 0 {
		err := fmt.Errorf("stbtt_PackFontRanges failed")
		return err
	}

	C.stbtt_PackEnd(&context)
	// Packing complete.

	// Convert from single-channel to RGBA, or pack_image_size * 4.
	rgba_pixels := make([]uint8, pack_image_size*4)
	for j := int32(0); j < pack_image_size; j++ {
		rgba_pixels[(j*4)+0] = pixels[j]
		rgba_pixels[(j*4)+1] = pixels[j]
		rgba_pixels[(j*4)+2] = pixels[j]
		rgba_pixels[(j*4)+3] = pixels[j]
	}

	// Write texture data to atlas.
	if !fs.textureSystem.WriteData(variant.Atlas.Texture, 0, uint32(pack_image_size*4), rgba_pixels) {
		err := fmt.Errorf("failed to write data for font variant atlas")
		return err
	}

	// Free pixel/rgba_pixel data.
	pixels = nil
	rgba_pixels = nil

	// Regenerate glyphs
	variant.Glyphs = make([]*metadata.FontGlyph, codepoint_count)

	for i := 0; i < len(variant.Glyphs); i++ {
		pc := packed_chars[i]
		g := variant.Glyphs[i]
		g.Codepoint = internal_data.Codepoints[i]
		g.PageID = 0
		g.XOffset = int16(pc.xoff)
		g.YOffset = int16(pc.yoff)
		g.X = uint16(pc.x0) // xmin;
		g.Y = uint16(pc.y0)
		g.Width = uint16(pc.x1 - pc.x0)
		g.Height = uint16(pc.y1 - pc.y0)
		g.XAdvance = int16(pc.xadvance)
	}

	// Regenerate kernings
	if len(variant.Kernings) > 0 {
		variant.Kernings = nil
	}
	kerning_count := C.stbtt_GetKerningTableLength(&lookup.Info)

	if kerning_count > 0 {
		variant.Kernings = make([]*metadata.FontKerning, kerning_count)
		kerning_table := make([]*C.stbtt_kerningentry, kerning_count)

		entry_count := C.stbtt_GetKerningTable(&lookup.Info, kerning_table[0], kerning_count)
		if entry_count != kerning_count {
			err := fmt.Errorf("kerning entry count mismatch: %d->%d", entry_count, kerning_count)
			return err
		}

		for i := 0; i < int(kerning_count); i++ {
			k := variant.Kernings[i]
			k.Codepoint0 = int32(kerning_table[i].glyph1)
			k.Codepoint1 = int32(kerning_table[i].glyph2)
			k.Amount = int16(kerning_table[i].advance)
		}
	} else {
		variant.Kernings = nil
	}

	return nil
}

func (fs *FontSystem) VerifySystemFontSizeVariant(lookup *SystemFontLookup, variant *metadata.FontData, text string) error {
	internal_data := variant.InternalData.(*SystemFontVariantData)

	char_length := uint32(len(text))
	added_codepoint_count := 0
	for i := uint32(0); i < char_length; {
		codepoint, advance, err := metadata.BytesToCodepoint(text, i)
		if err != nil {
			core.LogError("bytes_to_codepoint failed to get codepoint.")
			core.LogError(err.Error())
			i++
			continue
		} else {
			// Check if the codepoint is already contained. Note that ascii
			// codepoints are always included, so checking those may be skipped.
			i += uint32(advance)
			if codepoint < 128 {
				continue
			}
			codepoint_count := len(internal_data.Codepoints)
			found := false
			for j := 95; j < codepoint_count; j++ {
				if internal_data.Codepoints[j] == codepoint {
					found = true
					break
				}
			}
			if !found {
				internal_data.Codepoints = append(internal_data.Codepoints, codepoint)
				added_codepoint_count++
			}
		}
	}

	// If codepoints were added, rebuild the atlas.
	if added_codepoint_count > 0 {
		return fs.RebuildSystemFontVariantAtlas(lookup, variant)
	}

	// Otherwise, proceed as normal.
	return nil
}
