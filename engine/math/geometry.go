package math

import "github.com/spaghettifunk/anima/engine/core"

func geometry_generate_normals(vertex_count uint32, vertices []Vertex3D, index_count uint32, indices []uint32) {
	for i := uint32(0); i < index_count; i += 3 {
		i0 := indices[i+0]
		i1 := indices[i+1]
		i2 := indices[i+2]

		edge1 := vertices[i1].Position.Sub(vertices[i0].Position)
		edge2 := vertices[i2].Position.Sub(vertices[i0].Position)

		c := edge1.Cross(edge2)
		normal := c.Normalized()

		// NOTE: This just generates a face normal. Smoothing out should be done in a separate pass if desired.
		vertices[i0].Normal = normal
		vertices[i1].Normal = normal
		vertices[i2].Normal = normal
	}
}

func geometry_generate_tangents(vertex_count uint32, vertices []Vertex3D, index_count uint32, indices []uint32) {
	for i := uint32(0); i < index_count; i += 3 {
		i0 := indices[i+0]
		i1 := indices[i+1]
		i2 := indices[i+2]

		edge1 := vertices[i1].Position.Sub(vertices[i0].Position)
		edge2 := vertices[i2].Position.Sub(vertices[i0].Position)

		deltaU1 := vertices[i1].Texcoord.X - vertices[i0].Texcoord.X
		deltaV1 := vertices[i1].Texcoord.Y - vertices[i0].Texcoord.Y

		deltaU2 := vertices[i2].Texcoord.X - vertices[i0].Texcoord.X
		deltaV2 := vertices[i2].Texcoord.Y - vertices[i0].Texcoord.Y

		dividend := (deltaU1*deltaV2 - deltaU2*deltaV1)
		fc := 1.0 / dividend

		tangent := Vec3{
			(fc * (deltaV2*edge1.X - deltaV1*edge2.X)),
			(fc * (deltaV2*edge1.Y - deltaV1*edge2.Y)),
			(fc * (deltaV2*edge1.Z - deltaV1*edge2.Z))}

		tangent = tangent.Normalized()

		sx := deltaU1
		sy := deltaU2
		tx := deltaV1
		ty := deltaV2

		handedness := 1.0
		if (tx*sy - ty*sx) < 0.0 {
			handedness = -1.0
		}

		t4 := tangent.MulScalar(float32(handedness))
		vertices[i0].Tangent = t4
		vertices[i1].Tangent = t4
		vertices[i2].Tangent = t4
	}
}

func vertex3d_equal(vert_0 Vertex3D, vert_1 Vertex3D) bool {
	return vert_0.Position.Compare(vert_1.Position, K_FLOAT_EPSILON) &&
		vert_0.Normal.Compare(vert_1.Normal, K_FLOAT_EPSILON) &&
		vert_0.Texcoord.Compare(vert_1.Texcoord, K_FLOAT_EPSILON) &&
		vert_0.Colour.Compare(vert_1.Colour, K_FLOAT_EPSILON) &&
		vert_0.Tangent.Compare(vert_1.Tangent, K_FLOAT_EPSILON)
}

func reassignIndex(index_count uint32, indices []uint32, from uint32, to uint32) {
	for i := uint32(0); i < index_count; i++ {
		if indices[i] == from {
			indices[i] = to
		} else if indices[i] > from {
			// Pull in all indicies higher than 'from' by 1.
			indices[i]--
		}
	}
}

// Go version of the geometry_deduplicate_vertices function
func GeometryDeduplicateVertices(vertexCount uint32, vertices []Vertex3D, indexCount uint32, indices []uint32) (uint32, []Vertex3D) {
	// Create a new slice for unique vertices
	uniqueVerts := make([]Vertex3D, vertexCount)
	outVertexCount := uint32(0)

	foundCount := uint32(0)

	for v := uint32(0); v < vertexCount; v++ {
		found := false
		for u := uint32(0); u < outVertexCount; u++ {
			if vertices[v] == uniqueVerts[u] {
				// Reassign indices, do not copy
				reassignIndex(indexCount, indices, v-foundCount, u)
				found = true
				foundCount++
				break
			}
		}

		if !found {
			// Copy over to unique
			uniqueVerts[outVertexCount] = vertices[v]
			outVertexCount++
		}
	}

	// Allocate new vertices array for the final result
	outVertices := make([]Vertex3D, outVertexCount)
	// Copy over unique vertices
	outVertices = uniqueVerts[:outVertexCount]

	removedCount := vertexCount - outVertexCount
	core.LogDebug("geometry_deduplicate_vertices: removed %d vertices, orig/now %d/%d.\n", removedCount, vertexCount, outVertexCount)

	return outVertexCount, outVertices
}
