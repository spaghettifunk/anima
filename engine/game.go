package engine

type Game struct {
	ApplicationConfig *ApplicationConfig
	State             interface{}
	// pointers to functions
	FnInitialize Initialize
	FnUpdate     Update
	FnRender     Render
	FnOnResize   OnResize
}

type Initialize func() error
type Update func(deltaTime float32) error
type Render func(deltaTime float32) error
type OnResize func(width uint32, height uint32) error
