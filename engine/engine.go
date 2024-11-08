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
}

func New(g *Game) (*Engine, error) {
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

	if err := e.systemManager.Initialize(); err != nil {
		return err
	}

	// START: temporary stuff
	// skyboxConfig := &metadata.RenderViewConfig{
	// 	RenderViewType: metadata.RENDERER_VIEW_KNOWN_TYPE_SKYBOX,
	// 	Width:          0,
	// 	Height:         0,
	// 	Name:           "skybox",
	// 	PassCount:      1,
	// 	Passes: []metadata.RenderViewPassConfig{
	// 		{
	// 			Name: "Renderpass.Builtin.Skybox",
	// 		},
	// 	},
	// 	ViewMatrixSource: metadata.RENDER_VIEW_VIEW_MATRIX_SOURCE_SCENE_CAMERA,
	// }
	// if err := e.systemManager.RenderViewCreate(skyboxConfig); err != nil {
	// 	core.LogFatal(err.Error())
	// 	return err
	// }

	// opaqueWorldConfig := &metadata.RenderViewConfig{
	// 	RenderViewType: metadata.RENDERER_VIEW_KNOWN_TYPE_WORLD,
	// 	Width:          0,
	// 	Height:         0,
	// 	Name:           "world_opaque",
	// 	PassCount:      1,
	// 	Passes: []metadata.RenderViewPassConfig{
	// 		{
	// 			Name: "Renderpass.Builtin.World",
	// 		},
	// 	},
	// 	ViewMatrixSource: metadata.RENDER_VIEW_VIEW_MATRIX_SOURCE_SCENE_CAMERA,
	// }
	// if err := e.systemManager.RenderViewCreate(opaqueWorldConfig); err != nil {
	// 	core.LogFatal(err.Error())
	// 	return err
	// }

	// // Skybox
	// e.skybox.Cubemap.FilterMagnify = metadata.TextureFilterModeLinear
	// e.skybox.Cubemap.FilterMinify = metadata.TextureFilterModeLinear
	// e.skybox.Cubemap.RepeatU = metadata.TextureRepeatClampToEdge
	// e.skybox.Cubemap.RepeatV = metadata.TextureRepeatClampToEdge
	// e.skybox.Cubemap.RepeatW = metadata.TextureRepeatClampToEdge
	// e.skybox.Cubemap.Use = metadata.TextureUseMapCubemap
	// if !e.systemManager.RendererSystem.TextureMapAcquireResources(e.skybox.Cubemap) {
	// 	err := fmt.Errorf("unable to acquire resources for cube map texture")
	// 	return err
	// }

	// t, err := e.systemManager.TextureSystem.AcquireCube("skybox", true)
	// if err != nil {
	// 	return err
	// }
	// e.skybox.Cubemap.Texture = t
	// skyboxCubeConfig, err := e.systemManager.GeometrySystem.GenerateCubeConfig(10.0, 10.0, 10.0, 1.0, 1.0, "skybox_cube", "")
	// if err != nil {
	// 	return err
	// }

	// // Clear out the material name.
	// skyboxCubeConfig.MaterialName = ""
	// g, err := e.systemManager.GeometrySystem.AcquireFromConfig(skyboxCubeConfig, true)
	// if err != nil {
	// 	return err
	// }
	// e.skybox.Geometry = g
	// e.skybox.RenderFrameNumber = metadata.InvalidIDUint64
	// skyboxShader, err := e.systemManager.ShaderSystem.GetShader(metadata.BUILTIN_SHADER_NAME_SKYBOX)
	// if err != nil {
	// 	return err
	// }
	// maps := []*metadata.TextureMap{e.skybox.Cubemap}
	// e.skybox.InstanceID, err = e.systemManager.RendererSystem.ShaderAcquireInstanceResources(skyboxShader, maps)
	// if err != nil {
	// 	return err
	// }

	// // Invalidate all meshes.
	// for i := 0; i < 10; i++ {
	// 	e.meshes[i].Generation = metadata.InvalidIDUint8
	// }

	// meshCount := 0

	// // Load up a cube configuration, and load geometry from it.
	// e.meshes[meshCount].GeometryCount = 1
	// e.meshes[meshCount].Geometries = make([]*metadata.Geometry, 1)
	// gConfig, err := e.systemManager.GeometrySystem.GenerateCubeConfig(10.0, 10.0, 10.0, 1.0, 1.0, "test_cube", "test_material")
	// if err != nil {
	// 	return err
	// }
	// c, err := e.systemManager.GeometrySystem.AcquireFromConfig(gConfig, true)
	// if err != nil {
	// 	return err
	// }
	// e.meshes[meshCount].Geometries[0] = c
	// e.meshes[meshCount].Transform = math.TransformCreate()
	// meshCount++
	// e.meshes[meshCount].Generation = 0
	// // Clean up the allocations for the geometry config.
	// e.systemManager.GeometrySystem.ConfigDispose(gConfig)

	// // A second cube
	// e.meshes[meshCount].GeometryCount = 1
	// e.meshes[meshCount].Geometries = make([]*metadata.Geometry, 1)
	// gConfig, err = e.systemManager.GeometrySystem.GenerateCubeConfig(5.0, 5.0, 5.0, 1.0, 1.0, "test_cube_2", "test_material")
	// if err != nil {
	// 	return err
	// }
	// c, err = e.systemManager.GeometrySystem.AcquireFromConfig(gConfig, true)
	// if err != nil {
	// 	return err
	// }
	// e.meshes[meshCount].Geometries[0] = c
	// e.meshes[meshCount].Transform = math.TransformFromPosition(math.NewVec3(10.0, 0.0, 1.0))
	// // Set the first cube as the parent to the second.
	// e.meshes[meshCount].Transform.Parent = e.meshes[meshCount].Transform
	// meshCount++
	// e.meshes[meshCount].Generation = 0
	// // Clean up the allocations for the geometry config.
	// e.systemManager.GeometrySystem.ConfigDispose(gConfig)

	// // A third cube!
	// e.meshes[meshCount].GeometryCount = 1
	// e.meshes[meshCount].Geometries = make([]*metadata.Geometry, 1)
	// gConfig, err = e.systemManager.GeometrySystem.GenerateCubeConfig(2.0, 2.0, 2.0, 1.0, 1.0, "test_cube_2", "test_material")
	// if err != nil {
	// 	return err
	// }
	// c, err = e.systemManager.GeometrySystem.AcquireFromConfig(gConfig, true)
	// if err != nil {
	// 	return err
	// }
	// e.meshes[meshCount].Geometries[0] = c
	// e.meshes[meshCount].Transform = math.TransformFromPosition(math.NewVec3(5.0, 0.0, 1.0))
	// // Set the second cube as the parent to the third.
	// e.meshes[meshCount].Transform.Parent = e.meshes[meshCount].Transform
	// meshCount++
	// e.meshes[meshCount].Generation = 0
	// // Clean up the allocations for the geometry config.
	// e.systemManager.GeometrySystem.ConfigDispose(gConfig)

	// e.carMesh = e.meshes[meshCount]
	// e.carMesh.Transform = math.TransformFromPosition(math.NewVec3(15.0, 0.0, 1.0))
	// meshCount++

	// e.sponzaMesh = e.meshes[meshCount]
	// e.sponzaMesh.Transform = math.TransformFromPositionRotationScale(math.NewVec3(15.0, 0.0, 1.0), math.NewQuatIdentity(), math.NewVec3(0.05, 0.05, 0.05))
	// meshCount++

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

	var runningTime float64 = 0.0
	var frameCount uint8 = 0
	var targetFrameSeconds float64 = 1.0 / 60.0

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
				ViewCount: 3,
				Views:     make([]*metadata.RenderViewPacket, 3),
			}

			// skybox
			skyboxPacketData := &metadata.SkyboxPacketData{
				Skybox: e.skybox,
			}
			packet.Views[0] = e.systemManager.RenderViewSystem.BuildPacket(e.systemManager.RenderViewSystem.Get("skybox"), skyboxPacketData)

			// World
			meshes := []*metadata.Mesh{}
			for _, m := range e.meshes {
				if m.Generation != metadata.InvalidIDUint8 {
					meshes = append(meshes, m)
				}
			}
			worldMeshData := &metadata.MeshPacketData{
				MeshCount: uint32(len(meshes)),
				Meshes:    meshes,
			}
			packet.Views[1] = e.systemManager.RenderViewSystem.BuildPacket(e.systemManager.RenderViewSystem.Get("world_opaque"), worldMeshData)

			// Update the bitmap text with camera position. NOTE: just using the default camera for now.
			worldCamera := e.systemManager.CameraSystem.GetDefault()
			pos := worldCamera.GetPosition()
			rot := worldCamera.GetEulerRotation()

			core.LogInfo(fmt.Sprintf("Camera Pos: [%.3f, %.3f, %.3f]\nCamera Rot: [%.3f, %.3f, %.3f]", pos.X, pos.Y, pos.Z, math.RadToDeg(rot.X), math.RadToDeg(rot.Y), math.RadToDeg(rot.Z)))

			// Draw freame
			e.systemManager.DrawFrame(packet)

			// Figure out how long the frame took and, if below
			var frameEndTime float64 = platform.GetAbsoluteTime()
			var frameElapsedTime float64 = frameEndTime - frameStartTime
			runningTime += frameElapsedTime
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
