package engine

import (
	"fmt"
	"os"

	"github.com/spaghettifunk/anima/engine/assets"
	"github.com/spaghettifunk/anima/engine/core"
	"github.com/spaghettifunk/anima/engine/math"
	"github.com/spaghettifunk/anima/engine/platform"
	"github.com/spaghettifunk/anima/engine/renderer/metadata"
	"github.com/spaghettifunk/anima/engine/systems"
)

type Stage uint8

const (
	// Engine is in an uninitialized state
	EngineStageUninitialized Stage = iota
	// Engine is currently booting up
	EngineStageBooting
	// Engine completed boot process and is ready to be initialized
	EngineStageBootComplete
	// Engine is currently initializing
	EngineStageInitializing
	// Engine initialization is complete
	EngineStageInitialized
	// Engine is currently running
	EngineStageRunning
	// Engine is in the process of shutting down
	EngineStageShuttingDown
)

type Engine struct {
	currentStage  Stage
	gameInstance  *Game
	isRunning     bool
	isSuspended   bool
	platform      *platform.Platform
	assetManager  *assets.AssetManager
	systemManager *systems.SystemManager
	width         uint32
	height        uint32
	clock         *core.Clock
	lastTime      float64

	// Temporary for testing
	skybox       *metadata.Skybox
	meshes       []*metadata.Mesh
	carMesh      *metadata.Mesh
	sponzaMesh   *metadata.Mesh
	modelsLoaded bool

	uiMeshes    []*metadata.Mesh
	testText    *metadata.UIText
	testSysText *metadata.UIText
}

func New(g *Game) (*Engine, error) {
	// initialize the logger immediately
	core.InitializeLogger(g.ApplicationConfig.LogLevel)

	p := platform.New()

	am, err := assets.NewAssetManager()
	if err != nil {
		core.LogError(err.Error())
		return nil, err
	}

	sm, err := systems.NewSystemManager(g.ApplicationConfig.Name, g.ApplicationConfig.StartWidth, g.ApplicationConfig.StartHeight, p, am)
	if err != nil {
		core.LogError(err.Error())
		return nil, err
	}

	g.SystemManager = sm

	return &Engine{
		currentStage:  EngineStageUninitialized,
		gameInstance:  g,
		clock:         core.NewClock(),
		platform:      p,
		assetManager:  am,
		systemManager: sm,
		isRunning:     true,
		isSuspended:   false,
		width:         g.ApplicationConfig.StartWidth,
		height:        g.ApplicationConfig.StartHeight,
		lastTime:      0,
		// temp stuff
		skybox: &metadata.Skybox{
			Cubemap:  &metadata.TextureMap{},
			Geometry: &metadata.Geometry{},
		},
		modelsLoaded: false,
	}, nil
}

func (e *Engine) Initialize() error {
	// initialize input
	if err := core.InputInitialize(); err != nil {
		return err
	}

	// initialize events
	if !core.EventSystemInitialize() {
		return fmt.Errorf("failed to initialize the event system")
	}

	// register some events
	core.EventRegister(core.EVENT_CODE_APPLICATION_QUIT, e.onEvent)
	core.EventRegister(core.EVENT_CODE_KEY_PRESSED, e.onKey)
	core.EventRegister(core.EVENT_CODE_KEY_RELEASED, e.onKey)
	core.EventRegister(core.EVENT_CODE_RESIZED, e.onResized)

	if err := e.platform.Startup(e.gameInstance.ApplicationConfig.Name,
		e.gameInstance.ApplicationConfig.StartPosX,
		e.gameInstance.ApplicationConfig.StartPosY,
		e.gameInstance.ApplicationConfig.StartWidth,
		e.gameInstance.ApplicationConfig.StartHeight); err != nil {
		return err
	}

	// initialize subsystems
	wd, err := os.Getwd()
	if err != nil {
		return err
	}
	if err := e.assetManager.Initialize(fmt.Sprintf("%s/assets", wd)); err != nil {
		return err
	}

	// initialize all the managers (including the rendering system)
	if err := e.systemManager.Initialize(); err != nil {
		return err
	}

	// TODO: temp
	// Create test ui text objects
	// text, err := e.systemManager.FontSystem.UITextCreate(metadata.UI_TEXT_TYPE_BITMAP, "Ubuntu Mono 21px", 21, "Some test text 123,\n\tyo!")
	// if err != nil {
	// 	core.LogError("failed to load basic ui bitmap text")
	// 	return err
	// }
	// e.testText = text

	// // Move debug text to new bottom of screen.
	// e.systemManager.FontSystem.UITextSetPosition(e.testText, math.NewVec3(20, float32(e.height-75), 0))

	// text, err = e.systemManager.FontSystem.UITextCreate(metadata.UI_TEXT_TYPE_SYSTEM, "Noto Sans CJK JP", 31, "Some system text 123, \n\tyo!\n\n\tこんにちは 한")
	// if err != nil {
	// 	core.LogError("failed to load basic ui system text")
	// 	return err
	// }
	// e.testSysText = text
	// e.systemManager.FontSystem.UITextSetPosition(e.testSysText, math.NewVec3(50, 250, 0))

	// text, err = e.systemManager.FontSystem.UITextCreate(metadata.UI_TEXT_TYPE_SYSTEM, "Noto Sans CJK JP", 31, "Some system text 123, \n\tyo!\n\n\tこんにちは 한")
	// if err != nil {
	// 	core.LogError("failed to load basic ui system text")
	// 	return err
	// }
	// e.testSysText = text
	// e.systemManager.FontSystem.UITextSetPosition(e.testSysText, math.NewVec3(50, 200, 0))

	// Skybox
	e.skybox.Cubemap.FilterMagnify = metadata.TextureFilterModeLinear
	e.skybox.Cubemap.FilterMinify = metadata.TextureFilterModeLinear
	e.skybox.Cubemap.RepeatU = metadata.TextureRepeatClampToEdge
	e.skybox.Cubemap.RepeatV = metadata.TextureRepeatClampToEdge
	e.skybox.Cubemap.RepeatW = metadata.TextureRepeatClampToEdge
	e.skybox.Cubemap.Use = metadata.TextureUseMapCubemap
	if err := e.systemManager.RendererSystem.TextureMapAcquireResources(e.skybox.Cubemap); err != nil {
		core.LogError("unable to acquire resources for cube map texture")
		return err
	}

	t, err := e.systemManager.TextureSystem.AquireCube("skybox", true)
	if err != nil {
		return err
	}
	e.skybox.Cubemap.Texture = t
	skyboxCubeConfig, err := e.systemManager.GeometrySystem.GenerateCubeConfig(10.0, 10.0, 10.0, 1.0, 1.0, "skybox_cube", "")
	if err != nil {
		return err
	}

	// Clear out the material name.
	skyboxCubeConfig.MaterialName = ""
	g, err := e.systemManager.GeometrySystem.AcquireFromConfig(skyboxCubeConfig, true)
	if err != nil {
		return err
	}
	e.skybox.Geometry = g
	e.skybox.RenderFrameNumber = metadata.InvalidIDUint64
	skyboxShader, err := e.systemManager.ShaderSystem.GetShader("Shader.Builtin.Skybox")
	if err != nil {
		return err
	}
	maps := []*metadata.TextureMap{e.skybox.Cubemap}
	e.skybox.InstanceID, err = e.systemManager.RendererSystem.ShaderAcquireInstanceResources(skyboxShader, maps)
	if err != nil {
		return err
	}

	// Invalidate all meshes.
	if len(e.meshes) == 0 {
		e.meshes = make([]*metadata.Mesh, 10)
	}
	for i := 0; i < 10; i++ {
		if e.meshes[i] == nil {
			e.meshes[i] = &metadata.Mesh{}
		}
		e.meshes[i].Generation = metadata.InvalidIDUint8
	}

	meshCount := 0

	// Load up a cube configuration, and load geometry from it.
	cubeMesh1 := e.meshes[meshCount]
	cubeMesh1.GeometryCount = 1
	cubeMesh1.Geometries = make([]*metadata.Geometry, 1)
	gConfig, err := e.systemManager.GeometrySystem.GenerateCubeConfig(10.0, 10.0, 10.0, 1.0, 1.0, "test_cube", "test_material")
	if err != nil {
		return err
	}
	c, err := e.systemManager.GeometrySystem.AcquireFromConfig(gConfig, true)
	if err != nil {
		return err
	}
	cubeMesh1.Geometries[0] = c
	cubeMesh1.Transform = math.TransformCreate()
	cubeMesh1.Generation = 0
	meshCount++

	// Clean up the allocations for the geometry config.
	e.systemManager.GeometrySystem.ConfigDispose(gConfig)

	// A second cube
	cubeMesh2 := e.meshes[meshCount]
	cubeMesh2.GeometryCount = 1
	cubeMesh2.Geometries = make([]*metadata.Geometry, 1)
	gConfig, err = e.systemManager.GeometrySystem.GenerateCubeConfig(5.0, 5.0, 5.0, 1.0, 1.0, "test_cube_2", "test_material")
	if err != nil {
		return err
	}
	c, err = e.systemManager.GeometrySystem.AcquireFromConfig(gConfig, true)
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
	e.systemManager.GeometrySystem.ConfigDispose(gConfig)

	// A third cube!
	cubeMesh3 := e.meshes[meshCount]
	cubeMesh3.GeometryCount = 1
	cubeMesh3.Geometries = make([]*metadata.Geometry, 1)
	gConfig, err = e.systemManager.GeometrySystem.GenerateCubeConfig(2.0, 2.0, 2.0, 1.0, 1.0, "test_cube_3", "test_material")
	if err != nil {
		return err
	}
	c, err = e.systemManager.GeometrySystem.AcquireFromConfig(gConfig, true)
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
	e.systemManager.GeometrySystem.ConfigDispose(gConfig)

	e.carMesh = e.meshes[meshCount]
	e.carMesh.Transform = math.TransformFromPosition(math.NewVec3(15.0, 0.0, 1.0))
	meshCount++

	e.sponzaMesh = e.meshes[meshCount]
	e.sponzaMesh.Transform = math.TransformFromPositionRotationScale(math.NewVec3(15.0, 0.0, 1.0), math.NewQuatIdentity(), math.NewVec3(0.05, 0.05, 0.05))
	meshCount++

	// END: temporary stuff

	if err := e.gameInstance.FnInitialize(); err != nil {
		return err
	}

	if err := e.gameInstance.FnOnResize(e.width, e.height); err != nil {
		return err
	}
	return nil
}

func (e *Engine) Run() error {
	e.clock.Start()
	e.clock.Update()

	e.lastTime = e.clock.Elapsed()

	// start goroutine to process all the events around the engine
	go core.ProcessEvents()

	// var runningTime float64 = 0.0
	var frameCount uint8 = 0
	var frameElapsedTime float64 = 0
	var targetFrameSeconds float64 = 1.0 / 60.0
	var frame_avg_counter uint8 = 0
	ms_times := [30]float64{0}
	var msAvg float64 = 0
	var frames int32 = 0
	var accumulated_frame_ms float64 = 0
	var fps float64 = 0
	var AVG_COUNT = 30

	for e.isRunning {
		if !e.platform.PumpMessages() {
			e.isRunning = false
		}

		if !e.isSuspended {
			// Update clock and get delta time.
			e.clock.Update()

			var currentTime float64 = e.clock.Elapsed()
			var delta float64 = (currentTime - e.lastTime)
			var frameStartTime float64 = platform.GetAbsoluteTime()

			if err := e.gameInstance.FnUpdate(delta); err != nil {
				core.LogFatal("Game update failed, shutting down.")
				e.isRunning = false
				break
			}

			// Call the game's render routine.
			if err := e.gameInstance.FnRender(delta); err != nil {
				core.LogFatal("Game render failed, shutting down.")
				e.isRunning = false
				break
			}

			// Perform a small rotation on the first mesh.
			rotation := math.NewQuatFromAxisAngle(math.NewVec3(0, 1, 0), float32(0.5*delta), false)
			e.meshes[0].Transform.Rotate(rotation)
			// Perform a similar rotation on the second mesh, if it exists.
			e.meshes[1].Transform.Rotate(rotation)
			// Perform a similar rotation on the third mesh, if it exists.
			e.meshes[2].Transform.Rotate(rotation)

			packet := &metadata.RenderPacket{
				DeltaTime: delta,
				ViewCount: 4,
				Views:     make([]*metadata.RenderViewPacket, 4),
			}

			// skybox
			skyboxPacketData := &metadata.SkyboxPacketData{
				Skybox: e.skybox,
			}
			rvp, err := e.systemManager.RenderViewSystem.BuildPacket(e.systemManager.RenderViewSystem.Get("skybox"), skyboxPacketData)
			if err != nil {
				core.LogError("Failed to build packet for view 'skybox'.")
				return err
			}
			packet.Views[0] = rvp

			// World
			meshCount := 0
			meshes := make([]*metadata.Mesh, 10)
			for i := 0; i < 10; i++ {
				if e.meshes[i].Generation != metadata.InvalidIDUint8 {
					meshes[meshCount] = e.meshes[i]
					meshCount++
				}
			}
			worldMeshData := &metadata.MeshPacketData{
				MeshCount: uint32(meshCount),
				Meshes:    meshes,
			}
			rvp, err = e.systemManager.RenderViewSystem.BuildPacket(e.systemManager.RenderViewSystem.Get("world"), worldMeshData)
			if err != nil {
				core.LogError("Failed to build packet for view 'world'.")
				return err
			}
			packet.Views[1] = rvp

			// Update the bitmap text with camera position. NOTE: just using the default camera for now.
			worldCamera := e.systemManager.CameraSystem.GetDefault()
			pos := worldCamera.GetPosition()
			rot := worldCamera.GetEulerRotation()

			// also track on current mouse state
			leftDown := core.InputIsButtonDown(core.BUTTON_LEFT)
			rightDown := core.InputIsButtonDown(core.BUTTON_RIGHT)
			mouseX, mouseY := core.InputGetMousePosition()

			// convert to NDC
			mouseXNDC := math.RangeConvertFloat32(float32(mouseX), 0, float32(e.width), -1, 1)
			mouseYNDC := math.RangeConvertFloat32(float32(mouseY), 0, float32(e.height), -1, 1)

			// Calculate frame ms average
			frame_ms := (frameElapsedTime * 1000.0)
			ms_times[frame_avg_counter] = frame_ms
			if frame_avg_counter == uint8(AVG_COUNT-1) {
				for i := 0; i < AVG_COUNT; i++ {
					msAvg += ms_times[i]
				}
				msAvg /= float64(AVG_COUNT)
			}
			frame_avg_counter++
			frame_avg_counter %= uint8(AVG_COUNT)

			// Calculate frames per second.
			accumulated_frame_ms += frame_ms
			if accumulated_frame_ms > 1000 {
				fps = float64(frames)
				accumulated_frame_ms -= 1000
				frames = 0
			}

			textBuffer := fmt.Sprintf(
				"FPS: %5.1f(%4.1fms)        Pos=[%7.3f %7.3f %7.3f ] Rot=[%7.3f, %7.3f, %7.3f  ]\n"+
					"Mouse: X=%-5d Y=%-5d   L=%s R=%s   NDC: X=%.6f, Y=%.6f\n"+
					"Hovered: %s%d",
				fps,
				msAvg,
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

			ui_packet := &metadata.UIPacketData{
				MeshData: &metadata.MeshPacketData{},
			}

			ui_mesh_count := uint32(0)
			ui_meshes := make([]*metadata.Mesh, 10)

			// TODO: flexible size array
			for i := 0; i < len(e.uiMeshes); i++ {
				if e.uiMeshes[i] != nil {
					if e.uiMeshes[i].Generation != metadata.InvalidIDUint8 {
						ui_meshes[ui_mesh_count] = e.uiMeshes[i]
						ui_mesh_count++
					}
				}
			}

			ui_packet.MeshData.MeshCount = ui_mesh_count
			ui_packet.MeshData.Meshes = ui_meshes
			ui_packet.Texts = make([]*metadata.UIText, 2)

			ui_packet.Texts[0] = e.testText
			ui_packet.Texts[1] = e.testSysText

			rvp, err = e.systemManager.RenderViewSystem.BuildPacket(e.systemManager.RenderViewSystem.Get("ui"), ui_packet)
			if err != nil {
				core.LogError("Failed to build packet for view 'ui'.")
				return err
			}
			packet.Views[2] = rvp

			// Pick uses both world and ui packet data.
			pick_packet := &metadata.PickPacketData{
				UIMeshData:    ui_packet.MeshData,
				WorldMeshData: worldMeshData,
				Texts:         ui_packet.Texts,
				TextCount:     uint32(len(ui_packet.Texts)),
			}

			rvp, err = e.systemManager.RenderViewSystem.BuildPacket(e.systemManager.RenderViewSystem.Get("pick"), pick_packet)
			if err != nil {
				core.LogError("Failed to build packet for view 'pick'.")
				return err
			}
			packet.Views[3] = rvp

			// Draw frame
			if err := e.systemManager.DrawFrame(packet); err != nil {
				core.LogError("failed to draw frame")
				return err
			}

			// TODO: temp
			// Cleanup the packet.
			for i := 0; i < int(packet.ViewCount); i++ {
				if err := packet.Views[i].View.View.OnDestroy(); err != nil {
					core.LogError("failed to destroy renderview")
					return err
				}
			}
			// TODO: end temp

			// Figure out how long the frame took and, if below
			var frameEndTime float64 = platform.GetAbsoluteTime()
			var frameElapsedTime float64 = frameEndTime - frameStartTime
			// runningTime += frameElapsedTime
			var remainingSeconds float64 = targetFrameSeconds - frameElapsedTime

			if remainingSeconds > 0 {
				remainingMS := (remainingSeconds * 1000)
				// If there is time left, give it back to the OS.
				limitFrames := false
				if remainingMS > 0 && limitFrames {
					e.platform.Sleep(remainingMS - 1)
				}
				frameCount++
			}

			frames++

			// NOTE: Input update/state copying should always be handled
			// after any input should be recorded; I.E. before this line.
			// As a safety, input is the last thing to be updated before
			// this frame ends.
			core.InputUpdate(delta)

			// Update last time
			e.lastTime = currentTime
		}
	}

	return nil
}

func (e *Engine) Shutdown() error {
	if err := core.EventSystemShutdown(); err != nil {
		return err
	}
	if err := core.InputShutdown(); err != nil {
		return err
	}
	if err := e.systemManager.Shutdown(); err != nil {
		return err
	}
	if err := e.platform.Shutdown(); err != nil {
		return err
	}
	return nil
}

// ApplicationGetFramebufferSize returns the width and height (in this order)
// of the application Framebuffer
func (e *Engine) GetFramebufferSize() (uint32, uint32) {
	return e.width, e.height
}

func (e *Engine) onEvent(context core.EventContext) {
	switch context.Type {
	case core.EVENT_CODE_APPLICATION_QUIT:
		{
			core.LogInfo("EVENT_CODE_APPLICATION_QUIT recieved, shutting down.\n")
			e.isRunning = false
		}
	}
}

func (e *Engine) onKey(context core.EventContext) {
	ke, ok := context.Data.(*core.KeyEvent)
	if !ok {
		core.LogError("wrong event associated with the event type `%d`", context.Type)
		return
	}

	keyCode := ke.KeyCode

	if context.Type == core.EVENT_CODE_KEY_PRESSED {
		if keyCode == core.KEY_ESCAPE {
			// NOTE: Technically firing an event to itself, but there may be other listeners.
			data := core.EventContext{
				Type: core.EVENT_CODE_APPLICATION_QUIT,
			}
			core.EventFire(data)
			// Block anything else from processing this.
			return
		} else if keyCode == core.KEY_A {
			// Example on checking for a key
			core.LogInfo("Explicit - A key pressed!")
		} else {
			core.LogInfo("'%c' key pressed in window.", keyCode)
		}
	} else if context.Type == core.EVENT_CODE_KEY_RELEASED {
		if keyCode == core.KEY_B {
			// Example on checking for a key
			core.LogInfo("Explicit - B key released!")
		} else {
			core.LogInfo("'%c' key released in window.", keyCode)
		}
	}
}

func (e *Engine) onResized(context core.EventContext) {
	if context.Type == core.EVENT_CODE_RESIZED {
		se, ok := context.Data.(*core.SystemEvent)
		if !ok {
			core.LogError("wrong event associated with the event type `%d`", context.Type)
			return
		}

		width := se.WindowWidth
		height := se.WindowHeight

		// Check if different. If so, trigger a resize event.
		if width != e.width || height != e.height {
			e.width = width
			e.height = height

			core.LogDebug("Window resize: %d, %d", width, height)

			// Handle minimization
			if width == 0 || height == 0 {
				core.LogInfo("Window minimized, suspending application.")
				e.isSuspended = true
				return
			} else {
				if e.isSuspended {
					core.LogInfo("Window restored, resuming application.")
					e.isSuspended = false
				}
				e.gameInstance.FnOnResize(uint32(width), uint32(height))
				if err := e.systemManager.OnResize(uint16(width), uint16(height)); err != nil {
					core.LogError(err.Error())
				}
			}
		}
	}
}
