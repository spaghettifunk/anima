package metadata

import "github.com/spaghettifunk/anima/engine/math"

type SystemFontConfig struct {
	Name         string
	DefaultSize  uint16
	ResourceName string
}

type BitmapFontConfig struct {
	Name         string
	Size         uint16
	ResourceName string
}

type FontSystemConfig struct {
	DefaultSystemFontCount uint8
	SystemFontConfigs      []*SystemFontConfig
	DefaultBitmapFontCount uint8
	BitmapFontConfigs      []*BitmapFontConfig
	MaxSystemFontCount     uint8
	MaxBitmapFontCount     uint8
	AutoRelease            bool
}

type UITextType int

const (
	UI_TEXT_TYPE_BITMAP UITextType = iota
	UI_TEXT_TYPE_SYSTEM
)

type UIText struct {
	UniqueID          uint32
	InstanceID        uint32
	UITextType        UITextType
	Data              *FontData
	VertexBuffer      *RenderBuffer
	IndexBuffer       *RenderBuffer
	Text              string
	Transform         *math.Transform
	RenderFrameNumber uint64
}

type FontGlyph struct {
	Codepoint int32
	X         uint16
	Y         uint16
	Width     uint16
	Height    uint16
	XOffset   int16
	YOffset   int16
	XAdvance  int16
	PageID    uint8
}

type FontKerning struct {
	Codepoint0 int32
	Codepoint1 int32
	Amount     int16
}

type FontType int

const (
	FONT_TYPE_BITMAP FontType = iota
	FONT_TYPE_SYSTEM
)

type FontData struct {
	FontType         *FontType
	Face             string
	Size             uint32
	LineHeight       int32
	Baseline         int32
	AtlasSizeX       int32
	AtlasSizeY       int32
	Atlas            *TextureMap
	Glyphs           []*FontGlyph
	Kernings         []*FontKerning
	TabXAdvance      float32
	InternalDataSize uint32
	InternalData     interface{}
}

type BitmapFontPage struct {
	ID   int8
	Name string
}

type BitmapFontResourceData struct {
	Data  *FontData
	Pages []*BitmapFontPage
}

type SystemFontFace struct {
	Name string
}
