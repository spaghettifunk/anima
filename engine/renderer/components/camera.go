package components

import (
	"github.com/spaghettifunk/anima/engine/math"
)

/**
 * @brief Represents a camera that can be used for
 * a variety of things, especially rendering. Ideally,
 * these are created and managed by the camera system.
 */
type Camera struct {
	/**
	 * @brief The position of this camera.
	 * NOTE: Do not set this directly, use camera_positon_set() instead
	 * so the view matrix is recalculated when needed.
	 */
	Position math.Vec3
	/**
	 * @brief The rotation of this camera using Euler angles (pitch, yaw, roll).
	 * NOTE: Do not set this directly, use camera_rotation_euler_set() instead
	 * so the view matrix is recalculated when needed.
	 */
	EulerRotation math.Vec3
	/** @brief Internal flag used to determine when the view matrix needs to be rebuilt. */
	IsDirty bool
	/**
	 * @brief The view matrix of this camera.
	 * NOTE: IMPORTANT: Do not get this directly, use camera_view_get() instead
	 * so the view matrix is recalculated when needed.
	 */
	ViewMatrix math.Mat4
}

type CameraLookup struct {
	ID             uint16
	ReferenceCount uint16
	Camera         *Camera
}

/** @brief The name of the default camera. */
const DEFAULT_CAMERA_NAME string = "default"

func NewCamera() *Camera {
	camera := &Camera{}
	camera.Reset()
	return camera
}

func (c *Camera) Reset() {
	c.EulerRotation = math.NewVec3Zero()
	c.Position = math.NewVec3Zero()
	c.IsDirty = false
	c.ViewMatrix = math.NewMat4Identity()
}

func (c *Camera) GetPosition() math.Vec3 {
	return c.Position
}

func (c *Camera) SetPosition(position math.Vec3) {
	c.Position = position
	c.IsDirty = true
}

func (c *Camera) GetEulerRotation() math.Vec3 {
	return c.EulerRotation
}

func (c *Camera) SetEulerRotation(rotation math.Vec3) {
	c.EulerRotation = rotation
	c.IsDirty = true
}

func (c *Camera) GetView() math.Mat4 {
	if c.IsDirty {
		rotation := math.NewMat4EulerXYZ(c.EulerRotation.X, c.EulerRotation.Y, c.EulerRotation.Z)
		translation := math.NewMat4Translation(c.Position)

		c.ViewMatrix = rotation.Mul(translation)
		c.ViewMatrix = c.ViewMatrix.Inverse()

		c.IsDirty = false
	}
	return c.ViewMatrix

}

func (c *Camera) Forward() math.Vec3 {
	view := c.GetView()
	return view.Forward()
}

func (c *Camera) Backward() math.Vec3 {
	view := c.GetView()
	return view.Backward()

}

func (c *Camera) Left() math.Vec3 {
	view := c.GetView()
	return view.Left()
}

func (c *Camera) Right() math.Vec3 {
	view := c.GetView()
	return view.Right()
}

func (c *Camera) MoveForward(amount float32) {
	direction := c.Forward()
	direction = direction.MulScalar(amount)
	c.Position = c.Position.Add(direction)
	c.IsDirty = true

}

func (c *Camera) MoveBackward(amount float32) {
	direction := c.Backward()
	direction = direction.MulScalar(amount)
	c.Position = c.Position.Add(direction)
	c.IsDirty = true

}

func (c *Camera) MoveLeft(amount float32) {
	direction := c.Left()
	direction = direction.MulScalar(amount)
	c.Position = c.Position.Add(direction)
	c.IsDirty = true

}

func (c *Camera) MoveRight(amount float32) {
	direction := c.Right()
	direction = direction.MulScalar(amount)
	c.Position = c.Position.Add(direction)
	c.IsDirty = true

}

func (c *Camera) MoveUp(amount float32) {
	direction := math.NewVec3Up()
	direction = direction.MulScalar(amount)
	c.Position = c.Position.Add(direction)
	c.IsDirty = true

}

func (c *Camera) MoveDown(amount float32) {
	direction := math.NewVec3Down()
	direction = direction.MulScalar(amount)
	c.Position = c.Position.Add(direction)
	c.IsDirty = true

}

func (c *Camera) Yaw(amount float32) {
	c.EulerRotation.Y += amount
	c.IsDirty = true

}

func (c *Camera) Pitch(amount float32) {
	c.EulerRotation.X += amount

	// Clamp to avoid Gimbal lock.
	limit := float32(1.55334306) // 89 degrees, or equivalent to deg_to_rad(89.0f);
	c.EulerRotation.X = math.Clamp(c.EulerRotation.X, -limit, limit)

	c.IsDirty = true
}
