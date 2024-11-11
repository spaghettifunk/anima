package views

import "github.com/spaghettifunk/anima/engine/renderer/metadata"

type RenderViewUI struct{}

func (vu *RenderViewUI) OnCreateRenderView(uniforms map[string]uint16) bool {
	return false
}

func (vu *RenderViewUI) OnDestroyRenderView() error {
	return nil
}

func (vu *RenderViewUI) OnResizeRenderView(width, height uint32) {

}

func (vu *RenderViewUI) OnBuildPacketRenderView(data interface{}) (*metadata.RenderViewPacket, error) {
	return nil, nil
}

func (vu *RenderViewUI) OnDestroyPacketRenderView(packet *metadata.RenderViewPacket) {

}

func (vu *RenderViewUI) OnRenderRenderView(view *metadata.RenderView, packet *metadata.RenderViewPacket, frame_number, render_target_index uint64) bool {
	return false
}
