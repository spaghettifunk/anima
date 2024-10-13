package vulkan

/**
 * @brief Max number of material instances
 * @todo TODO: make configurable
 */
const VULKAN_MAX_MATERIAL_COUNT uint32 = 1024

/**
 * @brief Max number of simultaneously uploaded geometries
 * @todo TODO: make configurable
 */
const VULKAN_MAX_GEOMETRY_COUNT uint32 = 4096

/**
 * @brief Internal buffer data for geometry. This data gets loaded
 * directly into a buffer.
 */
type vulkan_geometry_data struct {
	/** @brief The unique geometry identifier. */
	ID uint32
	/** @brief The geometry generation. Incremented every time the geometry data changes. */
	Generation uint32
	/** @brief The vertex count. */
	VertexCount uint32
	/** @brief The size of each vertex. */
	VertexElementSize uint32
	/** @brief The offset in bytes in the vertex buffer. */
	VertexBufferOffset uint64
	/** @brief The index count. */
	IndexCount uint32
	/** @brief The size of each index. */
	IndexElementSize uint32
	/** @brief The offset in bytes in the index buffer. */
	IndexBufferOffset uint64
}

/**
 * @brief Max number of UI control instances
 * @todo TODO: make configurable
 */
const VULKAN_MAX_UI_COUNT uint32 = 1024
