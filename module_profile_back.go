//go:build !wasm

package user

func (m *profileModule) RenderHTML() string {
	m.form.SetSSR(true)
	m.passwordForm.SetSSR(true)
	return m.form.RenderHTML() + "<hr>" + m.passwordForm.RenderHTML()
}

func (m *profileModule) Create(data ...any) (any, error) {
	// Implementation depends on how user ID is passed.
	// Assuming site/crudp passes it, but we don't have the spec.
	return nil, nil
}

// Handler to support POST (Update)
func (m *profileModule) Update(id string, data ...any) error {
	if len(data) == 0 {
		return nil
	}

	switch d := data[0].(type) {
	case *ProfileData:
		return UpdateUser(id, d.Name, d.Phone)
	case *PasswordData:
		// Verify current password first?
		// PasswordData has Current, New, Confirm.
		if d.New != d.Confirm {
			return ErrInvalidCredentials // Password mismatch
		}
		if err := VerifyPassword(id, d.Current); err != nil {
			return err
		}
		return SetPassword(id, d.New)
	}
	return nil
}
