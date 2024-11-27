package views

import (
	"fmt"

	"github.com/spaghettifunk/anima/engine/core"
	"github.com/spaghettifunk/anima/engine/math"
	"github.com/spaghettifunk/anima/engine/renderer/components"
	"github.com/spaghettifunk/anima/engine/renderer/metadata"
)

type PickShaderInfo struct {
	FOV        float32
	NearClip   float32
	FarClip    float32
	Projection math.Mat4
	View       math.Mat4

	IDColorLocation    uint16
	ModelLocation      uint16
	ProjectionLocation uint16
	ViewLocation       uint16

	Renderpass *metadata.RenderPass
	Shader     *metadata.Shader
}

type RenderViewPick struct {
	UIShaderInfo    *PickShaderInfo
	WorldShaderInfo *PickShaderInfo

	// Used as the colour attachment for both renderpasses.
	ColourTargetAttachmentTexture *metadata.Texture
	// The depth attachment.
	DepthTargetAttachmentTexture *metadata.Texture

	InstanceCount   int32
	InstanceUpdated []bool

	MouseX int16
	MouseY int16

	View        *metadata.RenderView
	WorldCamera *components.Camera
	// u32 render_mode;
}

func NewRenderViewPick(view *metadata.RenderView, shaderUIPick *metadata.Shader, shaderWorldPick *metadata.Shader, c *components.Camera) *RenderViewPick {
	rp := &RenderViewPick{
		UIShaderInfo: &PickShaderInfo{
			Shader: shaderUIPick,
		},
		WorldShaderInfo: &PickShaderInfo{
			Shader: shaderWorldPick,
		},
		InstanceUpdated:               make([]bool, 1),
		ColourTargetAttachmentTexture: &metadata.Texture{},
		DepthTargetAttachmentTexture:  &metadata.Texture{},
		WorldCamera:                   c,
		View:                          view,
	}
	view.InternalData = rp

	return rp
}

func (vp *RenderViewPick) OnCreate(uniforms map[string]uint16) error {
	// NOTE: In this heavily-customized view, the exact number of passes is known, so
	// these index assumptions are fine.
	vp.UIShaderInfo.Renderpass = vp.View.Passes[0]
	vp.WorldShaderInfo.Renderpass = vp.View.Passes[1]

	// Extract uniform locations
	vp.UIShaderInfo.IDColorLocation = uniforms["ui_id_colour"]
	vp.UIShaderInfo.ModelLocation = uniforms["ui_model"]
	vp.UIShaderInfo.ProjectionLocation = uniforms["ui_projection"]
	vp.UIShaderInfo.ViewLocation = uniforms["ui_view"]

	// Default UI properties
	vp.UIShaderInfo.NearClip = -100.0
	vp.UIShaderInfo.FarClip = 100.0
	vp.UIShaderInfo.FOV = 0
	vp.UIShaderInfo.Projection = math.NewMat4Orthographic(0.0, 1280.0, 720.0, 0.0, vp.UIShaderInfo.NearClip, vp.UIShaderInfo.FarClip)
	vp.UIShaderInfo.View = math.NewMat4Identity()

	// Extract uniform locations.
	vp.WorldShaderInfo.IDColorLocation = uniforms["world_id_colour"]
	vp.WorldShaderInfo.ModelLocation = uniforms["world_model"]
	vp.WorldShaderInfo.ProjectionLocation = uniforms["world_projection"]
	vp.WorldShaderInfo.ViewLocation = uniforms["world_view"]

	// Default World properties
	vp.WorldShaderInfo.NearClip = 0.1
	vp.WorldShaderInfo.FarClip = 1000.0
	vp.WorldShaderInfo.FOV = math.DegToRad(45.0)
	vp.WorldShaderInfo.Projection = math.NewMat4Perspective(vp.WorldShaderInfo.FOV, 1280/720.0, vp.WorldShaderInfo.NearClip, vp.WorldShaderInfo.FarClip)
	vp.WorldShaderInfo.View = math.NewMat4Identity()

	vp.InstanceCount = 0

	core.EventRegister(core.EVENT_CODE_MOUSE_MOVED, vp.onMouseMoved)

	return nil
}

func (vp *RenderViewPick) OnDestroy() error {
	return nil
}

func (vp *RenderViewPick) OnResize(width, height uint32) {
	vp.View.Width = uint16(width)
	vp.View.Height = uint16(height)

	// UI
	vp.UIShaderInfo.Projection = math.NewMat4Orthographic(0.0, float32(width), float32(height), 0.0, vp.UIShaderInfo.NearClip, vp.UIShaderInfo.FarClip)

	// World
	aspect := float32(vp.View.Width / vp.View.Height)
	vp.WorldShaderInfo.Projection = math.NewMat4Perspective(vp.WorldShaderInfo.FOV, aspect, vp.WorldShaderInfo.NearClip, vp.WorldShaderInfo.FarClip)

	for i := 0; i < int(vp.View.RenderpassCount); i++ {
		vp.View.Passes[i].RenderArea.X = 0
		vp.View.Passes[i].RenderArea.Y = 0
		vp.View.Passes[i].RenderArea.Z = float32(width)
		vp.View.Passes[i].RenderArea.W = float32(height)
	}
}

func (vp *RenderViewPick) OnBuildPacket(data interface{}) (*metadata.RenderViewPacket, error) {
	if data == nil {
		err := fmt.Errorf("render_view_pick_on_build_packet requires valid pointer to view, packet, and data")
		return nil, err
	}

	packet_data := data.(*metadata.PickPacketData)

	out_packet := &metadata.RenderViewPacket{
		Geometries:   make([]*metadata.GeometryRenderData, 1),
		View:         vp.View,
		ExtendedData: packet_data,
	}

	// TODO: Get active camera.
	vp.WorldShaderInfo.View = vp.WorldCamera.GetView()

	// Set the pick packet data to extended data.
	packet_data.WorldGeometryCount = 0
	packet_data.UIGeometryCount = 0
	out_packet.ExtendedData = data

	highest_instance_id := uint32(0)
	// Iterate all meshes in world data.
	for i := 0; i < int(packet_data.WorldMeshData.MeshCount); i++ {
		m := packet_data.WorldMeshData.Meshes[i]
		for j := 0; j < int(m.GeometryCount); j++ {
			render_data := &metadata.GeometryRenderData{
				Geometry: m.Geometries[j],
				Model:    m.Transform.GetWorld(),
				UniqueID: m.UniqueID,
			}
			out_packet.Geometries = append(out_packet.Geometries, render_data)
			out_packet.GeometryCount++
			packet_data.WorldGeometryCount++
		}
		// Count all geometries as a single id.
		if m.UniqueID > highest_instance_id {
			highest_instance_id = m.UniqueID
		}
	}

	// Iterate all meshes in UI data.
	for i := 0; i < int(packet_data.UIMeshData.MeshCount); i++ {
		m := packet_data.UIMeshData.Meshes[i]
		for j := 0; j < int(m.GeometryCount); j++ {
			render_data := &metadata.GeometryRenderData{
				Geometry: m.Geometries[j],
				Model:    m.Transform.GetWorld(),
				UniqueID: m.UniqueID,
			}
			out_packet.Geometries = append(out_packet.Geometries, render_data)
			out_packet.GeometryCount++
			packet_data.UIGeometryCount++
		}
		// Count all geometries as a single id.
		if m.UniqueID > highest_instance_id {
			highest_instance_id = m.UniqueID
		}
	}

	// Count texts as well.
	// for i := 0; i < int(packet_data.TextCount); i++ {
	// 	if packet_data.Texts[i].UniqueID > highest_instance_id {
	// 		highest_instance_id = packet_data.Texts[i].UniqueID
	// 	}
	// }

	packet_data.RequiredInstanceCount = highest_instance_id + 1

	return out_packet, nil
}

func (vp *RenderViewPick) OnDestroyPacket(packet *metadata.RenderViewPacket) error {
	packet.Geometries = nil
	packet = nil
	return nil
}

func (vp *RenderViewPick) OnRender(packet *metadata.RenderViewPacket, frame_number, render_target_index uint64) error {
	return nil
}

func (vp *RenderViewPick) RegenerateAttachmentTarget(passIndex uint32, attachment *metadata.RenderTargetAttachment) error {
	return nil
}

func (vp *RenderViewPick) onMouseMoved(event_data core.EventContext) {
	if event_data.Type == core.EVENT_CODE_MOUSE_MOVED {
		// Update position and regenerate the projection matrix.
		x := event_data.Data.(*core.MouseEvent).PosX
		y := event_data.Data.(*core.MouseEvent).PosY

		vp.MouseX = int16(x)
		vp.MouseY = int16(y)
	}
}
