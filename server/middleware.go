package userserver

import (
	"github.com/tinywasm/fmt"
	"github.com/tinywasm/model"
	"github.com/tinywasm/time"

	"github.com/tinywasm/router"
	"github.com/tinywasm/user"
)

var _ model.Authorizer = (*Module)(nil).Can

// Authenticate returns a router.Middleware that validates the session (Cookie or Bearer).
// If valid, it sets the UserId in the context via ctx.SetUserID(id).
// If invalid, the UserId remains empty ("") indicating an anonymous user.
func (m *Module) Authenticate() router.Middleware {
	return func(next router.HandlerFunc) router.HandlerFunc {
		return func(ctx router.Context) {
			u, err := m.validateSession(ctx)
			if err == nil && u != nil {
				ctx.SetUserID(u.Id)
			}
			next(ctx)
		}
	}
}

// Can checks if the userID has permission for the resource/action.
// It also handles security event notification on failure.
func (m *Module) Can(userID string, resource model.Resource, action model.Action) bool {
	if userID == "" {
		return false
	}
	ok, _ := m.HasPermission(userID, resource, action)
	if !ok {
		m.notify(user.SecurityEvent{
			Type:      user.EventAccessDenied,
			UserID:    userID,
			Resource:  string(resource),
			Timestamp: time.Now() / 1e9,
		})
	}
	return ok
}

// validateSession validates the session from the router.Context.
func (m *Module) validateSession(ctx router.Context) (*user.User, error) {
	// AuthModeBearer: API/MCP clients — JWT in Authorization header, no cookie.
	if m.config.AuthMode == user.AuthModeBearer {
		const prefix = "Bearer "
		auth := ctx.GetHeader("Authorization")
		if !fmt.HasPrefix(auth, prefix) {
			return nil, user.ErrSessionExpired
		}
		return m.validateJWT(auth[len(prefix):])
	}

	// Cookie modes: browser clients.
	cookie, ok := ctx.Cookie(m.config.CookieName)
	if !ok {
		return nil, user.ErrSessionExpired
	}

	if m.config.AuthMode == user.AuthModeJWT {
		return m.validateJWT(cookie.Value)
	}

	sess, err := m.GetSession(cookie.Value)
	if err != nil {
		return nil, err
	}

	u, err := m.GetUser(sess.UserId)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// validateJWT validates a raw JWT string and returns the active user.
func (m *Module) validateJWT(token string) (*user.User, error) {
	userID, err := ValidateJWT(m.config.JWTSecret, token)
	if err != nil {
		m.notify(user.SecurityEvent{Type: user.EventJWTTampered, Timestamp: time.Now() / 1e9})
		return nil, err
	}
	u, err := m.GetUser(userID)
	if err != nil {
		return nil, err
	}
	if u.Status != "active" {
		m.notify(user.SecurityEvent{Type: user.EventNonActiveAccess, UserID: u.Id, Timestamp: time.Now() / 1e9})
		return nil, user.ErrSuspended
	}
	return &u, nil
}
