package systems

import (
	"fmt"
	"sync"

	"github.com/spaghettifunk/anima/engine/core"
	"github.com/spaghettifunk/anima/engine/math"
	"github.com/spaghettifunk/anima/engine/renderer"
	"github.com/spaghettifunk/anima/engine/renderer/metadata"
	"github.com/spaghettifunk/anima/engine/resources/loaders"
)

type geometrySystemState struct {
	Config            *metadata.GeometrySystemConfig
	DefaultGeometry   *metadata.Geometry
	Default2DGeometry *metadata.Geometry
	// Array of registered meshes.
	RegisteredGeometries []*metadata.GeometryReference
}

var onceGeometrySystem sync.Once
var gsState *geometrySystemState

/**
 * @brief Initializes the geometry system.
 * Should be called twice; once to get the memory requirement (passing state=0), and a second
 * time passing an allocated block of memory to actually initialize the system.
 *
 * @param memory_requirement A pointer to hold the memory requirement as it is calculated.
 * @param state A block of memory to hold the state or, if gathering the memory requirement, 0.
 * @param config The configuration for this system.
 * @return True on success; otherwise false.
 */
func NewGeometrySystem(config *metadata.GeometrySystemConfig) error {
	if config.MaxGeometryCount == 0 {
		err := fmt.Errorf("func NewGeometrySystem - config.MaxGeometryCount must be > 0")
		core.LogWarn(err.Error())
		return err
	}

	var err error
	onceGeometrySystem.Do(func() {
		gsState = &geometrySystemState{
			Config:               config,
			RegisteredGeometries: make([]*metadata.GeometryReference, config.MaxGeometryCount),
		}

		// Invalidate all geometries in the array.
		count := gsState.Config.MaxGeometryCount
		for i := uint32(0); i < count; i++ {
			gsState.RegisteredGeometries[i].Geometry.ID = loaders.InvalidID
			gsState.RegisteredGeometries[i].Geometry.InternalID = loaders.InvalidID
			gsState.RegisteredGeometries[i].Geometry.Generation = loaders.InvalidIDUint16
		}

		if !gsState.createDefaultGeometries() {
			err = fmt.Errorf("failed to create default geometries. Application cannot continue")
			core.LogError(err.Error())
		}
	})

	return err
}

/**
 * @brief Shuts down the geometry system.
 *
 * @param state The state block of memory.
 */
func GeometrySystemShutdown() {}

/**
 * @brief Acquires an existing geometry by id.
 *
 * @param id The geometry identifier to acquire by.
 * @return A pointer to the acquired geometry or nullptr if failed.
 */
func GeometrySystemAcquireByID(id uint32) (*metadata.Geometry, error) {
	if id != loaders.InvalidID && gsState.RegisteredGeometries[id].Geometry.ID != loaders.InvalidID {
		gsState.RegisteredGeometries[id].ReferenceCount++
		return gsState.RegisteredGeometries[id].Geometry, nil
	}

	// NOTE: Should return default geometry instead?
	err := fmt.Errorf("func GeometrySystemAcquireByID cannot load invalid geometry id. Returning nullptr")
	core.LogError(err.Error())
	return nil, err
}

/**
 * @brief Registers and acquires a new geometry using the given config.
 *
 * @param config The geometry configuration.
 * @param auto_release Indicates if the acquired geometry should be unloaded when its reference count reaches 0.
 * @return A pointer to the acquired geometry or nullptr if failed.
 */
func GeometrySystemAcquireFromConfig(config *metadata.GeometryConfig, autoRelease bool) (*metadata.Geometry, error) {
	var geometry *metadata.Geometry
	for i := uint32(0); i < gsState.Config.MaxGeometryCount; i++ {
		if gsState.RegisteredGeometries[i].Geometry.ID == loaders.InvalidID {
			// Found empty slot.
			gsState.RegisteredGeometries[i].AutoRelease = autoRelease
			gsState.RegisteredGeometries[i].ReferenceCount = 1
			geometry = gsState.RegisteredGeometries[i].Geometry
			geometry.ID = i
			break
		}
	}

	if geometry == nil {
		err := fmt.Errorf("unable to obtain free slot for geometry. Adjust configuration to allow more space. Returning nullptr")
		core.LogError(err.Error())
		return nil, err
	}

	if !gsState.create_geometry(config, geometry) {
		err := fmt.Errorf("failed to create geometry. Returning nullptr")
		core.LogError(err.Error())
		return nil, err
	}

	return geometry, nil
}

/**
 * @brief Frees resources held by the provided configuration.
 *
 * @param config A pointer to the configuration to be disposed.
 */
func GeometrySystemConfigDispose(config *metadata.GeometryConfig) error {
	if len(config.Vertices) > 0 {
		config.Vertices = nil
	}
	if len(config.Indices) > 0 {
		config.Indices = nil
	}
	return nil
}

/**
 * @brief Releases a reference to the provided geometry.
 *
 * @param geometry The geometry to be released.
 */
func GeometrySystemRelease(geometry *metadata.Geometry) {
	if geometry != nil && geometry.ID != loaders.InvalidID {
		ref := gsState.RegisteredGeometries[geometry.ID]

		// Take a copy of the id;
		id := geometry.ID
		if ref.Geometry.ID == id {
			if ref.ReferenceCount > 0 {
				ref.ReferenceCount--
			}

			// Also blanks out the geometry id.
			if ref.ReferenceCount < 1 && ref.AutoRelease {
				gsState.destroyGeometry(ref.Geometry)
				ref.ReferenceCount = 0
				ref.AutoRelease = false
			}
		} else {
			core.LogError("Geometry id mismatch. Check registration logic, as this should never occur.")
		}
		return
	}

	core.LogWarn("geometry_system_release cannot release invalid geometry id. Nothing was done.")
}

/**
 * @brief Obtains a pointer to the default geometry.
 *
 * @return A pointer to the default geometry.
 */
func GeometrySystemGetDefault() *metadata.Geometry {
	return gsState.DefaultGeometry
}

/**
 * @brief Obtains a pointer to the default geometry.
 *
 * @return A pointer to the default geometry.
 */
func GeometrySystemGetDefault2D() *metadata.Geometry {
	return gsState.Default2DGeometry
}

/**
 * @brief Generates configuration for plane geometries given the provided parameters.
 * NOTE: vertex and index arrays are dynamically allocated and should be freed upon object disposal.
 * Thus, this should not be considered production code.
 *
 * @param width The overall width of the plane. Must be non-zero.
 * @param height The overall height of the plane. Must be non-zero.
 * @param xSegmentCount The number of segments along the x-axis in the plane. Must be non-zero.
 * @param ySegmentCount The number of segments along the y-axis in the plane. Must be non-zero.
 * @param tileX The number of times the texture should tile across the plane on the x-axis. Must be non-zero.
 * @param tileY The number of times the texture should tile across the plane on the y-axis. Must be non-zero.
 * @param name The name of the generated geometry.
 * @param material_name The name of the material to be used.
 * @return A geometry configuration which can then be fed into geometry_system_acquire_from_config().
 */
func GeometrySystemGeneratePlaneConfig(width, height float32, xSegmentCount, ySegmentCount uint32, tileX, tileY float32, name, materialName string) (*metadata.GeometryConfig, error) {
	if width == 0 {
		core.LogWarn("Width must be nonzero. Defaulting to one.")
		width = 1.0
	}
	if height == 0 {
		core.LogWarn("Height must be nonzero. Defaulting to one.")
		height = 1.0
	}
	if xSegmentCount < 1 {
		core.LogWarn("xSegmentCount must be a positive number. Defaulting to one.")
		xSegmentCount = 1
	}
	if ySegmentCount < 1 {
		core.LogWarn("ySegmentCount must be a positive number. Defaulting to one.")
		ySegmentCount = 1
	}

	if tileX == 0 {
		core.LogWarn("tileX must be nonzero. Defaulting to one.")
		tileX = 1.0
	}
	if tileY == 0 {
		core.LogWarn("tileY must be nonzero. Defaulting to one.")
		tileY = 1.0
	}

	config := &metadata.GeometryConfig{
		VertexSize:  0,
		VertexCount: xSegmentCount * ySegmentCount * 4, // 4 verts per segment
		Vertices:    make([]math.Vertex3D, xSegmentCount*ySegmentCount*4),
		IndexSize:   4,                                 // 4 bytes is the size of uint32 in Go
		IndexCount:  xSegmentCount * ySegmentCount * 6, // 6 indices per segment
		Indices:     make([]uint32, xSegmentCount*ySegmentCount*6),
	}

	// TODO: This generates extra vertices, but we can always deduplicate them later.
	seg_width := width / float32(xSegmentCount)
	seg_height := height / float32(ySegmentCount)
	half_width := width * 0.5
	half_height := height * 0.5
	for y := uint32(0); y < ySegmentCount; y++ {
		for x := uint32(0); x < xSegmentCount; x++ {
			// Generate vertices
			min_x := (float32(x) * seg_width) - half_width
			min_y := (float32(y) * seg_height) - half_height
			max_x := min_x + seg_width
			max_y := min_y + seg_height
			min_uvx := (float32(x) / float32(xSegmentCount)) * tileX
			min_uvy := (float32(y) / float32(ySegmentCount)) * tileY
			max_uvx := (float32((x + 1)) / float32(xSegmentCount)) * tileX
			max_uvy := (float32((y + 1)) / float32(ySegmentCount)) * tileY

			v_offset := ((y * xSegmentCount) + x) * 4
			v0 := (config.Vertices)[v_offset+0]
			v1 := (config.Vertices)[v_offset+1]
			v2 := (config.Vertices)[v_offset+2]
			v3 := (config.Vertices)[v_offset+3]

			v0.Position.X = min_x
			v0.Position.Y = min_y
			v0.Texcoord.X = min_uvx
			v0.Texcoord.Y = min_uvy

			v1.Position.X = max_x
			v1.Position.Y = max_y
			v1.Texcoord.X = max_uvx
			v1.Texcoord.Y = max_uvy

			v2.Position.X = min_x
			v2.Position.Y = max_y
			v2.Texcoord.X = min_uvx
			v2.Texcoord.Y = max_uvy

			v3.Position.X = max_x
			v3.Position.Y = min_y
			v3.Texcoord.X = max_uvx
			v3.Texcoord.Y = min_uvy

			// Generate indices
			i_offset := ((y * xSegmentCount) + x) * 6
			(config.Indices)[i_offset+0] = v_offset + 0
			(config.Indices)[i_offset+1] = v_offset + 1
			(config.Indices)[i_offset+2] = v_offset + 2
			(config.Indices)[i_offset+3] = v_offset + 0
			(config.Indices)[i_offset+4] = v_offset + 3
			(config.Indices)[i_offset+5] = v_offset + 1
		}
	}

	if len(name) > 0 {
		config.Name = name
	} else {
		config.Name = metadata.DefaultGeometryName
	}

	if len(materialName) > 0 {
		config.MaterialName = materialName
	} else {
		config.MaterialName = metadata.DefaultMaterialName
	}

	return config, nil
}

func GeometrySystemGenerateCubeConfig(width, height, depth, tileX, tileY float32, name, materialName string) (*metadata.GeometryConfig, error) {
	if width == 0 {
		core.LogWarn("Width must be nonzero. Defaulting to one.")
		width = 1.0
	}
	if height == 0 {
		core.LogWarn("Height must be nonzero. Defaulting to one.")
		height = 1.0
	}
	if depth == 0 {
		core.LogWarn("Depth must be nonzero. Defaulting to one.")
		depth = 1
	}
	if tileX == 0 {
		core.LogWarn("tileX must be nonzero. Defaulting to one.")
		tileX = 1.0
	}
	if tileY == 0 {
		core.LogWarn("tileY must be nonzero. Defaulting to one.")
		tileY = 1.0
	}

	config := &metadata.GeometryConfig{
		VertexSize:  0,
		VertexCount: 4 * 6, // 4 verts per side, 6 side
		Vertices:    make([]math.Vertex3D, 4*6),
		IndexSize:   4,     // number of bytes of a uint32
		IndexCount:  6 * 6, // 6 indices per side, 6 side
		Indices:     make([]uint32, 6*6),
	}

	half_width := width * 0.5
	half_height := height * 0.5
	half_depth := depth * 0.5
	min_x := -half_width
	min_y := -half_height
	min_z := -half_depth
	max_x := half_width
	max_y := half_height
	max_z := half_depth
	min_uvx := float32(0.0)
	min_uvy := float32(0.0)
	max_uvx := tileX
	max_uvy := tileY

	config.MinExtents.X = min_x
	config.MinExtents.Y = min_y
	config.MinExtents.Z = min_z
	config.MaxExtents.X = max_x
	config.MinExtents.Y = max_y
	config.MinExtents.Z = max_z
	// Always 0 since min/max of each axis are -/+ half of the size.
	config.Center.X = 0
	config.Center.Y = 0
	config.Center.Z = 0

	verts := make([]math.Vertex3D, 24)

	// Front face
	verts[(0*4)+0].Position = math.NewVec3(min_x, min_y, max_z)
	verts[(0*4)+1].Position = math.NewVec3(max_x, max_y, max_z)
	verts[(0*4)+2].Position = math.NewVec3(min_x, max_y, max_z)
	verts[(0*4)+3].Position = math.NewVec3(max_x, min_y, max_z)
	verts[(0*4)+0].Texcoord = math.NewVec2(min_uvx, min_uvy)
	verts[(0*4)+1].Texcoord = math.NewVec2(max_uvx, max_uvy)
	verts[(0*4)+2].Texcoord = math.NewVec2(min_uvx, max_uvy)
	verts[(0*4)+3].Texcoord = math.NewVec2(max_uvx, min_uvy)
	verts[(0*4)+0].Normal = math.NewVec3(0.0, 0.0, 1.0)
	verts[(0*4)+1].Normal = math.NewVec3(0.0, 0.0, 1.0)
	verts[(0*4)+2].Normal = math.NewVec3(0.0, 0.0, 1.0)
	verts[(0*4)+3].Normal = math.NewVec3(0.0, 0.0, 1.0)

	// Back face
	verts[(1*4)+0].Position = math.NewVec3(max_x, min_y, min_z)
	verts[(1*4)+1].Position = math.NewVec3(min_x, max_y, min_z)
	verts[(1*4)+2].Position = math.NewVec3(max_x, max_y, min_z)
	verts[(1*4)+3].Position = math.NewVec3(min_x, min_y, min_z)
	verts[(1*4)+0].Texcoord = math.NewVec2(min_uvx, min_uvy)
	verts[(1*4)+1].Texcoord = math.NewVec2(max_uvx, max_uvy)
	verts[(1*4)+2].Texcoord = math.NewVec2(min_uvx, max_uvy)
	verts[(1*4)+3].Texcoord = math.NewVec2(max_uvx, min_uvy)
	verts[(1*4)+0].Normal = math.NewVec3(0.0, 0.0, -1.0)
	verts[(1*4)+1].Normal = math.NewVec3(0.0, 0.0, -1.0)
	verts[(1*4)+2].Normal = math.NewVec3(0.0, 0.0, -1.0)
	verts[(1*4)+3].Normal = math.NewVec3(0.0, 0.0, -1.0)

	// Left
	verts[(2*4)+0].Position = math.NewVec3(min_x, min_y, min_z)
	verts[(2*4)+1].Position = math.NewVec3(min_x, max_y, max_z)
	verts[(2*4)+2].Position = math.NewVec3(min_x, max_y, min_z)
	verts[(2*4)+3].Position = math.NewVec3(min_x, min_y, max_z)
	verts[(2*4)+0].Texcoord = math.NewVec2(min_uvx, min_uvy)
	verts[(2*4)+1].Texcoord = math.NewVec2(max_uvx, max_uvy)
	verts[(2*4)+2].Texcoord = math.NewVec2(min_uvx, max_uvy)
	verts[(2*4)+3].Texcoord = math.NewVec2(max_uvx, min_uvy)
	verts[(2*4)+0].Normal = math.NewVec3(-1.0, 0.0, 0.0)
	verts[(2*4)+1].Normal = math.NewVec3(-1.0, 0.0, 0.0)
	verts[(2*4)+2].Normal = math.NewVec3(-1.0, 0.0, 0.0)
	verts[(2*4)+3].Normal = math.NewVec3(-1.0, 0.0, 0.0)

	// Right face
	verts[(3*4)+0].Position = math.NewVec3(max_x, min_y, max_z)
	verts[(3*4)+1].Position = math.NewVec3(max_x, max_y, min_z)
	verts[(3*4)+2].Position = math.NewVec3(max_x, max_y, max_z)
	verts[(3*4)+3].Position = math.NewVec3(max_x, min_y, min_z)
	verts[(3*4)+0].Texcoord = math.NewVec2(min_uvx, min_uvy)
	verts[(3*4)+1].Texcoord = math.NewVec2(max_uvx, max_uvy)
	verts[(3*4)+2].Texcoord = math.NewVec2(min_uvx, max_uvy)
	verts[(3*4)+3].Texcoord = math.NewVec2(max_uvx, min_uvy)
	verts[(3*4)+0].Normal = math.NewVec3(1.0, 0.0, 0.0)
	verts[(3*4)+1].Normal = math.NewVec3(1.0, 0.0, 0.0)
	verts[(3*4)+2].Normal = math.NewVec3(1.0, 0.0, 0.0)
	verts[(3*4)+3].Normal = math.NewVec3(1.0, 0.0, 0.0)

	// Bottom face
	verts[(4*4)+0].Position = math.NewVec3(max_x, min_y, max_z)
	verts[(4*4)+1].Position = math.NewVec3(min_x, min_y, min_z)
	verts[(4*4)+2].Position = math.NewVec3(max_x, min_y, min_z)
	verts[(4*4)+3].Position = math.NewVec3(min_x, min_y, max_z)
	verts[(4*4)+0].Texcoord = math.NewVec2(min_uvx, min_uvy)
	verts[(4*4)+1].Texcoord = math.NewVec2(max_uvx, max_uvy)
	verts[(4*4)+2].Texcoord = math.NewVec2(min_uvx, max_uvy)
	verts[(4*4)+3].Texcoord = math.NewVec2(max_uvx, min_uvy)
	verts[(4*4)+0].Normal = math.NewVec3(0.0, -1.0, 0.0)
	verts[(4*4)+1].Normal = math.NewVec3(0.0, -1.0, 0.0)
	verts[(4*4)+2].Normal = math.NewVec3(0.0, -1.0, 0.0)
	verts[(4*4)+3].Normal = math.NewVec3(0.0, -1.0, 0.0)

	// Top face
	verts[(5*4)+0].Position = math.NewVec3(min_x, max_y, max_z)
	verts[(5*4)+1].Position = math.NewVec3(max_x, max_y, min_z)
	verts[(5*4)+2].Position = math.NewVec3(min_x, max_y, min_z)
	verts[(5*4)+3].Position = math.NewVec3(max_x, max_y, max_z)
	verts[(5*4)+0].Texcoord = math.NewVec2(min_uvx, min_uvy)
	verts[(5*4)+1].Texcoord = math.NewVec2(max_uvx, max_uvy)
	verts[(5*4)+2].Texcoord = math.NewVec2(min_uvx, max_uvy)
	verts[(5*4)+3].Texcoord = math.NewVec2(max_uvx, min_uvy)
	verts[(5*4)+0].Normal = math.NewVec3(0.0, 1.0, 0.0)
	verts[(5*4)+1].Normal = math.NewVec3(0.0, 1.0, 0.0)
	verts[(5*4)+2].Normal = math.NewVec3(0.0, 1.0, 0.0)
	verts[(5*4)+3].Normal = math.NewVec3(0.0, 1.0, 0.0)

	for i := 0; i < 6; i++ {
		v_offset := i * 4
		i_offset := i * 6
		(config.Indices)[i_offset+0] = uint32(v_offset + 0)
		(config.Indices)[i_offset+1] = uint32(v_offset + 1)
		(config.Indices)[i_offset+2] = uint32(v_offset + 2)
		(config.Indices)[i_offset+3] = uint32(v_offset + 0)
		(config.Indices)[i_offset+4] = uint32(v_offset + 3)
		(config.Indices)[i_offset+5] = uint32(v_offset + 1)
	}

	if len(name) > 0 {
		config.Name = name
	} else {
		config.Name = metadata.DefaultGeometryName
	}

	if len(materialName) > 0 {
		config.MaterialName = materialName
	} else {
		config.MaterialName = metadata.DefaultMaterialName
	}

	config.Vertices = math.GeometryGenerateTangents(config.VertexCount, config.Vertices, config.IndexCount, config.Indices)

	return config, nil
}

func (gs *geometrySystemState) createDefaultGeometries() bool {
	verts := make([]math.Vertex3D, 4)

	f := float32(10.0)

	verts[0].Position.X = -0.5 * f // 0    3
	verts[0].Position.Y = -0.5 * f //
	verts[0].Texcoord.X = 0.0      //
	verts[0].Texcoord.Y = 0.0      // 2    1

	verts[1].Position.X = 0.5 * f
	verts[1].Position.Y = 0.5 * f
	verts[1].Texcoord.X = 1.0
	verts[1].Texcoord.Y = 1.0

	verts[2].Position.X = -0.5 * f
	verts[2].Position.Y = 0.5 * f
	verts[2].Texcoord.X = 0.0
	verts[2].Texcoord.Y = 1.0

	verts[3].Position.X = 0.5 * f
	verts[3].Position.Y = -0.5 * f
	verts[3].Texcoord.X = 1.0
	verts[3].Texcoord.Y = 0.0

	indices := []uint32{0, 1, 2, 0, 3, 1}

	// Send the geometry off to the renderer to be uploaded to the GPU.
	gs.DefaultGeometry.InternalID = loaders.InvalidID
	if !renderer.CreateGeometry(gs.DefaultGeometry, 0, 4, verts, 0, 6, indices) {
		core.LogFatal("Failed to create default geometry. Application cannot continue.")
		return false
	}

	// Acquire the default material.
	gs.DefaultGeometry.Material = MaterialSystemGetDefault()

	// Create default 2d geometry.
	verts2d := make([]math.Vertex2D, 4)
	verts2d[0].Position.X = -0.5 * f // 0    3
	verts2d[0].Position.Y = -0.5 * f //
	verts2d[0].Texcoord.X = 0.0      //
	verts2d[0].Texcoord.Y = 0.0      // 2    1

	verts2d[1].Position.X = 0.5 * f
	verts2d[1].Position.Y = 0.5 * f
	verts2d[1].Texcoord.X = 1.0
	verts2d[1].Texcoord.Y = 1.0

	verts2d[2].Position.X = -0.5 * f
	verts2d[2].Position.Y = 0.5 * f
	verts2d[2].Texcoord.X = 0.0
	verts2d[2].Texcoord.Y = 1.0

	verts2d[3].Position.X = 0.5 * f
	verts2d[3].Position.Y = -0.5 * f
	verts2d[3].Texcoord.X = 1.0
	verts2d[3].Texcoord.Y = 0.0

	// Indices (NOTE: counter-clockwise)
	indices2d := []uint32{2, 1, 0, 3, 0, 1}

	// Send the geometry off to the renderer to be uploaded to the GPU.
	if !renderer.CreateGeometry(gs.Default2DGeometry, 0, 4, verts2d, 0, 6, indices2d) {
		core.LogFatal("Failed to create default 2d geometry. Application cannot continue.")
		return false
	}

	// Acquire the default material.
	gs.Default2DGeometry.Material = MaterialSystemGetDefault()

	return true
}

func (gs *geometrySystemState) create_geometry(config *metadata.GeometryConfig, geometry *metadata.Geometry) bool {
	// Send the geometry off to the renderer to be uploaded to the GPU.
	if !renderer.CreateGeometry(geometry, config.VertexSize, config.VertexCount, config.Vertices, config.IndexSize, config.IndexCount, config.Indices) {
		// Invalidate the entry.
		gs.RegisteredGeometries[geometry.ID].ReferenceCount = 0
		gs.RegisteredGeometries[geometry.ID].AutoRelease = false
		geometry.ID = loaders.InvalidID
		geometry.Generation = loaders.InvalidIDUint16
		geometry.InternalID = loaders.InvalidID

		return false
	}

	// Copy over extents, center, etc.
	geometry.Center = config.Center
	geometry.Extents.Min = config.MinExtents
	geometry.Extents.Max = config.MaxExtents

	// Acquire the material
	if len(config.MaterialName) > 0 {
		mat, err := MaterialSystemAcquire(config.MaterialName)
		if err != nil {
			core.LogError(err.Error())
			return false
		}
		geometry.Material = mat
		if geometry.Material == nil {
			geometry.Material = MaterialSystemGetDefault()
		}
	}
	return true
}

func (gs *geometrySystemState) destroyGeometry(geometry *metadata.Geometry) {
	renderer.DestroyGeometry(geometry)
	geometry.InternalID = loaders.InvalidID
	geometry.Generation = loaders.InvalidIDUint16
	geometry.ID = loaders.InvalidID

	geometry.Name = ""

	// Release the material.
	if geometry.Material != nil && len(geometry.Material.Name) > 0 {
		MaterialSystemRelease(geometry.Material.Name)
		geometry.Material = nil
	}
}
