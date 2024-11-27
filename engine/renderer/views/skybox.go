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
	View   *metadata.RenderView
}

func NewRenderViewSkybox(view *metadata.RenderView, shader *metadata.Shader, camera *components.Camera) *RenderViewSkybox {
	rs := &RenderViewSkybox{
		ShaderID:    shader.ID,
		Shader:      shader,
		WorldCamera: camera,
		View:        view,
	}
	view.InternalData = rs
	return rs
}

func (vs *RenderViewSkybox) OnCreate(uniforms map[string]uint16) error {
	vs.ProjectionLocation = uniforms["projection"]
	vs.ViewLocation = uniforms["view"]
	vs.CubeMapLocation = uniforms["cube_texture"]

	// TODO: Set from configuration.
	vs.NearClip = 0.1
	vs.FarClip = 1000.0
	vs.FOV = math.DegToRad(45.0)

	// Default
	vs.ProjectionMatrix = math.NewMat4Perspective(vs.FOV, 1280/720.0, vs.NearClip, vs.FarClip)
	return nil
}

func (vs *RenderViewSkybox) OnDestroy() error {
	return nil
}

func (vs *RenderViewSkybox) OnResize(width, height uint32) {
	aspect := width / height
	vs.ProjectionMatrix = math.NewMat4Perspective(vs.FOV, float32(aspect), vs.NearClip, vs.FarClip)
}

func (vs *RenderViewSkybox) OnBuildPacket(data interface{}) (*metadata.RenderViewPacket, error) {
	if data == nil {
		err := fmt.Errorf("render_view_skybox_on_build_packet requires valid pointer to view, packet, and data")
		return nil, err
	}

	skybox_data := data.(*metadata.SkyboxPacketData)

	// Set matrices, etc.
	out_packet := &metadata.RenderViewPacket{
		ProjectionMatrix: vs.ProjectionMatrix,
		ViewMatrix:       vs.WorldCamera.GetView(),
		ViewPosition:     vs.WorldCamera.GetPosition(),
		View:             vs.View,
		// Just set the extended data to the skybox data
		ExtendedData: skybox_data,
	}
	return out_packet, nil
}

func (vs *RenderViewSkybox) OnDestroyPacket(packet *metadata.RenderViewPacket) error {
	return nil
}

func (vs *RenderViewSkybox) OnRender(packet *metadata.RenderViewPacket, frame_number, render_target_index uint64) error {
	return nil
}

func (vs *RenderViewSkybox) RegenerateAttachmentTarget(passIndex uint32, attachment *metadata.RenderTargetAttachment) error {
	return nil
}
