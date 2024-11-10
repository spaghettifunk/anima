package metadata

import "fmt"

/** @brief Determines face culling mode during rendering. */
type FaceCullMode int

const (
	/** @brief No faces are culled. */
	FaceCullModeNone FaceCullMode = 0x0
	/** @brief Only front faces are culled. */
	FaceCullModeFront FaceCullMode = 0x1
	/** @brief Only back faces are culled. */
	FaceCullModeBack FaceCullMode = 0x2
	/** @brief Both front and back faces are culled. */
	FaceCullModeFrontAndBack FaceCullMode = 0x3
)

func CullModeFromString(s string) (FaceCullMode, error) {
	if s == "front" {
		return FaceCullModeFront, nil
	}
	if s == "back" {
		return FaceCullModeBack, nil
	}
	if s == "front_and_back" {
		return FaceCullModeFrontAndBack, nil
	}
	if s == "none" {
		return FaceCullModeNone, nil
	}
	return 0, fmt.Errorf("string %s is not a valid face cull mode", s)
}
