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

	return false
}

func (vu *RenderViewUI) OnDestroy() error {
	return nil
}

func (vu *RenderViewUI) OnResize(width, height uint32) {
	vu.ProjectionMatrix = math.NewMat4Orthographic(0.0, float32(width), float32(height), 0.0, vu.NearClip, vu.FarClip)
}

func (vu *RenderViewUI) OnBuildPacket(data interface{}) (*metadata.RenderViewPacket, error) {
	// packet_data := data.(*metadata.UIPacketData);

	// out_packet->geometries = darray_create(geometry_render_data);
	// out_packet->view = self;

	// // Set matrices, etc.
	// out_packet->projection_matrix = internal_data->projection_matrix;
	// out_packet->view_matrix = internal_data->view_matrix;

	// // TODO: temp set extended data to the test text objects for now.
	// out_packet->extended_data = data;

	// // Obtain all geometries from the current scene.
	// // Iterate all meshes and add them to the packet's geometries collection
	// for (u32 i = 0; i < packet_data->mesh_data.mesh_count; ++i) {
	//     mesh* m = packet_data->mesh_data.meshes[i];
	//     for (u32 j = 0; j < m->geometry_count; ++j) {
	//         geometry_render_data render_data;
	//         render_data.geometry = m->geometries[j];
	//         render_data.model = transform_get_world(&m->transform);
	//         darray_push(out_packet->geometries, render_data);
	//         out_packet->geometry_count++;
	//     }
	// }

	// return true;
	return nil, nil
}

func (vu *RenderViewUI) OnDestroyPacket(packet *metadata.RenderViewPacket) {

}

func (vu *RenderViewUI) OnRender(packet *metadata.RenderViewPacket, frame_number, render_target_index uint64) bool {
	return false
}

func (vu *RenderViewUI) RegenerateAttachmentTarget(passIndex uint32, attachment *metadata.RenderTargetAttachment) bool {
	return true
}
