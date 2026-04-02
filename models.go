package user

type User struct {
	ID          string
	Email       string `json:",omitempty" db:"unique"`
	Name        string
	Phone       string `json:",omitempty"`
	Status      string // "active", "suspended"
	CreatedAt   int64
	Roles       []Role       `json:",omitempty" db:"-"`
	Permissions []Permission `json:",omitempty" db:"-"`
}

type Session struct {
	ID        string
	UserID    string `db:"ref=users"`
	ExpiresAt int64
	IP        string `json:",omitempty"`
	UserAgent string `json:",omitempty"`
	CreatedAt int64
}

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

type Identity struct {
	ID         string
	UserID     string `db:"ref=users"`
	Provider   string
	ProviderID string
	Email      string `json:",omitempty"`
	CreatedAt  int64
}

type Role struct {
	ID          string
	Code        string
	Name        string
	Description string
}

type UserRole struct {
	UserID string
	RoleID string
}

type Permission struct {
	ID       string
	Name     string
	Resource string
	Action   string
}

type RolePermission struct {
	RoleID       string
	PermissionID string
}

type LANIP struct {
	ID        string
	UserID    string `db:"ref=users"`
	IP        string
	Label     string
	CreatedAt int64
}

type OAuthState struct {
	State     string `db:"pk"`
	Provider  string
	ExpiresAt int64
	CreatedAt int64
}
