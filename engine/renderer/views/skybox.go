package views

import "github.com/spaghettifunk/anima/engine/renderer/metadata"

func RenderViewSkyboxOnCreate() bool {
	return false
}

func RenderViewSkyboxOnDestroy() error {
	return nil
}

func RenderViewSkyboxOnResize(width, height uint32) {}

func RenderViewSkyboxOnBuildPacket(data interface{}) (*metadata.RenderViewPacket, error) {
	return nil, nil
}

func RenderViewSkyboxOnDestroyPacket(packet *metadata.RenderViewPacket) {

}

func RenderViewSkyboxOnRender(packet *metadata.RenderViewPacket, frame_number, render_target_index uint64) bool {
	return false
}
