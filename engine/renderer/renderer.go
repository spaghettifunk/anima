package renderer

type Renderer interface {
	Create() error
	DrawImage() error
	Clean() error
}

type RendererType uint8

const (
	Vulkan RendererType = iota
	DirectX
	Metal
	OpenGL
)
