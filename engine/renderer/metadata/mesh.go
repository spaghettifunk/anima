package metadata

import (
	"github.com/spaghettifunk/anima/engine/math"
)

// Also used as result_data from job.
type MeshLoadParams struct {
	ResourceName string
	OutMesh      *Mesh
	MeshResource *Resource
}

type Mesh struct {
	UniqueID      uint32
	Generation    uint8
	GeometryCount uint16
	Geometries    []*Geometry
	Transform     *math.Transform
}
