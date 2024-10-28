package metadata

import (
	"github.com/spaghettifunk/anima/engine/math"
)

/** @brief The name of the default geometry. */
const DefaultGeometryName string = "default"

/**
 * @brief Represents the configuration for a geometry.
 */
type GeometryConfig struct {
	/** @brief The size of each vertex. */
	VertexSize uint32
	/** @brief The number of vertices. */
	VertexCount uint32
	/** @brief An array of Vertices. */
	Vertices []math.Vertex3D
	/** @brief The size of each index. */
	IndexSize uint32
	/** @brief The number of indices. */
	IndexCount uint32
	/** @brief An array of Indices. */
	Indices []uint32

	Center     math.Vec3
	MinExtents math.Vec3
	MaxExtents math.Vec3

	/** @brief The Name of the geometry. */
	Name string
	/** @brief The name of the material used by the geometry. */
	MaterialName string
}

type GeometryReference struct {
	ReferenceCount uint64
	Geometry       *Geometry
	AutoRelease    bool
}

/**
 * @brief Represents actual geometry in the world.
 * Typically (but not always, depending on use) paired with a material.
 */
type Geometry struct {
	/** @brief The geometry identifier. */
	ID uint32
	/** @brief The internal geometry identifier, used by the renderer backend to map to internal resources. */
	InternalID uint32
	/** @brief The geometry generation. Incremented every time the geometry changes. */
	Generation uint16
	/** @brief The center of the geometry in local coordinates. */
	Center math.Vec3
	/** @brief The extents of the geometry in local coordinates. */
	Extents math.Extents3D
	/** @brief The geometry name. */
	Name string
	/** @brief A pointer to the material associated with this geometry.. */
	Material *Material
}
