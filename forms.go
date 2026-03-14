package user

import (
	"github.com/tinywasm/fmt"
)

// LoginData is validated by LoginModule on both frontend and backend.
type LoginData struct {
	Email    string
	Password string
}

func (m *LoginData) Schema() []fmt.Field {
	return []fmt.Field{
		{Name: "Email", Type: fmt.FieldText},
		{Name: "Password", Type: fmt.FieldText, Input: "password"},
	}
}

func (m *LoginData) Pointers() []any {
	return []any{&m.Email, &m.Password}
}

// RegisterData is validated by RegisterModule.
type RegisterData struct {
	Name     string
	Email    string
	Password string
	Phone    string
}

func (m *RegisterData) Schema() []fmt.Field {
	return []fmt.Field{
		{Name: "Name", Type: fmt.FieldText},
		{Name: "Email", Type: fmt.FieldText},
		{Name: "Password", Type: fmt.FieldText, Input: "password"},
		{Name: "Phone", Type: fmt.FieldText},
	}
}

func (m *RegisterData) Pointers() []any {
	return []any{&m.Name, &m.Email, &m.Password, &m.Phone}
}

// ProfileData is validated by ProfileModule (name/phone update).
type ProfileData struct {
	Name  string
	Phone string
}

func (m *ProfileData) Schema() []fmt.Field {
	return []fmt.Field{
		{Name: "Name", Type: fmt.FieldText},
		{Name: "Phone", Type: fmt.FieldText},
	}
}

func (m *ProfileData) Pointers() []any {
	return []any{&m.Name, &m.Phone}
}

// PasswordData is validated by ProfileModule (password change sub-form).
type PasswordData struct {
	Current string
	New     string
	Confirm string
}

func (m *PasswordData) Schema() []fmt.Field {
	return []fmt.Field{
		{Name: "Current", Type: fmt.FieldText, Input: "password"},
		{Name: "New", Type: fmt.FieldText, Input: "password"},
		{Name: "Confirm", Type: fmt.FieldText, Input: "password"},
	}
}

func (m *PasswordData) Pointers() []any {
	return []any{&m.Current, &m.New, &m.Confirm}
}
