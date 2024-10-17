package views

import "github.com/spaghettifunk/anima/engine/renderer/metadata"

func RenderViewUIOnCreate() bool {
	return false
}

func RenderViewUIOnDestroy() error {
	return nil
}

func RenderViewUIOnResize(width, height uint32) {}

func RenderViewUIOnBuildPacket(data interface{}) (*metadata.RenderViewPacket, error) {
	return nil, nil
}

func RenderViewUIOnDestroyPacket(packet *metadata.RenderViewPacket) {

}

func RenderViewUIOnRender(packet *metadata.RenderViewPacket, frame_number, render_target_index uint64) bool {
	return false
}
