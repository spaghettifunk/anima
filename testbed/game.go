package testbed

import (
	"fmt"

	"github.com/spaghettifunk/anima/engine"
	"github.com/spaghettifunk/anima/engine/core"
	"github.com/spaghettifunk/anima/engine/math"
	"github.com/spaghettifunk/anima/engine/renderer/components"
	"github.com/spaghettifunk/anima/engine/renderer/metadata"
)

type TestGame struct {
	*engine.Game
}

type gameState struct {
	DeltaTime   uint32
	WorldCamera *components.Camera

	width  uint32
	height uint32

	// Temporary for testing
	skybox       *metadata.Skybox
	meshes       []*metadata.Mesh
	carMesh      *metadata.Mesh
	sponzaMesh   *metadata.Mesh
	modelsLoaded bool

	uiMeshes    []*metadata.Mesh
	testText    *metadata.UIText
	testSysText *metadata.UIText

	hoveredObjectID uint32
}

func NewTestGame() (*TestGame, error) {
	tg := &TestGame{
		Game: &engine.Game{
			ApplicationConfig: &engine.ApplicationConfig{
				StartPosX:   100,
				StartPosY:   100,
				StartWidth:  1280,
				StartHeight: 720,
				Name:        "Anima Game Engine",
				LogLevel:    core.DebugLevel,
			},
			State: &gameState{
				skybox: &metadata.Skybox{
					Cubemap:  &metadata.TextureMap{},
					Geometry: &metadata.Geometry{},
				},
				modelsLoaded: false,
			},
		},
	}

	tg.FnBoot = tg.Boot
	tg.FnInitialize = tg.Initialize
	tg.FnUpdate = tg.Update
	tg.FnRender = tg.Render
	tg.FnOnResize = tg.OnResize
	tg.FnShutdown = tg.Shutdown

	return tg, nil
}

func (g *TestGame) Boot() error {
	core.LogInfo("booting testbed...")

	config := g.ApplicationConfig

	// TODO: temp
	// Create test ui text objects
	// text, err := g.SystemManager.FontSystem.UITextCreate(metadata.UI_TEXT_TYPE_BITMAP, "Ubuntu Mono 21px", 21, "Some test text 123,\n\tyo!")
	// if err != nil {
	// 	core.LogError("failed to load basic ui bitmap text")
	// 	return err
	// }
	// e.testText = text

	// // Move debug text to new bottom of screen.
	// g.SystemManager.FontSystem.UITextSetPosition(e.testText, math.NewVec3(20, float32(e.height-75), 0))

	// text, err = g.SystemManager.FontSystem.UITextCreate(metadata.UI_TEXT_TYPE_SYSTEM, "Noto Sans CJK JP", 31, "Some system text 123, \n\tyo!\n\n\tこんにちは 한")
	// if err != nil {
	// 	core.LogError("failed to load basic ui system text")
	// 	return err
	// }
	// e.testSysText = text
	// g.SystemManager.FontSystem.UITextSetPosition(e.testSysText, math.NewVec3(50, 250, 0))

	// text, err = g.SystemManager.FontSystem.UITextCreate(metadata.UI_TEXT_TYPE_SYSTEM, "Noto Sans CJK JP", 31, "Some system text 123, \n\tyo!\n\n\tこんにちは 한")
	// if err != nil {
	// 	core.LogError("failed to load basic ui system text")
	// 	return err
	// }
	// e.testSysText = text
	// g.SystemManager.FontSystem.UITextSetPosition(e.testSysText, math.NewVec3(50, 200, 0))

	// Configure render views. TODO: read from file?
	if err := g.configureRenderViews(config); err != nil {
		core.LogError("failed to configure renderer views. Aborting application")
		return err
	}

	return nil
}

func (g *TestGame) Initialize() error {
	core.LogDebug("TestGame Initialize fn....")

	if g.SystemManager == nil {
		return fmt.Errorf("the engine is not yet initialized with all the system managers ")
	}

	state := g.State.(*gameState)
	state.modelsLoaded = false

	state.WorldCamera = g.SystemManager.CameraSystem.GetDefault()
	state.WorldCamera.SetPosition(math.NewVec3(10.5, 5.0, 9.5))

	// Skybox
	state.skybox.Cubemap.FilterMagnify = metadata.TextureFilterModeLinear
	state.skybox.Cubemap.FilterMinify = metadata.TextureFilterModeLinear
	state.skybox.Cubemap.RepeatU = metadata.TextureRepeatClampToEdge
	state.skybox.Cubemap.RepeatV = metadata.TextureRepeatClampToEdge
	state.skybox.Cubemap.RepeatW = metadata.TextureRepeatClampToEdge
	state.skybox.Cubemap.Use = metadata.TextureUseMapCubemap
	if err := g.SystemManager.RendererSystem.TextureMapAcquireResources(state.skybox.Cubemap); err != nil {
		core.LogError("unable to acquire resources for cube map texture")
		return err
	}

	t, err := g.SystemManager.TextureSystem.AquireCube("skybox", true)
	if err != nil {
		return err
	}
	state.skybox.Cubemap.Texture = t
	skyboxCubeConfig, err := g.SystemManager.GeometrySystem.GenerateCubeConfig(10.0, 10.0, 10.0, 1.0, 1.0, "skybox_cube", "")
	if err != nil {
		return err
	}

	// Clear out the material name.
	skyboxCubeConfig.MaterialName = ""
	geom, err := g.SystemManager.GeometrySystem.AcquireFromConfig(skyboxCubeConfig, true)
	if err != nil {
		return err
	}
	state.skybox.Geometry = geom
	state.skybox.RenderFrameNumber = metadata.InvalidIDUint64
	skyboxShader, err := g.SystemManager.ShaderSystem.GetShader("Shader.Builtin.Skybox")
	if err != nil {
		return err
	}
	maps := []*metadata.TextureMap{state.skybox.Cubemap}
	state.skybox.InstanceID, err = g.SystemManager.RendererSystem.ShaderAcquireInstanceResources(skyboxShader, maps)
	if err != nil {
		return err
	}

	// World meshes
	// Invalidate all meshes.
	state.meshes = make([]*metadata.Mesh, 10)
	state.uiMeshes = make([]*metadata.Mesh, 10)

	for i := 0; i < 10; i++ {
		if state.meshes[i] == nil {
			state.meshes[i] = &metadata.Mesh{
				Generation: metadata.InvalidIDUint8,
			}
		}
		if state.uiMeshes[i] == nil {
			state.uiMeshes[i] = &metadata.Mesh{
				Generation: metadata.InvalidIDUint8,
			}
		}
	}

	meshCount := 0

	// Load up a cube configuration, and load geometry from it.
	cubeMesh1 := state.meshes[meshCount]
	cubeMesh1.GeometryCount = 1
	cubeMesh1.Geometries = make([]*metadata.Geometry, 1)
	gConfig, err := g.SystemManager.GeometrySystem.GenerateCubeConfig(10.0, 10.0, 10.0, 1.0, 1.0, "test_cube", "test_material")
	if err != nil {
		return err
	}
	c, err := g.SystemManager.GeometrySystem.AcquireFromConfig(gConfig, true)
	if err != nil {
		return err
	}
	cubeMesh1.Geometries[0] = c
	cubeMesh1.Transform = math.TransformCreate()
	cubeMesh1.Generation = 0
	meshCount++

	// Clean up the allocations for the geometry config.
	g.SystemManager.GeometrySystem.ConfigDispose(gConfig)

	// A second cube
	cubeMesh2 := state.meshes[meshCount]
	cubeMesh2.GeometryCount = 1
	cubeMesh2.Geometries = make([]*metadata.Geometry, 1)
	gConfig, err = g.SystemManager.GeometrySystem.GenerateCubeConfig(5.0, 5.0, 5.0, 1.0, 1.0, "test_cube_2", "test_material")
	if err != nil {
		return err
	}
	c, err = g.SystemManager.GeometrySystem.AcquireFromConfig(gConfig, true)
	if err != nil {
		return err
	}
	cubeMesh2.Geometries[0] = c
	cubeMesh2.Transform = math.TransformFromPosition(math.NewVec3(10.0, 0.0, 1.0))
	// Set the first cube as the parent to the second.
	cubeMesh2.Transform.Parent = cubeMesh1.Transform
	cubeMesh2.Generation = 0
	meshCount++

	// Clean up the allocations for the geometry config.
	g.SystemManager.GeometrySystem.ConfigDispose(gConfig)

	// A third cube!
	cubeMesh3 := state.meshes[meshCount]
	cubeMesh3.GeometryCount = 1
	cubeMesh3.Geometries = make([]*metadata.Geometry, 1)
	gConfig, err = g.SystemManager.GeometrySystem.GenerateCubeConfig(2.0, 2.0, 2.0, 1.0, 1.0, "test_cube_3", "test_material")
	if err != nil {
		return err
	}
	c, err = g.SystemManager.GeometrySystem.AcquireFromConfig(gConfig, true)
	if err != nil {
		return err
	}
	cubeMesh3.Geometries[0] = c
	cubeMesh3.Transform = math.TransformFromPosition(math.NewVec3(5.0, 0.0, 1.0))
	// Set the second cube as the parent to the third.
	cubeMesh3.Transform.Parent = cubeMesh2.Transform
	cubeMesh3.Generation = 0
	meshCount++

	// Clean up the allocations for the geometry config.
	g.SystemManager.GeometrySystem.ConfigDispose(gConfig)

	state.carMesh = state.meshes[meshCount]
	state.carMesh.Transform = math.TransformFromPosition(math.NewVec3(15.0, 0.0, 1.0))
	meshCount++

	state.sponzaMesh = state.meshes[meshCount]
	state.sponzaMesh.Transform = math.TransformFromPositionRotationScale(math.NewVec3(15.0, 0.0, 1.0), math.NewQuatIdentity(), math.NewVec3(0.05, 0.05, 0.05))
	meshCount++

	core.EventRegister(core.EVENT_CODE_DEBUG0, g.gameOnDebugEvent)
	core.EventRegister(core.EVENT_CODE_DEBUG1, g.gameOnDebugEvent)
	core.EventRegister(core.EVENT_CODE_OBJECT_HOVER_ID_CHANGED, g.gameOnEvent)

	core.EventRegister(core.EVENT_CODE_KEY_PRESSED, g.gameOnKey)
	core.EventRegister(core.EVENT_CODE_KEY_RELEASED, g.gameOnKey)

	return nil
}

var tempMoveSpeed float32 = 50.0

func (g *TestGame) Update(deltaTime float64) error {
	state := g.State.(*gameState)

	// HACK: temp hack to move camera around.
	if core.InputIsKeyDown(core.KEY_A) || core.InputIsKeyDown(core.KEY_LEFT) {
		state.WorldCamera.Yaw(float32(1.0 * deltaTime))
	}

	if core.InputIsKeyDown(core.KEY_D) || core.InputIsKeyDown(core.KEY_RIGHT) {
		state.WorldCamera.Yaw(float32(-1.0 * deltaTime))
	}

	if core.InputIsKeyDown(core.KEY_UP) {
		state.WorldCamera.Yaw(float32(1.0 * deltaTime))
	}

	if core.InputIsKeyDown(core.KEY_DOWN) {
		state.WorldCamera.Yaw(float32(-1.0 * deltaTime))
	}

	if core.InputIsKeyDown(core.KEY_W) {
		state.WorldCamera.Yaw(tempMoveSpeed * float32(deltaTime))
	}

	if core.InputIsKeyDown(core.KEY_S) {
		state.WorldCamera.Yaw(tempMoveSpeed * float32(deltaTime))
	}

	if core.InputIsKeyDown(core.KEY_Q) {
		state.WorldCamera.Yaw(tempMoveSpeed * float32(deltaTime))
	}

	if core.InputIsKeyDown(core.KEY_E) {
		state.WorldCamera.Yaw(tempMoveSpeed * float32(deltaTime))
	}

	if core.InputIsKeyDown(core.KEY_SPACE) {
		state.WorldCamera.Yaw(tempMoveSpeed * float32(deltaTime))
	}

	if core.InputIsKeyDown(core.KEY_X) {
		state.WorldCamera.Yaw(tempMoveSpeed * float32(deltaTime))
	}

	// TODO: temp
	if core.InputIsKeyUp(core.KEY_P) && core.InputWasKeyDown(core.KEY_P) {
		core.LogDebug("Pos:[%.2f, %.2f, %.2f", state.WorldCamera.Position.X, state.WorldCamera.Position.Y, state.WorldCamera.Position.Z)
	}

	// RENDERER DEBUG FUNCTIONS
	if core.InputIsKeyUp(core.KEY_NUMPAD1) && core.InputWasKeyDown(core.KEY_NUMPAD1) {
		data := core.EventContext{
			Type: core.EVENT_CODE_SET_RENDER_MODE,
			Data: metadata.RENDERER_VIEW_MODE_LIGHTING,
		}
		core.EventFire(data)
	}

	if core.InputIsKeyUp(core.KEY_NUMPAD2) && core.InputWasKeyDown(core.KEY_NUMPAD2) {
		data := core.EventContext{
			Type: core.EVENT_CODE_SET_RENDER_MODE,
			Data: metadata.RENDERER_VIEW_MODE_NORMALS,
		}
		core.EventFire(data)
	}

	if core.InputIsKeyUp(core.KEY_NUMPAD0) && core.InputWasKeyDown(core.KEY_NUMPAD0) {
		data := core.EventContext{
			Type: core.EVENT_CODE_SET_RENDER_MODE,
			Data: metadata.RENDERER_VIEW_MODE_DEFAULT,
		}
		core.EventFire(data)
	}

	// Bind a key to load up some data.
	if core.InputIsKeyUp(core.KEY_L) && core.InputWasKeyDown(core.KEY_L) {
		data := core.EventContext{}
		core.EventFire(data)
	}

	// Perform a small rotation on the first mesh.
	rotation := math.NewQuatFromAxisAngle(math.NewVec3(0, 1, 0), float32(0.5*deltaTime), false)
	state.meshes[0].Transform.Rotate(rotation)
	// Perform a similar rotation on the second mesh, if it exists.
	state.meshes[1].Transform.Rotate(rotation)
	// Perform a similar rotation on the third mesh, if it exists.
	state.meshes[2].Transform.Rotate(rotation)

	// Update the bitmap text with camera position. NOTE: just using the default camera for now.
	worldCamera := g.SystemManager.CameraSystem.GetDefault()
	pos := worldCamera.GetPosition()
	rot := worldCamera.GetEulerRotation()

	// also track on current mouse state
	leftDown := core.InputIsButtonDown(core.BUTTON_LEFT)
	rightDown := core.InputIsButtonDown(core.BUTTON_RIGHT)
	mouseX, mouseY := core.InputGetMousePosition()

	// convert to NDC
	mouseXNDC := math.RangeConvertFloat32(float32(mouseX), 0, float32(state.width), -1, 1)
	mouseYNDC := math.RangeConvertFloat32(float32(mouseY), 0, float32(state.height), -1, 1)

	fps, frameTime := core.MetricsFrame()

	textBuffer := fmt.Sprintf(
		"FPS: %5.1f(%4.1fms) Pos=[%7.3f %7.3f %7.3f ] Rot=[%7.3f, %7.3f, %7.3f  ]\n"+
			"Mouse: X=%-5d Y=%-5d   L=%s R=%s   NDC: X=%.6f, Y=%.6f\n"+
			"Hovered: %s%d",
		fps,
		frameTime,
		pos.X, pos.Y, pos.Z,
		math.RadToDeg(rot.X), math.RadToDeg(rot.Y), math.RadToDeg(rot.Z),
		mouseX, mouseY,
		map[bool]string{true: "Y", false: "N"}[leftDown],
		map[bool]string{true: "Y", false: "N"}[rightDown],
		mouseXNDC,
		mouseYNDC,
		// FIXME: the two belows are hardcoded
		"none",
		0,
		// func() string {
		// 	if appState.hoveredObjectID == INVALID_ID {
		// 		return "none"
		// 	}
		// 	return ""
		// }(),
		// func() uint {
		// 	if appState.hoveredObjectID == INVALID_ID {
		// 		return 0
		// 	}
		// 	return appState.hoveredObjectID
		// }(),
	)

	core.LogInfo(textBuffer)

	return nil
}

func (g *TestGame) Render(packet *metadata.RenderPacket, deltaTime float64) error {
	state := g.State.(*gameState)

	packet.DeltaTime = deltaTime
	packet.ViewCount = 4
	packet.ViewPackets = make([]*metadata.RenderViewPacket, 4)

	// skybox
	skyboxPacketData := &metadata.SkyboxPacketData{
		Skybox: state.skybox,
	}
	rvp, err := g.SystemManager.RenderViewSystem.BuildPacket(g.SystemManager.RenderViewSystem.Get("skybox"), skyboxPacketData)
	if err != nil {
		core.LogError("Failed to build packet for view 'skybox'.")
		return err
	}
	packet.ViewPackets[0] = rvp

	// World
	meshCount := 0
	meshes := make([]*metadata.Mesh, 10)
	for i := 0; i < 10; i++ {
		if state.meshes[i].Generation != metadata.InvalidIDUint8 {
			meshes[meshCount] = state.meshes[i]
			meshCount++
		}
	}
	worldMeshData := &metadata.MeshPacketData{
		MeshCount: uint32(meshCount),
		Meshes:    meshes,
	}
	rvp, err = g.SystemManager.RenderViewSystem.BuildPacket(g.SystemManager.RenderViewSystem.Get("world"), worldMeshData)
	if err != nil {
		core.LogError("Failed to build packet for view 'world'.")
		return err
	}
	packet.ViewPackets[1] = rvp

	ui_packet := &metadata.UIPacketData{
		MeshData: &metadata.MeshPacketData{},
	}

	ui_mesh_count := uint32(0)
	ui_meshes := make([]*metadata.Mesh, 10)

	// TODO: flexible size array
	for i := 0; i < len(state.uiMeshes); i++ {
		if state.uiMeshes[i] != nil {
			if state.uiMeshes[i].Generation != metadata.InvalidIDUint8 {
				ui_meshes[ui_mesh_count] = state.uiMeshes[i]
				ui_mesh_count++
			}
		}
	}

	ui_packet.MeshData.MeshCount = ui_mesh_count
	ui_packet.MeshData.Meshes = ui_meshes
	ui_packet.Texts = make([]*metadata.UIText, 2)

	ui_packet.Texts[0] = state.testText
	ui_packet.Texts[1] = state.testSysText

	rvp, err = g.SystemManager.RenderViewSystem.BuildPacket(g.SystemManager.RenderViewSystem.Get("ui"), ui_packet)
	if err != nil {
		core.LogError("Failed to build packet for view 'ui'.")
		return err
	}
	packet.ViewPackets[2] = rvp

	// Pick uses both world and ui packet data.
	pick_packet := &metadata.PickPacketData{
		UIMeshData:    ui_packet.MeshData,
		WorldMeshData: worldMeshData,
		Texts:         ui_packet.Texts,
		TextCount:     uint32(len(ui_packet.Texts)),
	}

	rvp, err = g.SystemManager.RenderViewSystem.BuildPacket(g.SystemManager.RenderViewSystem.Get("pick"), pick_packet)
	if err != nil {
		core.LogError("Failed to build packet for view 'pick'.")
		return err
	}
	packet.ViewPackets[3] = rvp

	return nil
}

func (g *TestGame) OnResize(width uint32, height uint32) error {
	state := g.State.(*gameState)

	state.width = width
	state.height = height

	// TODO: temp
	// Move debug text to new bottom of screen.
	// ui_text_set_position(&state->test_text, vec3_create(20, state->height - 75, 0));
	// TODO: end temp

	return nil
}

func (g *TestGame) Shutdown() error {
	state := g.State.(*gameState)

	if state.skybox != nil {
		g.SystemManager.RendererSystem.TextureMapReleaseResources(state.skybox.Cubemap)
	}

	return nil
}

func (g *TestGame) configureRenderViews(config *engine.ApplicationConfig) error {
	// Load render views
	// Skybox view
	skybox_config := &metadata.RenderViewConfig{
		RenderViewType:   metadata.RENDERER_VIEW_KNOWN_TYPE_SKYBOX,
		Width:            0,
		Height:           0,
		Name:             "skybox",
		ViewMatrixSource: metadata.RENDER_VIEW_VIEW_MATRIX_SOURCE_SCENE_CAMERA,
		PassCount:        1,
		PassConfigs: []*metadata.RenderPassConfig{
			{
				Name:        "Renderpass.Builtin.Skybox",
				RenderArea:  math.NewVec4(0, 0, 1280, 720), // Default render area resolution
				ClearColour: math.NewVec4(0.0, 0.0, 0.2, 1.0),
				ClearFlags:  metadata.RENDERPASS_CLEAR_COLOUR_BUFFER_FLAG,
				Depth:       1.0,
				Stencil:     0,
				Target: &metadata.RenderTargetConfig{
					Attachments: []*metadata.RenderTargetAttachmentConfig{
						{
							RenderTargetAttachmentType: metadata.RENDER_TARGET_ATTACHMENT_TYPE_COLOUR,
							Source:                     metadata.RENDER_TARGET_ATTACHMENT_SOURCE_DEFAULT,
							LoadOperation:              metadata.RENDER_TARGET_ATTACHMENT_LOAD_OPERATION_DONT_CARE,
							StoreOperation:             metadata.RENDER_TARGET_ATTACHMENT_STORE_OPERATION_STORE,
							PresentAfter:               false,
						},
					},
				},
				RenderTargetCount: g.SystemManager.RendererSystem.GetWindowAttachmentCount(),
			},
		},
	}

	config.RenderViewConfigs = append(config.RenderViewConfigs, skybox_config)

	// World view.
	world_view_config := &metadata.RenderViewConfig{
		RenderViewType:   metadata.RENDERER_VIEW_KNOWN_TYPE_WORLD,
		Width:            0,
		Height:           0,
		Name:             "world",
		ViewMatrixSource: metadata.RENDER_VIEW_VIEW_MATRIX_SOURCE_SCENE_CAMERA,
		PassCount:        1,
		PassConfigs: []*metadata.RenderPassConfig{
			{
				Name:        "Renderpass.Builtin.World",
				RenderArea:  math.NewVec4(0, 0, 1280, 720), // Default render area resolution
				ClearColour: math.NewVec4(0.0, 0.0, 0.2, 1.0),
				ClearFlags:  metadata.RENDERPASS_CLEAR_DEPTH_BUFFER_FLAG | metadata.RENDERPASS_CLEAR_STENCIL_BUFFER_FLAG,
				Depth:       1.0,
				Stencil:     0,
				Target: &metadata.RenderTargetConfig{
					Attachments: []*metadata.RenderTargetAttachmentConfig{
						// Colour attachment
						{
							RenderTargetAttachmentType: metadata.RENDER_TARGET_ATTACHMENT_TYPE_COLOUR,
							Source:                     metadata.RENDER_TARGET_ATTACHMENT_SOURCE_DEFAULT,
							LoadOperation:              metadata.RENDER_TARGET_ATTACHMENT_LOAD_OPERATION_LOAD,
							StoreOperation:             metadata.RENDER_TARGET_ATTACHMENT_STORE_OPERATION_STORE,
							PresentAfter:               false,
						},
						{ // Depth attachment
							RenderTargetAttachmentType: metadata.RENDER_TARGET_ATTACHMENT_TYPE_DEPTH,
							Source:                     metadata.RENDER_TARGET_ATTACHMENT_SOURCE_DEFAULT,
							LoadOperation:              metadata.RENDER_TARGET_ATTACHMENT_LOAD_OPERATION_DONT_CARE,
							StoreOperation:             metadata.RENDER_TARGET_ATTACHMENT_STORE_OPERATION_STORE,
							PresentAfter:               false,
						},
					},
				},
				RenderTargetCount:  g.SystemManager.RendererSystem.GetWindowAttachmentCount(),
			},
		},
	}

	config.RenderViewConfigs = append(config.RenderViewConfigs, world_view_config)

	// UI view
	ui_view_config := &metadata.RenderViewConfig{
		RenderViewType:   metadata.RENDERER_VIEW_KNOWN_TYPE_UI,
		Width:            0,
		Height:           0,
		Name:             "ui",
		ViewMatrixSource: metadata.RENDER_VIEW_VIEW_MATRIX_SOURCE_SCENE_CAMERA,
		PassCount:        1,
		PassConfigs: []*metadata.RenderPassConfig{
			{
				Name:        "Renderpass.Builtin.UI",
				RenderArea:  math.NewVec4(0, 0, 1280, 720),
				ClearColour: math.NewVec4(0.0, 0.0, 0.2, 1.0),
				ClearFlags:  metadata.RENDERPASS_CLEAR_NONE_FLAG,
				Depth:       1.0,
				Stencil:     0,
				Target: &metadata.RenderTargetConfig{
					Attachments: []*metadata.RenderTargetAttachmentConfig{
						{
							// Colour attachment.
							RenderTargetAttachmentType: metadata.RENDER_TARGET_ATTACHMENT_TYPE_COLOUR,
							Source:                     metadata.RENDER_TARGET_ATTACHMENT_SOURCE_DEFAULT,
							LoadOperation:              metadata.RENDER_TARGET_ATTACHMENT_LOAD_OPERATION_LOAD,
							StoreOperation:             metadata.RENDER_TARGET_ATTACHMENT_STORE_OPERATION_STORE,
							PresentAfter:               true,
						},
					},
				},
				RenderTargetCount:  g.SystemManager.RendererSystem.GetWindowAttachmentCount(),
			},
		},
	}

	config.RenderViewConfigs = append(config.RenderViewConfigs, ui_view_config)

	// Pick pass.
	pick_view_config := &metadata.RenderViewConfig{
		RenderViewType:   metadata.RENDERER_VIEW_KNOWN_TYPE_PICK,
		Width:            0,
		Height:           0,
		Name:             "pick",
		ViewMatrixSource: metadata.RENDER_VIEW_VIEW_MATRIX_SOURCE_SCENE_CAMERA,
		PassCount:        2,
		PassConfigs: []*metadata.RenderPassConfig{
			{
				// World pass
				Name:        "Renderpass.Builtin.WorldPick",
				RenderArea:  math.NewVec4(0, 0, 1280, 720),
				ClearColour: math.NewVec4(1.0, 1.0, 1.0, 1.0), // HACK: clearing to white for better visibility// TODO: Clear to black, as 0 is invalid id,
				ClearFlags:  metadata.RENDERPASS_CLEAR_COLOUR_BUFFER_FLAG | metadata.RENDERPASS_CLEAR_DEPTH_BUFFER_FLAG,
				Depth:       1.0,
				Stencil:     0,
				Target: &metadata.RenderTargetConfig{
					Attachments: []*metadata.RenderTargetAttachmentConfig{
						{
							RenderTargetAttachmentType: metadata.RENDER_TARGET_ATTACHMENT_TYPE_COLOUR,
							Source:                     metadata.RENDER_TARGET_ATTACHMENT_SOURCE_VIEW, // Obtain the attachment from the view,
							LoadOperation:              metadata.RENDER_TARGET_ATTACHMENT_LOAD_OPERATION_DONT_CARE,
							StoreOperation:             metadata.RENDER_TARGET_ATTACHMENT_STORE_OPERATION_STORE,
							PresentAfter:               false,
						},
						{
							RenderTargetAttachmentType: metadata.RENDER_TARGET_ATTACHMENT_TYPE_DEPTH,
							Source:                     metadata.RENDER_TARGET_ATTACHMENT_SOURCE_VIEW, // Obtain the attachment from the view,
							LoadOperation:              metadata.RENDER_TARGET_ATTACHMENT_LOAD_OPERATION_DONT_CARE,
							StoreOperation:             metadata.RENDER_TARGET_ATTACHMENT_STORE_OPERATION_STORE,
							PresentAfter:               false,
						},
					},
				},
				RenderTargetCount: 1,
			},
			{
				Name:        "Renderpass.Builtin.UIPick",
				RenderArea:  math.NewVec4(0, 0, 1280, 720),
				ClearColour: math.NewVec4(1.0, 1.0, 1.0, 1.0),
				ClearFlags:  metadata.RENDERPASS_CLEAR_NONE_FLAG,
				Depth:       1.0,
				Stencil:     0,
				Target: &metadata.RenderTargetConfig{
					Attachments: []*metadata.RenderTargetAttachmentConfig{
						{
							RenderTargetAttachmentType: metadata.RENDER_TARGET_ATTACHMENT_TYPE_COLOUR,
							// Obtain the attachment from the view.
							Source:        metadata.RENDER_TARGET_ATTACHMENT_SOURCE_VIEW,
							LoadOperation: metadata.RENDER_TARGET_ATTACHMENT_LOAD_OPERATION_LOAD,
							// Need to store it so it can be sampled afterward.
							StoreOperation: metadata.RENDER_TARGET_ATTACHMENT_STORE_OPERATION_STORE,
							PresentAfter:   false,
						},
					},
				},
				RenderTargetCount: 1, // No triple buffering this
			},
		},
	}

	config.RenderViewConfigs = append(config.RenderViewConfigs, pick_view_config)

	return nil
}

func (g *TestGame) gameOnEvent(context core.EventContext) {
	state := g.State.(*gameState)
	switch context.Type {
	case core.EVENT_CODE_OBJECT_HOVER_ID_CHANGED:
		{
			state.hoveredObjectID = context.Data.(uint32)
		}
	}
}

func (g *TestGame) gameOnDebugEvent(data core.EventContext) {
	state := g.State.(*gameState)

	if data.Type == core.EVENT_CODE_DEBUG0 {
		names := []string{
			"cobblestone",
			"paving",
			"paving2"}
		choice := int8(2)

		// Save off the old names.
		old_name := names[choice]

		choice++
		choice %= 3

		// Just swap out the material on the first mesh if it exists.
		geom := state.meshes[0].Geometries[0]
		if geom != nil {
			// Acquire the new material.
			m, err := g.SystemManager.MaterialSystem.Acquire(names[choice])
			if err != nil {
				core.LogError("failed to retrieve material with name %s", names[choice])
				return
			}
			geom.Material = m
			if geom.Material == nil {
				core.LogWarn("event_on_debug_event no material found! Using default material")
				geom.Material = g.SystemManager.MaterialSystem.GetDefault()
			}

			// Release the old diffuse material.
			g.SystemManager.MaterialSystem.Release(old_name)
		}
	} else if data.Type == core.EVENT_CODE_DEBUG1 {
		if !state.modelsLoaded {
			core.LogDebug("loading models...")
			state.modelsLoaded = true
			if g.SystemManager.MeshLoaderSystem.LoadFromResource("falcon", state.carMesh) == false {
				core.LogError("failed to load falcon mesh!")
			}
			if g.SystemManager.MeshLoaderSystem.LoadFromResource("sponza", state.sponzaMesh) == false {
				core.LogError("Failed to load sponza mesh!")
			}
		}
	}
}

func (g *TestGame) gameOnKey(context core.EventContext) {
	if context.Type == core.EVENT_CODE_KEY_PRESSED {
		key_code := context.Data
		if key_code == core.KEY_ESCAPE {
			// NOTE: Technically firing an event to itself, but there may be other listeners.
			core.EventFire(core.EventContext{
				Type: core.EVENT_CODE_APPLICATION_QUIT,
			})
		} else if key_code == core.KEY_A {
			// Example on checking for a key
			core.LogDebug("Explicit - A key pressed!")
		} else {
			core.LogDebug("'%s' key pressed in window.", key_code)
		}
	} else if context.Type == core.EVENT_CODE_KEY_RELEASED {
		key_code := context.Data
		if key_code == core.KEY_B {
			// Example on checking for a key
			core.LogDebug("Explicit - B key released!")
		} else {
			core.LogDebug("'%s' key released in window.", key_code)
		}
	}
}
