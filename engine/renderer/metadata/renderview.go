package metadata

import (
	"github.com/spaghettifunk/anima/engine/core"
	"github.com/spaghettifunk/anima/engine/math"
	"github.com/spaghettifunk/anima/engine/renderer/components"
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
	Shader *Shader
}

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

	Renderpass *RenderPass
	Shader     *Shader
}

type RenderViewPick struct {
	UIShaderInfo    *PickShaderInfo
	WorldShaderInfo *PickShaderInfo

	// Used as the colour attachment for both renderpasses.
	ColourTargetAttachmentTexture *Texture
	// The depth attachment.
	DepthTargetAttachmentTexture *Texture

	InstanceCount   int32
	InstanceUpdated []bool

	MouseX int16
	MouseY int16

	WorldCamera *components.Camera
	// u32 render_mode;
}

func (vp *RenderViewPick) OnMouseMoved(event_data core.EventContext) {
	if event_data.Type == core.EVENT_CODE_MOUSE_MOVED {
		// Update position and regenerate the projection matrix.
		x := event_data.Data.(*core.MouseEvent).PosX
		y := event_data.Data.(*core.MouseEvent).PosY

		vp.MouseX = int16(x)
		vp.MouseY = int16(y)
	}
}

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
	Shader                *Shader
}

type RenderViewWorld struct {
	ShaderID         uint32
	FOV              float32
	NearClip         float32
	FarClip          float32
	ProjectionMatrix math.Mat4
	WorldCamera      *components.Camera

	AmbientColour math.Vec4
	RenderMode    RendererDebugViewMode

	// Shader
	Shader *Shader
}

type GeometryDistance struct {
	GeometryRenderData *GeometryRenderData
	Distance           float32
}

func (vw *RenderViewWorld) OnSetRenderMode(context core.EventContext) {
	switch context.Type {
	case core.EVENT_CODE_SET_RENDER_MODE:
		{
			mode := context.Data.(RendererDebugViewMode)
			switch mode {
			default:
				fallthrough
			case RENDERER_VIEW_MODE_DEFAULT:
				core.LogDebug("renderer mode set to default")
				vw.RenderMode = RENDERER_VIEW_MODE_DEFAULT
			case RENDERER_VIEW_MODE_LIGHTING:
				core.LogDebug("renderer mode set to lighting")
				vw.RenderMode = RENDERER_VIEW_MODE_LIGHTING
			case RENDERER_VIEW_MODE_NORMALS:
				core.LogDebug("renderer mode set to normals")
				vw.RenderMode = RENDERER_VIEW_MODE_NORMALS
			}
		}
	}
}
