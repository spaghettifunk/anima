package math

import (
	m "math"

	"github.com/spaghettifunk/anima/engine/platform"
	"golang.org/x/exp/rand"
)

const (
	/** @brief An approximate representation of PI. */
	K_PI float32 = 3.14159265358979323846
	/** @brief An approximate representation of PI multiplied by 2. */
	K_PI_2 float32 = 2.0 * K_PI
	/** @brief An approximate representation of PI divided by 2. */
	K_HALF_PI float32 = 0.5 * K_PI
	/** @brief An approximate representation of PI divided by 4. */
	K_QUARTER_PI float32 = 0.25 * K_PI
	/** @brief One divided by an approximate representation of PI. */
	K_ONE_OVER_PI float32 = 1.0 / K_PI
	/** @brief One divided by half of an approximate representation of PI. */
	K_ONE_OVER_TWO_PI float32 = 1.0 / K_PI_2
	/** @brief An approximation of the square root of 2. */
	K_SQRT_TWO float32 = 1.41421356237309504880
	/** @brief An approximation of the square root of 3. */
	K_SQRT_THREE float32 = 1.73205080756887729352
	/** @brief One divided by an approximation of the square root of 2. */
	K_SQRT_ONE_OVER_TWO float32 = 0.70710678118654752440
	/** @brief One divided by an approximation of the square root of 3. */
	K_SQRT_ONE_OVER_THREE float32 = 0.57735026918962576450
	/** @brief A multiplier used to convert degrees to radians. */
	K_DEG2RAD_MULTIPLIER float32 = K_PI / 180.0
	/** @brief A multiplier used to convert radians to degrees. */
	K_RAD2DEG_MULTIPLIER float32 = 180.0 / K_PI
	/** @brief The multiplier to convert seconds to milliseconds. */
	K_SEC_TO_MS_MULTIPLIER float32 = 1000.0
	/** @brief The multiplier to convert milliseconds to seconds. */
	K_MS_TO_SEC_MULTIPLIER float32 = 0.001
	/** @brief A huge number that should be larger than any valid number used. */
	K_INFINITY float32 = 1e30
	/** @brief Smallest positive number where 1.0 + FLOAT_EPSILON != 0 */
	K_FLOAT_EPSILON float32 = 1.192092896e-07
)

var rand_seeded bool = false

/**
 * Note that these are here in order to prevent having to import the
 * entire <math.h> everywhere.
 */
func ksin(x float32) float32 {
	return float32(m.Sin(float64(x)))
}

func kcos(x float32) float32 {
	return float32(m.Cos(float64(x)))
}

func ktan(x float32) float32 {
	return float32(m.Tan(float64(x)))
}

func kacos(x float32) float32 {
	return float32(m.Acos(float64(x)))
}

func ksqrt(x float32) float32 {
	return float32(m.Sqrt(float64(x)))
}

func kabs(x float32) float32 {
	return float32(m.Abs(float64(x)))
}

func krandom() int32 {
	if !rand_seeded {
		rand.Seed(uint64(platform.GetAbsoluteTime()))
		rand_seeded = true
	}
	return rand.Int31()
}

func krandom_in_range(min, max int32) int32 {
	if !rand_seeded {
		rand.Seed(uint64(platform.GetAbsoluteTime()))
		rand_seeded = true
	}
	return (rand.Int31() % (max - min + 1)) + min
}

func fkrandom() float32 {
	return float32(krandom() / rand.Int31())
}

func fkrandom_in_range(min, max float32) float32 {
	return min + fkrandom()*(max-min)
}

// ------------------------------------------
// Vector 2
// ------------------------------------------

/**
 * @brief Creates and returns a new 2-element vector using the supplied values.
 *
 * @param x The x value.
 * @param y The y value.
 * @return A new 2-element vector.
 */
func NewVec2(x, y float32) Vec2 {
	return Vec2{
		X: x,
		Y: y,
	}
}

/**
 * @brief Creates and returns a 2-component vector with all components set to 0.0f.
 */
func NewVec2Zero() Vec2 {
	return Vec2{X: 0.0, Y: 0.0}
}

/**
 * @brief Creates and returns a 2-component vector with all components set to 1.0f.
 */
func NewVec2One() Vec2 {
	return Vec2{1.0, 1.0}
}

/**
 * @brief Creates and returns a 2-component vector pointing up (0, 1).
 */
func NewVec2Up() Vec2 {
	return Vec2{0.0, 1.0}
}

/**
 * @brief Creates and returns a 2-component vector pointing down (0, -1).
 */
func NewVec2Down() Vec2 {
	return Vec2{0.0, -1.0}
}

/**
 * @brief Creates and returns a 2-component vector pointing left (-1, 0).
 */
func NewVec2Left() Vec2 {
	return Vec2{-1.0, 0.0}
}

/**
 * @brief Creates and returns a 2-component vector pointing right (1, 0).
 */
func NewVec2Right() Vec2 {
	return Vec2{1.0, 0.0}
}

/**
 *  Adds other to v and returns a copy of the result.
 */
func (v Vec2) Add(other Vec2) Vec2 {
	return Vec2{v.X + other.X, v.Y + other.Y}
}

/**
 * Subtracts v from other and returns a copy of the result.
 */
func (v Vec2) Sub(other Vec2) Vec2 {
	return Vec2{v.X - other.X, v.Y - other.Y}
}

/**
 *  Multiplies v by other and returns a copy of the result.
 */
func (v Vec2) Mul(other Vec2) Vec2 {
	return Vec2{v.X * other.X, v.Y * other.Y}
}

/**
 * Divides v by other and returns a copy of the result.
 */
func (v Vec2) Div(other Vec2) Vec2 {
	return Vec2{v.X / other.X, v.Y / other.Y}
}

/**
 * Returns the squared length of the provided vector.
 */
func (v Vec2) LengthSquared() float32 {
	return v.X*v.X + v.Y*v.Y
}

/**
 * @brief Returns the length of the provided vector.
 *
 * @param vector The vector to retrieve the length of.
 * @return The length.
 */
func (v Vec2) Length() float32 {
	return ksqrt(v.LengthSquared())
}

/**
 * Normalizes the provided vector in place to a unit vector.
 */
func (v Vec2) Normalize() Vec2 {
	length := v.Length()
	return Vec2{v.X / length, v.Y / length}
}

/**
 * @brief Returns a normalized copy of the supplied vector.
 *
 * @param vector The vector to be normalized.
 * @return A normalized copy of the supplied vector
 */
func (v Vec2) Normalized() Vec2 {
	return v.Normalize()
}

/**
 * @brief Compares all elements of vector_0 and vector_1 and ensures the difference
 * is less than tolerance.
 *
 * @param vector_0 The first vector.
 * @param vector_1 The second vector.
 * @param tolerance The difference tolerance. Typically K_FLOAT_EPSILON or similar.
 * @return True if within tolerance; otherwise false.
 */
func (v Vec2) Compare(other Vec2, tolerance float32) bool {
	if kabs(v.X-other.X) > tolerance {
		return false
	}
	if kabs(v.Y-other.Y) > tolerance {
		return false
	}
	return true
}

/**
 * @brief Returns the distance between vector_0 and vector_1.
 *
 * @param vector_0 The first vector.
 * @param vector_1 The second vector.
 * @return The distance between vector_0 and vector_1.
 */
func (v Vec2) Distance(other Vec2) float32 {
	d := Vec2{
		v.X - other.X,
		v.Y - other.Y}
	return d.Length()
}

// ------------------------------------------
// Vector 3
// ------------------------------------------

/**
 * @brief Creates and returns a new 3-element vector using the supplied values.
 *
 * @param x The x value.
 * @param y The y value.
 * @param z The z value.
 * @return A new 3-element vector.
 */
func NewVec3(x, y, z float32) Vec3 {
	return Vec3{x, y, z}
}

/**
 * @brief Returns a new vec3 containing the x, y and z components of the
 * supplied vec4, essentially dropping the w component.
 *
 * @param vector The 4-component vector to extract from.
 * @return A new vec3
 */
func NewVec3FromVec4(vector Vec4) Vec3 {
	return Vec3{vector.X, vector.Y, vector.Z}
}

/**
 * @brief Returns a new vec4 using vector as the x, y and z components and w for w.
 *
 * @param vector The 3-component vector.
 * @param w The w component.
 * @return A new vec4
 */
func (v Vec3) ToVec4(w float32) Vec4 {
	return Vec4{v.X, v.Y, v.Z, w}
}

/**
 * @brief Creates and returns a 3-component vector with all components set to 0.0f.
 */
func NewVec3Zero() Vec3 {
	return Vec3{0.0, 0.0, 0.0}
}

/**
 * @brief Creates and returns a 3-component vector with all components set to 1.0f.
 */
func NewVec3One() Vec3 {
	return Vec3{1.0, 1.0, 1.0}
}

/**
 * @brief Creates and returns a 3-component vector pointing up (0, 1, 0).
 */
func NewVec3Up() Vec3 {
	return Vec3{0.0, 1.0, 0.0}
}

/**
 * @brief Creates and returns a 3-component vector pointing down (0, -1, 0).
 */
func NewVec3Down() Vec3 {
	return Vec3{0.0, -1.0, 0.0}
}

/**
 * @brief Creates and returns a 3-component vector pointing left (-1, 0, 0).
 */
func NewVec3Left() Vec3 {
	return Vec3{-1.0, 0.0, 0.0}
}

/**
 * @brief Creates and returns a 3-component vector pointing right (1, 0, 0).
 */
func NewVec3Right() Vec3 {
	return Vec3{1.0, 0.0, 0.0}
}

/**
 * @brief Creates and returns a 3-component vector pointing forward (0, 0, -1).
 */
func NewVec3Forward() Vec3 {
	return Vec3{0.0, 0.0, -1.0}
}

/**
 * @brief Creates and returns a 3-component vector pointing backward (0, 0, 1).
 */
func NewVec3Back() Vec3 {
	return Vec3{0.0, 0.0, 1.0}
}

/**
 * @brief Adds vector_1 to vector_0 and returns a copy of the result.
 *
 * @param vector_0 The first vector.
 * @param vector_1 The second vector.
 * @return The resulting vector.
 */
func (v Vec3) Add(other Vec3) Vec3 {
	return Vec3{
		v.X + other.X,
		v.Y + other.Y,
		v.Z + other.Z}
}

/**
 * @brief Subtracts vector_1 from vector_0 and returns a copy of the result.
 *
 * @param vector_0 The first vector.
 * @param vector_1 The second vector.
 * @return The resulting vector.
 */
func (v Vec3) Sub(other Vec3) Vec3 {
	return Vec3{
		v.X - other.X,
		v.Y - other.Y,
		v.Z - other.Z}
}

/**
 * @brief Multiplies vector_0 by vector_1 and returns a copy of the result.
 *
 * @param vector_0 The first vector.
 * @param vector_1 The second vector.
 * @return The resulting vector.
 */
func (v Vec3) Mul(other Vec3) Vec3 {
	return Vec3{
		v.X * other.X,
		v.Y * other.Y,
		v.Z * other.Z}
}

/**
 * @brief Multiplies all elements of vector_0 by scalar and returns a copy of the result.
 *
 * @param vector_0 The vector to be multiplied.
 * @param scalar The scalar value.
 * @return A copy of the resulting vector.
 */
func (v Vec3) MulScalar(scalar float32) Vec3 {
	return Vec3{
		v.X * scalar,
		v.Y * scalar,
		v.Z * scalar}
}

/**
 * @brief Divides vector_0 by vector_1 and returns a copy of the result.
 *
 * @param vector_0 The first vector.
 * @param vector_1 The second vector.
 * @return The resulting vector.
 */
func (v Vec3) Div(other Vec3) Vec3 {
	return Vec3{
		v.X / other.X,
		v.Y / other.Y,
		v.Z / other.Z}
}

/**
 * @brief Returns the squared length of the provided vector.
 *
 * @param vector The vector to retrieve the squared length of.
 * @return The squared length.
 */
func (v Vec3) LengthSquared() float32 {
	return v.X*v.X + v.Y*v.Y + v.Z*v.Z
}

/**
 * @brief Returns the length of the provided vector.
 *
 * @param vector The vector to retrieve the length of.
 * @return The length.
 */
func (v Vec3) Length() float32 {
	return ksqrt(v.LengthSquared())
}

/**
 * @brief Normalizes the provided vector in place to a unit vector.
 *
 * @param vector A pointer to the vector to be normalized.
 */
func (v Vec3) Normalize() Vec3 {
	length := v.Length()
	return Vec3{
		v.X / length,
		v.Y / length,
		v.Z / length}
}

/**
 * @brief Returns a normalized copy of the supplied vector.
 *
 * @param vector The vector to be normalized.
 * @return A normalized copy of the supplied vector
 */
func (v Vec3) Normalized() Vec3 {
	return v.Normalize()
}

/**
 * @brief Returns the dot product between the provided vectors. Typically used
 * to calculate the difference in direction.
 *
 * @param vector_0 The first vector.
 * @param vector_1 The second vector.
 * @return The dot product.
 */
func (v Vec3) Dot(other Vec3) float32 {
	p := float32(0)
	p += v.X * other.X
	p += v.Y * other.Y
	p += v.Z * other.Z
	return p
}

/**
 * @brief Calculates and returns the cross product of the supplied vectors.
 * The cross product is a new vector which is orthoganal to both provided vectors.
 *
 * @param vector_0 The first vector.
 * @param vector_1 The second vector.
 * @return The cross product.
 */
func (v Vec3) Cross(other Vec3) Vec3 {
	return Vec3{
		v.Y*other.Z - v.Z*other.Y,
		v.Z*other.X - v.X*other.Z,
		v.X*other.Y - v.Y*other.X}
}

/**
 * @brief Compares all elements of vector_0 and vector_1 and ensures the difference
 * is less than tolerance.
 *
 * @param vector_0 The first vector.
 * @param vector_1 The second vector.
 * @param tolerance The difference tolerance. Typically K_FLOAT_EPSILON or similar.
 * @return True if within tolerance; otherwise false.
 */
func (v Vec3) Compare(other Vec3, tolerance float32) bool {
	if kabs(v.X-other.X) > tolerance {
		return false
	}

	if kabs(v.Y-other.Y) > tolerance {
		return false
	}

	if kabs(v.Z-other.Z) > tolerance {
		return false
	}

	return true
}

/**
 * @brief Returns the distance between vector_0 and vector_1.
 *
 * @param vector_0 The first vector.
 * @param vector_1 The second vector.
 * @return The distance between vector_0 and vector_1.
 */
func (v Vec3) Distance(other Vec3) float32 {
	d := Vec3{
		v.X - other.X,
		v.Y - other.Y,
		v.Z - other.Z}
	return d.Length()
}

/**
 * @brief Transform v by m. NOTE: It is assumed by this function that the
 * vector v is a point, not a direction, and is calculated as if a w component
 * with a value of 1.0f is there.
 *
 * @param v The vector to transform.
 * @param m The matrix to transform by.
 * @return A transformed copy of v.
 */
func (v Vec3) Transform(m Mat4) Vec3 {
	out := Vec3{}
	out.X = v.X*m.Data[0+0] + v.Y*m.Data[4+0] + v.Z*m.Data[8+0] + 1.0*m.Data[12+0]
	out.Y = v.X*m.Data[0+1] + v.Y*m.Data[4+1] + v.Z*m.Data[8+1] + 1.0*m.Data[12+1]
	out.Z = v.X*m.Data[0+2] + v.Y*m.Data[4+2] + v.Z*m.Data[8+2] + 1.0*m.Data[12+2]
	return out
}

// ------------------------------------------
// Vector 4
// ------------------------------------------

/**
 * @brief Creates and returns a new 4-element vector using the supplied values.
 *
 * @param x The x value.
 * @param y The y value.
 * @param z The z value.
 * @param w The w value.
 * @return A new 4-element vector.
 */
func NewVec4Create(x, y, z, w float32) Vec4 {
	out_vector := Vec4{}
	out_vector.X = x
	out_vector.Y = y
	out_vector.Z = z
	out_vector.W = w
	return out_vector
}

/**
 * @brief Returns a new vec3 containing the x, y and z components of the
 * supplied vec4, essentially dropping the w component.
 *
 * @param vector The 4-component vector to extract from.
 * @return A new vec3
 */
func (v Vec4) ToVec3() Vec3 {
	return Vec3{v.X, v.Y, v.Z}
}

/**
 * @brief Returns a new vec4 using vector as the x, y and z components and w for w.
 *
 * @param vector The 3-component vector.
 * @param w The w component.
 * @return A new vec4
 */
func NewVec4FromVec3(v Vec3, w float32) Vec4 {
	// TODO: SIMD
	// vec4 out_vector;
	// out_vector.Data = _mm_setr_ps(x, y, z, w);
	// return out_vector;

	return Vec4{v.X, v.Y, v.Z, w}

}

/**
 * @brief Creates and returns a 4-component vector with all components set to 0.0f.
 */
func NewVec4Zero() Vec4 {
	return Vec4{0.0, 0.0, 0.0, 0.0}
}

/**
 * @brief Creates and returns a 4-component vector with all components set to 1.0f.
 */
func NewVec4One() Vec4 {
	return Vec4{1.0, 1.0, 1.0, 1.0}
}

/**
 * @brief Adds vector_1 to vector_0 and returns a copy of the result.
 *
 * @param vector_0 The first vector.
 * @param vector_1 The second vector.
 * @return The resulting vector.
 */
func (v Vec4) Add(other Vec4) Vec4 {
	return Vec4{
		X: v.X + other.X,
		Y: v.Y + other.Y,
		Z: v.Z + other.Z,
		W: v.W + other.W,
	}
}

/**
 * @brief Subtracts vector_1 from vector_0 and returns a copy of the result.
 *
 * @param vector_0 The first vector.
 * @param vector_1 The second vector.
 * @return The resulting vector.
 */
func (v Vec4) Sub(other Vec4) Vec4 {
	return Vec4{
		X: v.X - other.X,
		Y: v.Y - other.Y,
		Z: v.Z - other.Z,
		W: v.W - other.W,
	}
}

/**
 * @brief Multiplies vector_0 by vector_1 and returns a copy of the result.
 *
 * @param vector_0 The first vector.
 * @param vector_1 The second vector.
 * @return The resulting vector.
 */
func (v Vec4) Mul(other Vec4) Vec4 {
	return Vec4{
		X: v.X * other.X,
		Y: v.Y * other.Y,
		Z: v.Z * other.Z,
		W: v.W * other.W,
	}
}

/**
 * @brief Divides vector_0 by vector_1 and returns a copy of the result.
 *
 * @param vector_0 The first vector.
 * @param vector_1 The second vector.
 * @return The resulting vector.
 */
func (v Vec4) Div(other Vec4) Vec4 {
	return Vec4{
		X: v.X / other.X,
		Y: v.Y / other.Y,
		Z: v.Z / other.Z,
		W: v.W / other.W,
	}
}

/**
 * @brief Returns the squared length of the provided vector.
 *
 * @param vector The vector to retrieve the squared length of.
 * @return The squared length.
 */
func (v Vec4) LengthSquared() float32 {
	return v.X*v.X + v.Y*v.Y + v.Z*v.Z + v.W*v.W
}

/**
 * @brief Returns the length of the provided vector.
 *
 * @param vector The vector to retrieve the length of.
 * @return The length.
 */
func (v Vec4) Length() float32 {
	return ksqrt(v.LengthSquared())
}

/**
 * @brief Normalizes the provided vector in place to a unit vector.
 *
 * @param vector A pointer to the vector to be normalized.
 */
func (v Vec4) Normalize() Vec4 {
	length := v.Length()
	return Vec4{
		v.X / length,
		v.Y / length,
		v.Z / length,
		v.W / length}
}

/**
 * @brief Returns a normalized copy of the supplied vector.
 *
 * @param vector The vector to be normalized.
 * @return A normalized copy of the supplied vector
 */
func (v Vec4) Normalized() Vec4 {
	return v.Normalize()
}

/**
 * @brief Calculates the dot product using the elements of vec4s provided in split-out format.
 *
 * @param a0 The first element of the a vector.
 * @param a1 The second element of the a vector.
 * @param a2 The third element of the a vector.
 * @param a3 The fourth element of the a vector.
 * @param b0 The first element of the b vector.
 * @param b1 The second element of the b vector.
 * @param b2 The third element of the b vector.
 * @param b3 The fourth element of the b vector.
 * @return The dot product of vectors and b.
 */
func Vec4DotFloat32(a0, a1, a2, a3, b0, b1, b2, b3 float32) float32 {
	p := a0*b0 + a1*b1 + a2*b2 + a3*b3
	return p
}

/**
 * @brief Compares all elements of vector_0 and vector_1 and ensures the difference
 * is less than tolerance.
 *
 * @param vector_0 The first vector.
 * @param vector_1 The second vector.
 * @param tolerance The difference tolerance. Typically K_FLOAT_EPSILON or similar.
 * @return True if within tolerance; otherwise false.
 */
func (v Vec4) Compare(other Vec4, tolerance float32) bool {
	if kabs(v.X-other.X) > tolerance {
		return false
	}

	if kabs(v.Y-other.Y) > tolerance {
		return false
	}

	if kabs(v.Z-other.Z) > tolerance {
		return false
	}

	if kabs(v.W-other.W) > tolerance {
		return false
	}

	return true
}

/**
 * @brief Creates and returns an identity matrix:
 *
 * {
 *   {1, 0, 0, 0},
 *   {0, 1, 0, 0},
 *   {0, 0, 1, 0},
 *   {0, 0, 0, 1}
 * }
 *
 * @return A new identity matrix
 */
func NewMat4Identity() Mat4 {
	out_matrix := Mat4{}
	// kzero_memory(out_matrix.Data, sizeof(f32) * 16);
	out_matrix.Data[0] = 1.0
	out_matrix.Data[5] = 1.0
	out_matrix.Data[10] = 1.0
	out_matrix.Data[15] = 1.0
	return out_matrix
}

/**
 * @brief Returns the result of multiplying matrix_0 and matrix_1.
 *
 * @param matrix_0 The first matrix to be multiplied.
 * @param matrix_1 The second matrix to be multiplied.
 * @return The result of the matrix multiplication.
 */
func (mt Mat4) Mul(other Mat4) Mat4 {
	out_matrix := NewMat4Identity()

	for row := 0; row < 4; row++ {
		for col := 0; col < 4; col++ {
			sum := float32(0)
			for i := 0; i < 4; i++ {
				sum += mt.Data[row*4+i] * other.Data[i*4+col]
			}
			out_matrix.Data[row*4+col] = sum
		}
	}

	return out_matrix
}

/**
 * @brief Creates and returns an orthographic projection matrix. Typically used to
 * render flat or 2D scenes.
 *
 * @param left The left side of the view frustum.
 * @param right The right side of the view frustum.
 * @param bottom The bottom side of the view frustum.
 * @param top The top side of the view frustum.
 * @param near_clip The near clipping plane distance.
 * @param far_clip The far clipping plane distance.
 * @return A new orthographic projection matrix.
 */
func NewMat4Orthographic(left, right, bottom, top, near_clip, far_clip float32) Mat4 {
	out_matrix := NewMat4Identity()

	lr := 1.0 / (left - right)
	bt := 1.0 / (bottom - top)
	nf := 1.0 / (near_clip - far_clip)

	out_matrix.Data[0] = -2.0 * lr
	out_matrix.Data[5] = -2.0 * bt
	out_matrix.Data[10] = 2.0 * nf

	out_matrix.Data[12] = (left + right) * lr
	out_matrix.Data[13] = (top + bottom) * bt
	out_matrix.Data[14] = (far_clip + near_clip) * nf
	return out_matrix
}

/**
 * @brief Creates and returns a perspective matrix. Typically used to render 3d scenes.
 *
 * @param fov_radians The field of view in radians.
 * @param aspect_ratio The aspect ratio.
 * @param near_clip The near clipping plane distance.
 * @param far_clip The far clipping plane distance.
 * @return A new perspective matrix.
 */
func NewMat4Perspective(fov_radians, aspect_ratio, near_clip, far_clip float32) Mat4 {
	half_tan_fov := ktan(fov_radians * 0.5)
	out_matrix := Mat4{}
	out_matrix.Data[0] = 1.0 / (aspect_ratio * half_tan_fov)
	out_matrix.Data[5] = 1.0 / half_tan_fov
	out_matrix.Data[10] = -((far_clip + near_clip) / (far_clip - near_clip))
	out_matrix.Data[11] = -1.0
	out_matrix.Data[14] = -((2.0 * far_clip * near_clip) / (far_clip - near_clip))
	return out_matrix
}

/**
 * @brief Creates and returns a look-at matrix, or a matrix looking
 * at target from the perspective of position.
 *
 * @param position The position of the matrix.
 * @param target The position to "look at".
 * @param up The up vector.
 * @return A matrix looking at target from the perspective of position.
 */
func NewMat4LookAt(position, target, up Vec3) Mat4 {
	out_matrix := Mat4{}
	z_axis := Vec3{}
	z_axis.X = target.X - position.X
	z_axis.Y = target.Y - position.Y
	z_axis.Z = target.Z - position.Z

	z_axis.Normalize()
	x_axis := up.Cross(z_axis)
	x_axis.Normalize()
	y_axis := z_axis.Cross(x_axis)

	out_matrix.Data[0] = x_axis.X
	out_matrix.Data[1] = y_axis.X
	out_matrix.Data[2] = -z_axis.X
	out_matrix.Data[3] = 0
	out_matrix.Data[4] = x_axis.Y
	out_matrix.Data[5] = y_axis.Y
	out_matrix.Data[6] = -z_axis.Y
	out_matrix.Data[7] = 0
	out_matrix.Data[8] = x_axis.Z
	out_matrix.Data[9] = y_axis.Z
	out_matrix.Data[10] = -z_axis.Z
	out_matrix.Data[11] = 0
	out_matrix.Data[12] = -x_axis.Dot(position)
	out_matrix.Data[13] = -y_axis.Dot(position)
	out_matrix.Data[14] = z_axis.Dot(position)
	out_matrix.Data[15] = 1.0

	return out_matrix
}

/**
 * @brief Returns a transposed copy of the provided matrix (rows->colums)
 *
 * @param matrix The matrix to be transposed.
 * @return A transposed copy of of the provided matrix.
 */
func NewMat4Transposed(matrix Mat4) Mat4 {
	out_matrix := NewMat4Identity()
	out_matrix.Data[0] = matrix.Data[0]
	out_matrix.Data[1] = matrix.Data[4]
	out_matrix.Data[2] = matrix.Data[8]
	out_matrix.Data[3] = matrix.Data[12]
	out_matrix.Data[4] = matrix.Data[1]
	out_matrix.Data[5] = matrix.Data[5]
	out_matrix.Data[6] = matrix.Data[9]
	out_matrix.Data[7] = matrix.Data[13]
	out_matrix.Data[8] = matrix.Data[2]
	out_matrix.Data[9] = matrix.Data[6]
	out_matrix.Data[10] = matrix.Data[10]
	out_matrix.Data[11] = matrix.Data[14]
	out_matrix.Data[12] = matrix.Data[3]
	out_matrix.Data[13] = matrix.Data[7]
	out_matrix.Data[14] = matrix.Data[11]
	out_matrix.Data[15] = matrix.Data[15]
	return out_matrix
}

/**
 * @brief Creates and returns an inverse of the provided matrix.
 *
 * @param matrix The matrix to be inverted.
 * @return A inverted copy of the provided matrix.
 */
func (mt Mat4) Inverse() Mat4 {
	m := mt.Data

	t0 := m[10] * m[15]
	t1 := m[14] * m[11]
	t2 := m[6] * m[15]
	t3 := m[14] * m[7]
	t4 := m[6] * m[11]
	t5 := m[10] * m[7]
	t6 := m[2] * m[15]
	t7 := m[14] * m[3]
	t8 := m[2] * m[11]
	t9 := m[10] * m[3]
	t10 := m[2] * m[7]
	t11 := m[6] * m[3]
	t12 := m[8] * m[13]
	t13 := m[12] * m[9]
	t14 := m[4] * m[13]
	t15 := m[12] * m[5]
	t16 := m[4] * m[9]
	t17 := m[8] * m[5]
	t18 := m[0] * m[13]
	t19 := m[12] * m[1]
	t20 := m[0] * m[9]
	t21 := m[8] * m[1]
	t22 := m[0] * m[5]
	t23 := m[4] * m[1]

	out_matrix := Mat4{}
	o := out_matrix.Data

	o[0] = (t0*m[5] + t3*m[9] + t4*m[13]) - (t1*m[5] + t2*m[9] + t5*m[13])
	o[1] = (t1*m[1] + t6*m[9] + t9*m[13]) - (t0*m[1] + t7*m[9] + t8*m[13])
	o[2] = (t2*m[1] + t7*m[5] + t10*m[13]) - (t3*m[1] + t6*m[5] + t11*m[13])
	o[3] = (t5*m[1] + t8*m[5] + t11*m[9]) - (t4*m[1] + t9*m[5] + t10*m[9])

	d := 1.0 / (m[0]*o[0] + m[4]*o[1] + m[8]*o[2] + m[12]*o[3])

	o[0] = d * o[0]
	o[1] = d * o[1]
	o[2] = d * o[2]
	o[3] = d * o[3]
	o[4] = d * ((t1*m[4] + t2*m[8] + t5*m[12]) - (t0*m[4] + t3*m[8] + t4*m[12]))
	o[5] = d * ((t0*m[0] + t7*m[8] + t8*m[12]) - (t1*m[0] + t6*m[8] + t9*m[12]))
	o[6] = d * ((t3*m[0] + t6*m[4] + t11*m[12]) - (t2*m[0] + t7*m[4] + t10*m[12]))
	o[7] = d * ((t4*m[0] + t9*m[4] + t10*m[8]) - (t5*m[0] + t8*m[4] + t11*m[8]))
	o[8] = d * ((t12*m[7] + t15*m[11] + t16*m[15]) - (t13*m[7] + t14*m[11] + t17*m[15]))
	o[9] = d * ((t13*m[3] + t18*m[11] + t21*m[15]) - (t12*m[3] + t19*m[11] + t20*m[15]))
	o[10] = d * ((t14*m[3] + t19*m[7] + t22*m[15]) - (t15*m[3] + t18*m[7] + t23*m[15]))
	o[11] = d * ((t17*m[3] + t20*m[7] + t23*m[11]) - (t16*m[3] + t21*m[7] + t22*m[11]))
	o[12] = d * ((t14*m[10] + t17*m[14] + t13*m[6]) - (t16*m[14] + t12*m[6] + t15*m[10]))
	o[13] = d * ((t20*m[14] + t12*m[2] + t19*m[10]) - (t18*m[10] + t21*m[14] + t13*m[2]))
	o[14] = d * ((t18*m[6] + t23*m[14] + t15*m[2]) - (t22*m[14] + t14*m[2] + t19*m[6]))
	o[15] = d * ((t22*m[10] + t16*m[2] + t21*m[6]) - (t20*m[6] + t23*m[10] + t17*m[2]))

	return out_matrix
}

/**
 * @brief Creates and returns a translation matrix from the given position.
 *
 * @param position The position to be used to create the matrix.
 * @return A newly created translation matrix.
 */
func NewMat4Translation(position Vec3) Mat4 {
	out_matrix := NewMat4Identity()
	out_matrix.Data[12] = position.X
	out_matrix.Data[13] = position.Y
	out_matrix.Data[14] = position.Z
	return out_matrix
}

/**
 * @brief Returns a scale matrix using the provided scale.
 *
 * @param scale The 3-component scale.
 * @return A scale matrix.
 */
func NewMat4Scale(scale Vec3) Mat4 {
	out_matrix := NewMat4Identity()
	out_matrix.Data[0] = scale.X
	out_matrix.Data[5] = scale.Y
	out_matrix.Data[10] = scale.Z
	return out_matrix
}

/**
 * @brief Creates a rotation matrix from the provided x angle.
 *
 * @param angle_radians The x angle in radians.
 * @return A rotation matrix.
 */
func NewMat4EulerX(angle_radians float32) Mat4 {
	out_matrix := NewMat4Identity()
	c := kcos(angle_radians)
	s := ksin(angle_radians)

	out_matrix.Data[5] = c
	out_matrix.Data[6] = s
	out_matrix.Data[9] = -s
	out_matrix.Data[10] = c
	return out_matrix
}

/**
 * @brief Creates a rotation matrix from the provided y angle.
 *
 * @param angle_radians The y angle in radians.
 * @return A rotation matrix.
 */
func NewMat4EulerY(angle_radians float32) Mat4 {
	out_matrix := NewMat4Identity()
	c := kcos(angle_radians)
	s := ksin(angle_radians)

	out_matrix.Data[0] = c
	out_matrix.Data[2] = -s
	out_matrix.Data[8] = s
	out_matrix.Data[10] = c
	return out_matrix
}

/**
 * @brief Creates a rotation matrix from the provided z angle.
 *
 * @param angle_radians The z angle in radians.
 * @return A rotation matrix.
 */
func NewMat4EulerZ(angle_radians float32) Mat4 {
	out_matrix := NewMat4Identity()

	c := kcos(angle_radians)
	s := ksin(angle_radians)

	out_matrix.Data[0] = c
	out_matrix.Data[1] = s
	out_matrix.Data[4] = -s
	out_matrix.Data[5] = c
	return out_matrix
}

/**
 * @brief Creates a rotation matrix from the provided x, y and z axis rotations.
 *
 * @param x_radians The x rotation.
 * @param y_radians The y rotation.
 * @param z_radians The z rotation.
 * @return A rotation matrix.
 */
func NewMat4EulerXYZ(x_radians, y_radians, z_radians float32) Mat4 {
	rx := NewMat4EulerX(x_radians)
	ry := NewMat4EulerY(y_radians)
	rz := NewMat4EulerZ(z_radians)
	out_matrix := rx.Mul(ry)
	out_matrix = out_matrix.Mul(rz)
	return out_matrix
}

/**
 * @brief Returns a forward vector relative to the provided matrix.
 *
 * @param matrix The matrix from which to base the vector.
 * @return A 3-component directional vector.
 */
func (mt Mat4) Forward() Vec3 {
	forward := Vec3{}
	forward.X = -mt.Data[2]
	forward.Y = -mt.Data[6]
	forward.Z = -mt.Data[10]
	forward.Normalize()
	return forward
}

/**
 * @brief Returns a backward vector relative to the provided matrix.
 *
 * @param matrix The matrix from which to base the vector.
 * @return A 3-component directional vector.
 */
func (mt Mat4) Backward() Vec3 {
	backward := Vec3{}
	backward.X = mt.Data[2]
	backward.Y = mt.Data[6]
	backward.Z = mt.Data[10]
	backward.Normalize()
	return backward
}

/**
 * @brief Returns a upward vector relative to the provided matrix.
 *
 * @param matrix The matrix from which to base the vector.
 * @return A 3-component directional vector.
 */
func (mt Mat4) Up() Vec3 {
	up := Vec3{}
	up.X = mt.Data[1]
	up.Y = mt.Data[5]
	up.Z = mt.Data[9]
	up.Normalize()
	return up
}

/**
 * @brief Returns a downward vector relative to the provided matrix.
 *
 * @param matrix The matrix from which to base the vector.
 * @return A 3-component directional vector.
 */
func (mt Mat4) Down() Vec3 {
	down := Vec3{}
	down.X = -mt.Data[1]
	down.Y = -mt.Data[5]
	down.Z = -mt.Data[9]
	down.Normalize()
	return down
}

/**
 * @brief Returns a left vector relative to the provided matrix.
 *
 * @param matrix The matrix from which to base the vector.
 * @return A 3-component directional vector.
 */
func (mt Mat4) Left() Vec3 {
	left := Vec3{}
	left.X = -mt.Data[0]
	left.Y = -mt.Data[4]
	left.Z = -mt.Data[8]
	left.Normalize()
	return left
}

/**
 * @brief Returns a right vector relative to the provided matrix.
 *
 * @param matrix The matrix from which to base the vector.
 * @return A 3-component directional vector.
 */
func (mt Mat4) Right() Vec3 {
	right := Vec3{}
	right.X = mt.Data[0]
	right.Y = mt.Data[4]
	right.Z = mt.Data[8]
	right.Normalize()
	return right
}

// // ------------------------------------------
// // Quaternion
// // ------------------------------------------

/**
 * @brief Creates an identity quaternion.
 *
 * @return An identity quaternion.
 */
func NewQuatIdentity() Quaternion {
	return Quaternion{0, 0, 0, 1.0}
}

/**
 * @brief Returns the normal of the provided quaternion.
 *
 * @param q The quaternion.
 * @return The normal of the provided quaternion.
 */
func (q Quaternion) Normal() float32 {
	return ksqrt(
		q.X*q.X +
			q.Y*q.Y +
			q.Z*q.Z +
			q.W*q.W)
}

/**
 * @brief Returns a normalized copy of the provided quaternion.
 *
 * @param q The quaternion to normalize.
 * @return A normalized copy of the provided quaternion.
 */
func (q Quaternion) Normalize() Quaternion {
	normal := q.Normal()
	return Quaternion{
		q.X / normal,
		q.Y / normal,
		q.Z / normal,
		q.W / normal}
}

/**
 * @brief Returns the conjugate of the provided quaternion. That is,
 * The x, y and z elements are negated, but the w element is untouched.
 *
 * @param q The quaternion to obtain a conjugate of.
 * @return The conjugate quaternion.
 */
func (q Quaternion) Conjugate() Quaternion {
	return Quaternion{-q.X, -q.Y, -q.Z, q.W}
}

/**
 * @brief Returns an inverse copy of the provided quaternion.
 *
 * @param q The quaternion to invert.
 * @return An inverse copy of the provided quaternion.
 */
func (q Quaternion) Inverse() Quaternion {
	c := q.Conjugate()
	return c.Normalize()
}

/**
 * @brief Multiplies the provided quaternions.
 *
 * @param q_0 The first quaternion.
 * @param q_1 The second quaternion.
 * @return The multiplied quaternion.
 */
func (q Quaternion) Mul(other Quaternion) Quaternion {
	out_quaternion := Quaternion{}

	out_quaternion.X = q.X*other.W +
		q.Y*other.Z -
		q.Z*other.Y +
		q.W*other.X

	out_quaternion.Y = -q.X*other.Z +
		q.Y*other.W +
		q.Z*other.X +
		q.W*other.Y

	out_quaternion.Z = q.X*other.Y -
		q.Y*other.X +
		q.Z*other.W +
		q.W*other.Z

	out_quaternion.W = -q.X*other.X -
		q.Y*other.Y -
		q.Z*other.Z +
		q.W*other.W

	return out_quaternion
}

/**
 * @brief Calculates the dot product of the provided quaternions.
 *
 * @param q_0 The first quaternion.
 * @param q_1 The second quaternion.
 * @return The dot product of the provided quaternions.
 */
func (q Quaternion) Dot(other Quaternion) float32 {
	return q.X*other.X +
		q.Y*other.Y +
		q.Z*other.Z +
		q.W*other.W
}

/**
 * @brief Creates a rotation matrix from the given quaternion.
 *
 * @param q The quaternion to be used.
 * @return A rotation matrix.
 */
func (q Quaternion) ToMat4() Mat4 {
	out_matrix := NewMat4Identity()

	// https://stackoverflow.com/questions/1556260/convert-quaternion-rotation-to-rotation-matrix

	q.Normalize()

	n := q

	out_matrix.Data[0] = 1.0 - 2.0*n.Y*n.Y - 2.0*n.Z*n.Z
	out_matrix.Data[1] = 2.0*n.X*n.Y - 2.0*n.Z*n.W
	out_matrix.Data[2] = 2.0*n.X*n.Z + 2.0*n.Y*n.W

	out_matrix.Data[4] = 2.0*n.X*n.Y + 2.0*n.Z*n.W
	out_matrix.Data[5] = 1.0 - 2.0*n.X*n.X - 2.0*n.Z*n.Z
	out_matrix.Data[6] = 2.0*n.Y*n.Z - 2.0*n.X*n.W

	out_matrix.Data[8] = 2.0*n.X*n.Z - 2.0*n.Y*n.W
	out_matrix.Data[9] = 2.0*n.Y*n.Z + 2.0*n.X*n.W
	out_matrix.Data[10] = 1.0 - 2.0*n.X*n.X - 2.0*n.Y*n.Y

	return out_matrix
}

/**
 * @brief Calculates a rotation matrix based on the quaternion and the passed in center point.
 *
 * @param q The quaternion.
 * @param center The center point.
 * @return A rotation matrix.
 */
func (q Quaternion) ToRotationMatrix(center Vec3) Mat4 {
	out_matrix := Mat4{}

	o := out_matrix.Data
	o[0] = (q.X * q.X) - (q.Y * q.Y) - (q.Z * q.Z) + (q.W * q.W)
	o[1] = 2. * ((q.X * q.Y) + (q.Z * q.W))
	o[2] = 2. * ((q.X * q.Z) - (q.Y * q.W))
	o[3] = center.X - center.X*o[0] - center.Y*o[1] - center.Z*o[2]

	o[4] = 2. * ((q.X * q.Y) - (q.Z * q.W))
	o[5] = -(q.X * q.X) + (q.Y * q.Y) - (q.Z * q.Z) + (q.W * q.W)
	o[6] = 2. * ((q.Y * q.Z) + (q.X * q.W))
	o[7] = center.Y - center.X*o[4] - center.Y*o[5] - center.Z*o[6]

	o[8] = 2. * ((q.X * q.Z) + (q.Y * q.W))
	o[9] = 2. * ((q.Y * q.Z) - (q.X * q.W))
	o[10] = -(q.X * q.X) - (q.Y * q.Y) + (q.Z * q.Z) + (q.W * q.W)
	o[11] = center.Z - center.X*o[8] - center.Y*o[9] - center.Z*o[10]

	o[12] = 0.
	o[13] = 0.
	o[14] = 0.
	o[15] = 1.
	return out_matrix
}

/**
 * @brief Creates a quaternion from the given axis and angle.
 *
 * @param axis The axis of rotation.
 * @param angle The angle of rotation.
 * @param normalize Indicates if the quaternion should be normalized.
 * @return A new quaternion.
 */
func NewQuatFromAxisAngle(axis Vec3, angle float32, normalize bool) Quaternion {
	half_angle := 0.5 * angle
	s := ksin(half_angle)
	c := kcos(half_angle)

	q := Quaternion{s * axis.X, s * axis.Y, s * axis.Z, c}
	if normalize {
		q.Normalize()
	}
	return q
}

/**
 * @brief Calculates spherical linear interpolation of a given percentage
 * between two quaternions.
 *
 * @param q_0 The first quaternion.
 * @param q_1 The second quaternion.
 * @param percentage The percentage of interpolation, typically a value from 0.0f-1.0f.
 * @return An interpolated quaternion.
 */
func (q Quaternion) Slerp(other Quaternion, percentage float32) Quaternion {
	// Source: https://en.Wikipedia.org/wiki/Slerp
	// Only unit quaternions are valid rotations.
	// Normalize to avoid undefined behavior.
	v0 := q.Normalize()
	v1 := other.Normalize()

	// Compute the cosine of the angle between the two vectors.
	dot := v0.Dot(v1)

	// If the dot product is negative, slerp won't take
	// the shorter path. Note that v1 and -v1 are equivalent when
	// the negation is applied to all four components. Fix by
	// reversing one quaternion.
	if dot < 0.0 {
		v1.X = -v1.X
		v1.Y = -v1.Y
		v1.Z = -v1.Z
		v1.W = -v1.W
		dot = -dot
	}

	DOT_THRESHOLD := float32(0.9995)
	if dot > DOT_THRESHOLD {
		// If the inputs are too close for comfort, linearly interpolate
		// and normalize the result.
		qt := Quaternion{
			v0.X + ((v1.X - v0.X) * percentage),
			v0.Y + ((v1.Y - v0.Y) * percentage),
			v0.Z + ((v1.Z - v0.Z) * percentage),
			v0.W + ((v1.W - v0.W) * percentage)}

		return qt.Normalize()
	}

	// Since dot is in range [0, DOT_THRESHOLD], acos is safe
	theta_0 := kacos(dot)         // theta_0 = angle between input vectors
	theta := theta_0 * percentage // theta = angle between v0 and result
	sin_theta := ksin(theta)      // compute this value only once
	sin_theta_0 := ksin(theta_0)  // compute this value only once

	s0 := kcos(theta) - dot*sin_theta/sin_theta_0 // == sin(theta_0 - theta) / sin(theta_0)
	s1 := sin_theta / sin_theta_0

	return Quaternion{
		(v0.X * s0) + (v1.X * s1),
		(v0.Y * s0) + (v1.Y * s1),
		(v0.Z * s0) + (v1.Z * s1),
		(v0.W * s0) + (v1.W * s1)}
}

/**
 * @brief Converts provided degrees to radians.
 *
 * @param degrees The degrees to be converted.
 * @return The amount in radians.
 */
func DegToRad(degrees float32) float32 {
	return degrees * K_DEG2RAD_MULTIPLIER
}

/**
 * @brief Converts provided radians to degrees.
 *
 * @param radians The radians to be converted.
 * @return The amount in degrees.
 */
func RadToDeg(radians float32) float32 {
	return radians * K_RAD2DEG_MULTIPLIER
}
