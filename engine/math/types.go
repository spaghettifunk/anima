package math

// Vec2 represents a 2D vector
type Vec2 struct {
	X, Y float32
}

// Vec3 represents a 3D vector
type Vec3 struct {
	X, Y, Z float32
}

// Vec4 represents a 4D vector
type Vec4 struct {
	X, Y, Z, W float32
}

/** @brief A quaternion, used to represent rotational orientation. */
type Quaternion Vec4

/** @brief a 4x4 matrix, typically used to represent object transformations. */
type Mat4 struct {
	/** @brief The matrix elements */
	Data [16]float32
}

/**
 * @brief Represents the extents of a 2d object.
 */
type Extents2D struct {
	/** @brief The minimum extents of the object. */
	Min Vec2
	/** @brief The maximum extents of the object. */
	Max Vec2
}

/**
 * @brief Represents the extents of a 3d object.
 */
type Extents3D struct {
	/** @brief The minimum extents of the object. */
	Min Vec3
	/** @brief The maximum extents of the object. */
	Max Vec3
}

/**
 * @brief Represents a single vertex in 3D space.
 */
type Vertex3D struct {
	/** @brief The position of the vertex */
	Position Vec3
	/** @brief The normal of the vertex. */
	Normal Vec3
	/** @brief The texture coordinate of the vertex. */
	Texcoord Vec2
	/** @brief The colour of the vertex. */
	Colour Vec4
	/** @brief The tangent of the vertex. */
	Tangent Vec3
}

/**
 * @brief Represents a single vertex in 2D space.
 */
type Vertex2D struct {
	/** @brief The position of the vertex */
	Position Vec2
	/** @brief The texture coordinate of the vertex. */
	Texcoord Vec2
}

/**
 * @brief Represents the transform of an object in the world.
 * Transforms can have a parent whose own transform is then
 * taken into account. NOTE: The properties of this should not
 * be edited directly, but done via the functions in transform.h
 * to ensure proper matrix generation.
 */
type Transform struct {
	/** @brief The position in the world. */
	Position Vec3
	/** @brief The rotation in the world. */
	Rotation Quaternion
	/** @brief The scale in the world. */
	Scale Vec3
	/**
	 * @brief Indicates if the position, rotation or scale have changed,
	 * indicating that the local matrix needs to be recalculated.
	 */
	IsDirty bool
	/**
	 * @brief The local transformation matrix, updated whenever
	 * the position, rotation or scale have changed.
	 */
	Local Mat4
	/** @brief A pointer to a parent transform if one is assigned. Can also be null. */
	Parent *Transform
}
