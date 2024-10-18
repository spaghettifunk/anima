package metadata

/**
 * @brief A structure to hold image resource data.
 */
type ImageResourceData struct {
	/** @brief The number of channels. */
	ChannelCount uint8
	/** @brief The width of the image. */
	Width uint32
	/** @brief The height of the image. */
	Height uint32
	/** @brief The pixel data of the image. */
	Pixels []uint8
}

/** @brief Parameters used when loading an image. */
type ImageResourceParams struct {
	/** @brief Indicates if the image should be flipped on the y-axis when loaded. */
	FlipY bool
}
