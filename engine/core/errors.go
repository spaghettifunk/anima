package core

import (
	"errors"
)

var (
	ErrSwapchainBooting = errors.New("swapchain resized or recreated, booting")
	ErrUnknown          = errors.New("unknown")
)
