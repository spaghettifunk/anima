package math

func TransformCreate() Transform {
	var t Transform
	transform_set_position_rotation_scale(&t, vec3_zero(), quat_identity(), vec3_one())
	t.Local = mat4_identity()
	t.Parent = nil
	return t
}

func transform_from_position(position Vec3) Transform {
	var t Transform
	transform_set_position_rotation_scale(&t, position, quat_identity(), vec3_one())
	t.Local = mat4_identity()
	t.Parent = nil
	return t
}

func transform_from_rotation(rotation Quaternion) Transform {
	var t Transform
	transform_set_position_rotation_scale(&t, vec3_zero(), rotation, vec3_one())
	t.Local = mat4_identity()
	t.Parent = nil
	return t
}

func transform_from_position_rotation(position Vec3, rotation Quaternion) Transform {
	var t Transform
	transform_set_position_rotation_scale(&t, position, rotation, vec3_one())
	t.Local = mat4_identity()
	t.Parent = nil
	return t
}

func transform_from_position_rotation_scale(position Vec3, rotation Quaternion, scale Vec3) Transform {
	var t Transform
	transform_set_position_rotation_scale(&t, position, rotation, scale)
	t.Local = mat4_identity()
	t.Parent = nil
	return t
}

func transform_get_parent(t *Transform) *Transform {
	if t == nil {
		return nil
	}
	return t.Parent
}

func transform_set_parent(t *Transform, parent *Transform) {
	if t != nil {
		t.Parent = parent
	}
}

func transform_get_position(t *Transform) Vec3 {
	return t.Position
}

func transform_set_position(t *Transform, position Vec3) {
	t.Position = position
	t.IsDirty = true
}

func transform_translate(t *Transform, translation Vec3) {
	t.Position = vec3_add(t.Position, translation)
	t.IsDirty = true
}

func transform_get_rotation(t *Transform) Quaternion {
	return t.Rotation
}

func transform_set_rotation(t *Transform, rotation Quaternion) {
	t.Rotation = rotation
	t.IsDirty = true
}

func transform_rotate(t *Transform, rotation Quaternion) {
	t.Rotation = quat_mul(t.Rotation, rotation)
	t.IsDirty = true
}

func transform_get_scale(t *Transform) Vec3 {
	return t.Scale
}

func transform_set_scale(t *Transform, scale Vec3) {
	t.Scale = scale
	t.IsDirty = true
}

func transform_scale(t *Transform, scale Vec3) {
	t.Scale = vec3_mul(t.Scale, scale)
	t.IsDirty = true
}

func transform_set_position_rotation(t *Transform, position Vec3, rotation Quaternion) {
	t.Position = position
	t.Rotation = rotation
	t.IsDirty = true
}

func transform_set_position_rotation_scale(t *Transform, position Vec3, rotation Quaternion, scale Vec3) {
	t.Position = position
	t.Rotation = rotation
	t.Scale = scale
	t.IsDirty = true
}

func transform_translate_rotate(t *Transform, translation Vec3, rotation Quaternion) {
	t.Position = vec3_add(t.Position, translation)
	t.Rotation = quat_mul(t.Rotation, rotation)
	t.IsDirty = true
}

func transform_get_local(t *Transform) Mat4 {
	if t != nil {
		if t.IsDirty {
			tr := mat4_mul(quat_to_mat4(t.Rotation), mat4_translation(t.Position))
			tr = mat4_mul(mat4_scale(t.Scale), tr)
			t.Local = tr
			t.IsDirty = false
		}

		return t.Local
	}
	return mat4_identity()
}

func transform_get_world(t *Transform) Mat4 {
	if t != nil {
		l := transform_get_local(t)
		if t.Parent != nil {
			p := transform_get_world(t.Parent)
			return mat4_mul(l, p)
		}
		return l
	}
	return mat4_identity()
}
