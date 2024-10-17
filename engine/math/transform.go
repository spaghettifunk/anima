package math

func TransformCreate() *Transform {
	t := &Transform{}
	t.SetPositionRotationScale(NewVec3Zero(), NewQuatIdentity(), NewVec3One())
	t.Local = NewMat4Identity()
	t.Parent = nil
	return t
}

func TransformFromPosition(position Vec3) *Transform {
	t := &Transform{}
	t.SetPositionRotationScale(position, NewQuatIdentity(), NewVec3One())
	t.Local = NewMat4Identity()
	t.Parent = nil
	return t
}

func TransformFromRotation(rotation Quaternion) *Transform {
	t := &Transform{}
	t.SetPositionRotationScale(NewVec3Zero(), rotation, NewVec3One())
	t.Local = NewMat4Identity()
	t.Parent = nil
	return t
}

func TransformFromPositionRotation(position Vec3, rotation Quaternion) *Transform {
	t := &Transform{}
	t.SetPositionRotationScale(position, rotation, NewVec3One())
	t.Local = NewMat4Identity()
	t.Parent = nil
	return t
}

func TransformFromPositionRotationScale(position Vec3, rotation Quaternion, scale Vec3) *Transform {
	t := &Transform{}
	t.SetPositionRotationScale(position, rotation, scale)
	t.Local = NewMat4Identity()
	t.Parent = nil
	return t
}

func (t *Transform) SetPosition(position Vec3) {
	t.Position = position
	t.IsDirty = true
}

func (t *Transform) Translate(translation Vec3) {
	t.Position = t.Position.Add(translation)
	t.IsDirty = true
}

func (t *Transform) SetRotation(rotation Quaternion) {
	t.Rotation = rotation
	t.IsDirty = true
}

func (t *Transform) Rotate(rotation Quaternion) {
	t.Rotation = t.Rotation.Mul(rotation)
	t.IsDirty = true
}

func (t *Transform) SetScale(scale Vec3) {
	t.Scale = scale
	t.IsDirty = true
}

func (t *Transform) ScaleIt(scale Vec3) {
	t.Scale = t.Scale.Mul(scale)
	t.IsDirty = true
}

func (t *Transform) SetPositionRotation(position Vec3, rotation Quaternion) {
	t.Position = position
	t.Rotation = rotation
	t.IsDirty = true
}

func (t *Transform) SetPositionRotationScale(position Vec3, rotation Quaternion, scale Vec3) {
	t.Position = position
	t.Rotation = rotation
	t.Scale = scale
	t.IsDirty = true
}

func (t *Transform) TranslateRotate(translation Vec3, rotation Quaternion) {
	t.Position = t.Position.Add(translation)
	t.Rotation = t.Rotation.Mul(rotation)
	t.IsDirty = true
}

func (t *Transform) GetLocal() Mat4 {
	if t != nil {
		if t.IsDirty {
			m := t.Rotation.ToMat4()
			tr := m.Mul(NewMat4Translation(t.Position))
			s := NewMat4Scale(t.Scale)
			tr = s.Mul(tr)
			t.Local = tr
			t.IsDirty = false
		}
		return t.Local
	}
	return NewMat4Identity()
}

func (t *Transform) GetWorld() Mat4 {
	if t != nil {
		l := t.GetLocal()
		if t.Parent != nil {
			p := t.Parent.GetWorld()
			return l.Mul(p)
		}
		return l
	}
	return NewMat4Identity()
}
