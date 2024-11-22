package views

import (
	"github.com/spaghettifunk/anima/engine/math"
	"github.com/spaghettifunk/anima/engine/renderer/metadata"
)

type RenderViewUI struct {
	ShaderID              uint32
	FOV                   float32
	NearClip              float32
	FarClip               float32
	ProjectionMatrix      math.Mat4
	ViewMatrix            math.Mat4
	DiffuseMapLocation    uint16
	DiffuseColourLocation uint16
	ModelLocation         uint16
	Shader                *metadata.Shader
	View                  *metadata.RenderView
}

func NewRenderViewUI(view *metadata.RenderView, shader *metadata.Shader) *RenderViewUI {
	rui := &RenderViewUI{
		ShaderID: shader.ID,
		Shader:   shader,
		View:     view,
	}
	view.InternalData = rui
	return rui
}

func (vu *RenderViewUI) OnCreate(uniforms map[string]uint16) bool {
	vu.DiffuseMapLocation = uniforms["diffuse_texture"]
	vu.DiffuseColourLocation = uniforms["diffuse_colour"]
	vu.ModelLocation = uniforms["model"]
	// TODO: Set from configuration.
	vu.NearClip = -100.0
	vu.FarClip = 100.0

	// Default
	vu.ProjectionMatrix = math.NewMat4Orthographic(0.0, 1280.0, 720.0, 0.0, vu.NearClip, vu.FarClip)
	vu.ViewMatrix = math.NewMat4Identity()

	return true
}

func (vu *RenderViewUI) OnDestroy() error {
	vu.View.InternalData = nil
	return nil
}

func (vu *RenderViewUI) OnResize(width, height uint32) {
	vu.ProjectionMatrix = math.NewMat4Orthographic(0.0, float32(width), float32(height), 0.0, vu.NearClip, vu.FarClip)
}

func (vu *RenderViewUI) OnBuildPacket(data interface{}) (*metadata.RenderViewPacket, error) {
	packet_data := data.(*metadata.UIPacketData)

	out_packet := &metadata.RenderViewPacket{
		Geometries: []*metadata.GeometryRenderData{},
		View:       vu.View,
		// Set matrices, etc.
		ProjectionMatrix: vu.ProjectionMatrix,
		ViewMatrix:       vu.ViewMatrix,
		// TODO: temp set extended data to the test text objects for now.
		ExtendedData: packet_data,
	}

	// Obtain all geometries from the current scene.
	// Iterate all meshes and add them to the packet's geometries collection
	for i := 0; i < int(packet_data.MeshData.MeshCount); i++ {
		m := packet_data.MeshData.Meshes[i]
		for j := 0; j < int(m.GeometryCount); j++ {
			render_data := &metadata.GeometryRenderData{
				Geometry: m.Geometries[j],
				Model:    m.Transform.GetWorld(),
			}
			out_packet.Geometries = append(out_packet.Geometries, render_data)
			out_packet.GeometryCount++
		}
	}

	return out_packet, nil
}

func (vu *RenderViewUI) OnDestroyPacket(packet *metadata.RenderViewPacket) {

}

func (vu *RenderViewUI) OnRender(packet *metadata.RenderViewPacket, frame_number, render_target_index uint64) bool {
	return false
}

func (vu *RenderViewUI) RegenerateAttachmentTarget(passIndex uint32, attachment *metadata.RenderTargetAttachment) bool {
	return true
}
