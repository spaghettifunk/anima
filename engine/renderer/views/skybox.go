package views

import (
	"fmt"

	"github.com/spaghettifunk/anima/engine/math"
	"github.com/spaghettifunk/anima/engine/renderer/components"
	"github.com/spaghettifunk/anima/engine/renderer/metadata"
)

type RenderViewSkybox struct {
	ShaderID         uint32
	FOV              float32
	NearClip         float32
	FarClip          float32
	ProjectionMatrix math.Mat4
	WorldCamera      *components.Camera
	// uniform locations
	ProjectionLocation uint16
	ViewLocation       uint16
	CubeMapLocation    uint16
	// Shader
	Shader *metadata.Shader
}

func NewRenderViewSkybox(shader *metadata.Shader, camera *components.Camera) *RenderViewSkybox {
	return &RenderViewSkybox{
		ShaderID:    shader.ID,
		Shader:      shader,
		WorldCamera: camera,
	}
}

func (vs *RenderViewSkybox) OnCreateRenderView(uniforms map[string]uint16) bool {
	vs.ProjectionLocation = uniforms["projection"]
	vs.ViewLocation = uniforms["view"]
	vs.CubeMapLocation = uniforms["cube_texture"]

	// TODO: Set from configuration.
	vs.NearClip = 0.1
	vs.FarClip = 1000.0
	vs.FOV = math.DegToRad(45.0)

	// Default
	vs.ProjectionMatrix = math.NewMat4Perspective(vs.FOV, 1280/720.0, vs.NearClip, vs.FarClip)
	return true
}

func (vs *RenderViewSkybox) OnDestroyRenderView() error {
	return nil
}

func (vs *RenderViewSkybox) OnResizeRenderView(width, height uint32) {
	aspect := width / height
	vs.ProjectionMatrix = math.NewMat4Perspective(vs.FOV, float32(aspect), vs.NearClip, vs.FarClip)
}

func (vs *RenderViewSkybox) OnBuildPacketRenderView(data interface{}) (*metadata.RenderViewPacket, error) {
	if data == nil {
		err := fmt.Errorf("render_view_skybox_on_build_packet requires valid pointer to view, packet, and data")
		return nil, err
	}

	out_packet := &metadata.RenderViewPacket{}
	skybox_data := data.(*metadata.SkyboxPacketData)

	// Set matrices, etc.
	out_packet.ProjectionMatrix = vs.ProjectionMatrix
	out_packet.ViewMatrix = vs.WorldCamera.GetView()
	out_packet.ViewPosition = vs.WorldCamera.GetPosition()

	// Just set the extended data to the skybox data
	out_packet.ExtendedData = skybox_data
	return out_packet, nil
}

func (vs *RenderViewSkybox) OnDestroyPacketRenderView(packet *metadata.RenderViewPacket) {
}

func (vs *RenderViewSkybox) OnRenderRenderView(view *metadata.RenderView, packet *metadata.RenderViewPacket, frame_number, render_target_index uint64) bool {
	return true
}
