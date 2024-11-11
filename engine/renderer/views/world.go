package views

import "github.com/spaghettifunk/anima/engine/renderer/metadata"

type RenderViewWorld struct{}

func (vw *RenderViewWorld) OnCreateRenderView(uniforms map[string]uint16) bool {
	return false
}

func (vw *RenderViewWorld) OnDestroyRenderView() error {
	return nil
}

func (vw *RenderViewWorld) OnResizeRenderView(width, height uint32) {

}

func (vw *RenderViewWorld) OnBuildPacketRenderView(data interface{}) (*metadata.RenderViewPacket, error) {
	return nil, nil
}

func (vw *RenderViewWorld) OnDestroyPacketRenderView(packet *metadata.RenderViewPacket) {

}

func (vw *RenderViewWorld) OnRenderRenderView(view *metadata.RenderView, packet *metadata.RenderViewPacket, frame_number, render_target_index uint64) bool {
	return false
}
