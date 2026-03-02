package user

// User
type User struct {
	ID          string       `json:"id" db:"pk"`
	Email       string       `json:"email,omitempty" db:"unique"`
	Name        string       `json:"name"`
	Phone       string       `json:"phone,omitempty"`
	Status      string       `json:"status"` // "active", "suspended"
	CreatedAt   int64        `json:"created_at"`
	Roles       []Role       `json:"roles,omitempty" db:"-"`
	Permissions []Permission `json:"permissions,omitempty" db:"-"`
}

func (User) TableName() string { return "users" }

// Session
type Session struct {
	ID        string `json:"id" db:"pk"`
	UserID    string `json:"user_id" db:"ref=users"`
	ExpiresAt int64  `json:"expires_at"`
	IP        string `json:"ip,omitempty"`
	UserAgent string `json:"user_agent,omitempty"`
	CreatedAt int64  `json:"created_at"`
}

func (Session) TableName() string { return "user_sessions" }

// Identity
type Identity struct {
	ID         string `json:"id" db:"pk"`
	UserID     string `json:"user_id" db:"ref=users"`
	Provider   string `json:"provider"`
	ProviderID string `json:"provider_id"`
	Email      string `json:"email,omitempty"`
	CreatedAt  int64  `json:"created_at"`
}

func (Identity) TableName() string { return "user_identities" }

// Role
type Role struct {
	ID          string `json:"id" db:"pk"`
	Code        string `json:"code"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

func (Role) TableName() string { return "rbac_roles" }

// UserRole
type UserRole struct {
	UserID string `json:"user_id"`
	RoleID string `json:"role_id"`
}

func (UserRole) TableName() string { return "rbac_user_roles" }

// Permission
type Permission struct {
	ID       string `json:"id" db:"pk"`
	Name     string `json:"name"`
	Resource string `json:"resource"`
	Action   string `json:"action"`
}

func (Permission) TableName() string { return "rbac_permissions" }

// RolePermission
type RolePermission struct {
	RoleID       string `json:"role_id"`
	PermissionID string `json:"permission_id"`
}

func (RolePermission) TableName() string { return "rbac_role_permissions" }

// LANIP
type LANIP struct {
	ID        string `json:"id" db:"pk"`
	UserID    string `json:"user_id" db:"ref=users"`
	IP        string `json:"ip"`
	Label     string `json:"label"`
	CreatedAt int64  `json:"created_at"`
}

func (LANIP) TableName() string { return "user_lan_ips" }

// OAuthState
type OAuthState struct {
	State     string `json:"state" db:"pk"`
	Provider  string `json:"provider"`
	ExpiresAt int64  `json:"expires_at"`
	CreatedAt int64  `json:"created_at"`
}

func (OAuthState) TableName() string { return "user_oauth_states" }
