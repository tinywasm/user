package user

// orm:typed_fields
type User struct {
	ID          string `db:"pk"`
	Email       string `db:"unique"`
	Name        string
	Phone       string
	Status      string // "active", "suspended"
	CreatedAt   int64
	Roles       []Role       `db:"-"`
	Permissions []Permission `db:"-"`
}

// orm:typed_fields
type Session struct {
	ID        string `db:"pk"`
	UserID    string `db:"ref=users"`
	ExpiresAt int64
	IP        string
	UserAgent string
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

// orm:typed_fields
type Identity struct {
	ID         string `db:"pk"`
	UserID     string `db:"ref=users"`
	Provider   string
	ProviderID string
	Email      string
	CreatedAt  int64
}

// orm:typed_fields
type Role struct {
	ID          string `db:"pk"`
	Code        string
	Name        string
	Description string
}

// orm:typed_fields
type UserRole struct {
	UserID string `db:"pk,ref=users"`
	RoleID string `db:"pk,ref=roles"`
}

// orm:typed_fields
type Permission struct {
	ID       string `db:"pk"`
	Name     string
	Resource string
	Action   string
}

// orm:typed_fields
type RolePermission struct {
	RoleID       string `db:"pk,ref=roles"`
	PermissionID string `db:"pk,ref=permissions"`
}

// orm:typed_fields
type LANIP struct {
	ID        string `db:"pk"`
	UserID    string `db:"ref=users"`
	IP        string
	Label     string
	CreatedAt int64
}

// orm:typed_fields
type OAuthState struct {
	State     string `db:"pk"`
	Provider  string
	ExpiresAt int64
	CreatedAt int64
}
