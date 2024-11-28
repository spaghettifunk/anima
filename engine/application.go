package engine

import (
	"github.com/spaghettifunk/anima/engine/core"
	"github.com/spaghettifunk/anima/engine/renderer/metadata"
)

type ApplicationConfig struct {
	// Window starting position x axis, if applicable.
	StartPosX uint32
	// Window starting position y axis, if applicable.
	StartPosY uint32
	// Window starting width, if applicable.
	StartWidth uint32
	// Window starting height, if applicable.
	StartHeight uint32
	// The application name used in windowing, if applicable.
	Name              string
	LogLevel          core.LogLevel
	RenderViewConfigs []*metadata.RenderViewConfig
}
