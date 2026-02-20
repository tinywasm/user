package user

// LoginData is validated by LoginModule on both frontend and backend.
type LoginData struct {
	Email    string
	Password string
}

// RegisterData is validated by RegisterModule.
type RegisterData struct {
	Name     string
	Email    string
	Password string
	Phone    string
}

// ProfileData is validated by ProfileModule (name/phone update).
type ProfileData struct {
	Name  string
	Phone string
}

// PasswordData is validated by ProfileModule (password change sub-form).
type PasswordData struct {
	Current string
	New     string
	Confirm string
}
