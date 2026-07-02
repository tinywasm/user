package userserver

import (
	"strings"
	"time"

	"github.com/tinywasm/router"
	"github.com/tinywasm/user"
)

const userKey = "tinywasm/user/user"

// RegisterMCP envuelve el handler MCP con middleware de sesión.
// Alternativa limpia a registrar hooks en el MCPServer (que no existe en tinywasm/mcp).
func (m *Module) RegisterMCP() router.Middleware {
	return m.Middleware()
}

// Middleware protects routes. Validates the session and injects
// the authenticated *User into the router.Context.
// Returns HTTP 401 if the session is missing or expired.
func (m *Module) Middleware() router.Middleware {
	return func(next router.HandlerFunc) router.HandlerFunc {
		return func(ctx router.Context) {
			u, err := m.validateSession(ctx)
			if err != nil {
				ctx.WriteStatus(401)
				ctx.Write([]byte("unauthorized"))
				return
			}
			ctx.SetValue(userKey, u)
			next(ctx)
		}
	}
}

// FromContext extracts the authenticated *User injected by Middleware or RegisterMCP.
// Returns (nil, false) if the context carries no authenticated user.
func (m *Module) FromContext(ctx router.Context) (*user.User, bool) {
	u, ok := ctx.Value(userKey).(*user.User)
	return u, ok
}

// AccessCheck is the bridge function for tinywasm/crudp and tinywasm/site.
// Reads the router.Context from data, validates the session, and checks RBAC permissions.
// Satisfies the site.SetAccessCheck(fn) signature directly.
//
// Usage: site.SetAccessCheck(m.AccessCheck)
func (m *Module) AccessCheck(resource string, action byte, data ...any) bool {
	for _, d := range data {
		if ctx, ok := d.(router.Context); ok {
			u, err := m.validateSession(ctx)
			if err != nil {
				return false
			}
			ok, _ := m.HasPermission(u.ID, resource, action)
			return ok
		}
	}
	return false
}

// InjectIdentity implements mcp.Authorizer.
// Delegates to validateSession (respects configured AuthMode).
// On failure: returns ctx unchanged — CanExecute will deny.
func (m *Module) InjectIdentity(ctx router.Context) {
	u, err := m.validateSession(ctx)
	if err != nil {
		m.notify(user.SecurityEvent{
			Type:      user.EventUnauthorizedAccess,
			IP:        clientIP(ctx),
			Timestamp: time.Now().Unix(),
		})
		return
	}
	ctx.SetValue(userKey, u)
}

// CanExecute implements mcp.Authorizer.
// Reads identity injected by InjectIdentity and checks RBAC.
func (m *Module) CanExecute(ctx router.Context, resource string, action byte) bool {
	u, ok := ctx.Value(userKey).(*user.User)
	if !ok || u == nil {
		return false
	}
	ok2, _ := m.HasPermission(u.ID, resource, action)
	if !ok2 {
		m.notify(user.SecurityEvent{
			Type:      user.EventAccessDenied,
			UserID:    u.ID,
			Resource:  resource,
			Timestamp: time.Now().Unix(),
		})
	}
	return ok2
}

// validateJWT validates a raw JWT string and returns the active user.
func (m *Module) validateJWT(token string) (*user.User, error) {
	userID, err := ValidateJWT(m.config.JWTSecret, token)
	if err != nil {
		m.notify(user.SecurityEvent{Type: user.EventJWTTampered, Timestamp: time.Now().Unix()})
		return nil, err
	}
	u, err := m.GetUser(userID)
	if err != nil {
		return nil, err
	}
	if u.Status != "active" {
		m.notify(user.SecurityEvent{Type: user.EventNonActiveAccess, UserID: u.ID, Timestamp: time.Now().Unix()})
		return nil, user.ErrSuspended
	}
	return &u, nil
}

func (m *Module) validateSession(ctx router.Context) (*user.User, error) {
	// AuthModeBearer: API/MCP clients — JWT in Authorization header, no cookie.
	if m.config.AuthMode == user.AuthModeBearer {
		const prefix = "Bearer "
		auth := ctx.GetHeader("Authorization")
		if !strings.HasPrefix(auth, prefix) {
			return nil, user.ErrSessionExpired
		}
		return m.validateJWT(auth[len(prefix):])
	}

	// Cookie modes: browser clients.
	cookie := ctx.GetHeader("Cookie") // Simple implementation, might need a helper to parse specific cookie
	if cookie == "" {
		return nil, user.ErrSessionExpired
	}

	// For simplicity in this refactor, we assume a helper or that GetHeader for "Cookie"
	// is managed. Ideally router.Context would have a GetCookie(name) method.
	// Since it doesn't, we might need to parse it manually if it's multiple cookies.
	token := ""
	for _, c := range strings.Split(cookie, ";") {
		c = strings.TrimSpace(c)
		if strings.HasPrefix(c, m.config.CookieName+"=") {
			token = c[len(m.config.CookieName)+1:]
			break
		}
	}

	if token == "" {
		return nil, user.ErrSessionExpired
	}

	if m.config.AuthMode == user.AuthModeJWT {
		return m.validateJWT(token)
	}

	sess, err := m.GetSession(token)
	if err != nil {
		return nil, err
	}

	u, err := m.GetUser(sess.UserID)
	if err != nil {
		return nil, err
	}
	return &u, nil
}
