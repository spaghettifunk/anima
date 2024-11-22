package views

import (
	mt "math"
	"sort"

	"github.com/spaghettifunk/anima/engine/core"
	"github.com/spaghettifunk/anima/engine/math"
	"github.com/spaghettifunk/anima/engine/renderer/components"
	"github.com/spaghettifunk/anima/engine/renderer/metadata"
)

type RenderViewWorld struct {
	ShaderID         uint32
	FOV              float32
	NearClip         float32
	FarClip          float32
	ProjectionMatrix math.Mat4
	WorldCamera      *components.Camera

	AmbientColour math.Vec4
	RenderMode    metadata.RendererDebugViewMode

	// Shader
	Shader *metadata.Shader
	View   *metadata.RenderView
}

type GeometryDistance struct {
	GeometryRenderData *metadata.GeometryRenderData
	Distance           float32
}

func NewRenderViewWorld(view *metadata.RenderView, shader *metadata.Shader, camera *components.Camera) *RenderViewWorld {
	rvw := &RenderViewWorld{
		ShaderID:    shader.ID,
		Shader:      shader,
		WorldCamera: camera,
		View:        view,
	}
	view.InternalData = rvw
	return rvw
}

func (vw *RenderViewWorld) renderViewOnEvent(context core.EventContext) {
	switch context.Type {
	case core.EVENT_CODE_SET_RENDER_MODE:
		{
			mode := context.Data.(metadata.RendererDebugViewMode)
			switch mode {
			default:
				fallthrough
			case metadata.RENDERER_VIEW_MODE_DEFAULT:
				core.LogDebug("renderer mode set to default")
				vw.RenderMode = metadata.RENDERER_VIEW_MODE_DEFAULT
			case metadata.RENDERER_VIEW_MODE_LIGHTING:
				core.LogDebug("renderer mode set to lighting")
				vw.RenderMode = metadata.RENDERER_VIEW_MODE_LIGHTING
			case metadata.RENDERER_VIEW_MODE_NORMALS:
				core.LogDebug("renderer mode set to normals")
				vw.RenderMode = metadata.RENDERER_VIEW_MODE_NORMALS
			}
		}
	}
}

func (vw *RenderViewWorld) OnCreate(uniforms map[string]uint16) bool {
	// Get either the custom shader override or the defined default.
	// TODO: Set from configuration.
	vw.NearClip = 0.1
	vw.FarClip = 1000.0
	vw.FOV = math.DegToRad(45.0)

	// Default
	vw.ProjectionMatrix = math.NewMat4Perspective(vw.FOV, 1280/720.0, vw.NearClip, vw.FarClip)

	// TODO: Obtain from scene
	vw.AmbientColour = math.NewVec4(0.25, 0.25, 0.25, 1.0)

	// Listen for mode changes.
	core.EventRegister(core.EVENT_CODE_SET_RENDER_MODE, vw.renderViewOnEvent)

	return true
}

func (vw *RenderViewWorld) OnDestroy() error {
	return nil
}

func (vw *RenderViewWorld) OnResize(width, height uint32) {
	aspect := float32(width / height)
	vw.ProjectionMatrix = math.NewMat4Perspective(vw.FOV, aspect, vw.NearClip, vw.FarClip)
}

func (vw *RenderViewWorld) OnBuildPacket(data interface{}) (*metadata.RenderViewPacket, error) {
	mesh_data := data.(*metadata.MeshPacketData)

	out_packet := &metadata.RenderViewPacket{
		Geometries:       []*metadata.GeometryRenderData{},
		ProjectionMatrix: vw.ProjectionMatrix,
		ViewMatrix:       vw.WorldCamera.GetView(),
		ViewPosition:     vw.WorldCamera.GetPosition(),
		AmbientColour:    vw.AmbientColour,
		View:             vw.View,
	}

	// Obtain all geometries from the current scene.
	geometry_distances := []*GeometryDistance{}

	for i := uint32(0); i < mesh_data.MeshCount; i++ {
		m := mesh_data.Meshes[i]
		model := m.Transform.GetWorld()

		for j := uint32(0); j < uint32(m.GeometryCount); j++ {
			render_data := &metadata.GeometryRenderData{
				Geometry: m.Geometries[j],
				Model:    model,
			}

			// TODO: Add something to material to check for transparency.
			if (m.Geometries[j].Material.DiffuseMap.Texture.Flags & metadata.TextureFlagBits(metadata.TextureFlagHasTransparency)) == 0 {
				// Only add meshes with _no_ transparency.
				out_packet.Geometries = append(out_packet.Geometries, render_data)
				out_packet.GeometryCount++
			} else {
				// For meshes _with_ transparency, add them to a separate list to be sorted by distance later.
				// Get the center, extract the global position from the model matrix and add it to the center,
				// then calculate the distance between it and the camera, and finally save it to a list to be sorted.
				// NOTE: This isn't perfect for translucent meshes that intersect, but is enough for our purposes now.
				center := render_data.Geometry.Center.Transform(model)
				distance := center.Distance(vw.WorldCamera.Position)

				gdist := &GeometryDistance{
					Distance:           float32(mt.Abs(float64(distance))),
					GeometryRenderData: render_data,
				}

				geometry_distances = append(geometry_distances, gdist)
			}
		}
	}

	// Sort the distances
	// FIXME: validate if it is the correct ordering
	sort.Slice(geometry_distances, func(i, j int) bool {
		return geometry_distances[i].Distance < geometry_distances[j].Distance
	})

	// Add them to the packet geometry.
	for i := 0; i < len(geometry_distances); i++ {
		out_packet.Geometries = append(out_packet.Geometries, geometry_distances[i].GeometryRenderData)
		out_packet.GeometryCount++
	}

	// Clean up.
	geometry_distances = nil

	return out_packet, nil
}

func (vw *RenderViewWorld) OnDestroyPacket(packet *metadata.RenderViewPacket) {
	packet.Geometries = nil
	packet = nil
}

func (vw *RenderViewWorld) OnRender(packet *metadata.RenderViewPacket, frame_number, render_target_index uint64) bool {
	return true
}

func (vw *RenderViewWorld) RegenerateAttachmentTarget(passIndex uint32, attachment *metadata.RenderTargetAttachment) bool {
	return true
}
