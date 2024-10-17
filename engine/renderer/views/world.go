package views

import "github.com/spaghettifunk/anima/engine/renderer/metadata"

func RenderViewWorldOnCreate() bool {
	return false
}

func RenderViewWorldOnDestroy() error {
	return nil
}

func RenderViewWorldOnResize(width, height uint32) {}

func RenderViewWorldOnBuildPacket(data interface{}) (*metadata.RenderViewPacket, error) {
	return nil, nil
}

func RenderViewWorldOnDestroyPacket(packet *metadata.RenderViewPacket) {

}

func RenderViewWorldOnRender(packet *metadata.RenderViewPacket, frame_number, render_target_index uint64) bool {
	return false
}
