//go:build !wasm

package user

import (
	"context"
	"net/http"
	"strings"
	"time"
)

type contextKey int

const userKey contextKey = iota

// RegisterMCP envuelve el handler MCP con middleware de sesión.
// Alternativa limpia a registrar hooks en el MCPServer (que no existe en tinywasm/mcp).
//
// Ejemplo:
//
//	mcpHandler := mcp.NewStreamableHTTPServer(srv)
//	mux.Handle("/mcp", m.RegisterMCP(mcpHandler))
func (m *Module) RegisterMCP(next http.Handler) http.Handler {
	return m.Middleware(next)
}

// Middleware protects HTTP routes. Validates the session cookie and injects
// the authenticated *User into the request context.
// Returns HTTP 401 if the session is missing or expired.
//
// Example:
//
//	mux.Handle("/admin", m.Middleware(adminHandler))
func (m *Module) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, err := m.validateSession(r)
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		ctx := context.WithValue(r.Context(), userKey, u)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// FromContext extracts the authenticated *User injected by Middleware or RegisterMCP.
// Returns (nil, false) if the context carries no authenticated user.
func (m *Module) FromContext(ctx context.Context) (*User, bool) {
	u, ok := ctx.Value(userKey).(*User)
	return u, ok
}

// AccessCheck is the bridge function for tinywasm/crudp and tinywasm/site.
// Reads the *http.Request from data, validates the session, and checks RBAC permissions.
// Satisfies the site.SetAccessCheck(fn) signature directly.
//
// Usage: site.SetAccessCheck(m.AccessCheck)
func (m *Module) AccessCheck(resource string, action byte, data ...any) bool {
	for _, d := range data {
		if r, ok := d.(*http.Request); ok {
			u, err := m.validateSession(r)
			if err != nil {
				return false
			}
			ok, _ := m.HasPermission(u.ID, resource, action)
			return ok
		}
	}
	return false
}

// mcpContextFunc returns a func compatible with mcp.SetHTTPContextFunc / SetSSEContextFunc.
func (m *Module) mcpContextFunc() func(context.Context, *http.Request) context.Context {
	return func(ctx context.Context, r *http.Request) context.Context {
		u, err := m.validateSession(r)
		if err != nil {
			return ctx // unauthenticated — tools check with FromContext
		}
		return context.WithValue(ctx, userKey, u)
	}
}

// InjectIdentity implements mcp.Authorizer.
// Delegates to validateSession (respects configured AuthMode).
// On failure: returns ctx unchanged — CanExecute will deny.
func (m *Module) InjectIdentity(ctx context.Context, r *http.Request) context.Context {
	u, err := m.validateSession(r)
	if err != nil {
		m.notify(SecurityEvent{
			Type:      EventUnauthorizedAccess,
			IP:        clientIP(r),
			Timestamp: time.Now().Unix(),
		})
		return ctx
	}
	return context.WithValue(ctx, userKey, u)
}

// CanExecute implements mcp.Authorizer.
// Reads identity injected by InjectIdentity and checks RBAC.
func (m *Module) CanExecute(ctx context.Context, resource string, action byte) bool {
	u, ok := ctx.Value(userKey).(*User)
	if !ok || u == nil {
		return false
	}
	ok2, _ := m.HasPermission(u.ID, resource, action)
	if !ok2 {
		m.notify(SecurityEvent{
			Type:      EventAccessDenied,
			UserID:    u.ID,
			Resource:  resource,
			Timestamp: time.Now().Unix(),
		})
	}
	return ok2
}

// validateJWT validates a raw JWT string and returns the active user.
func (m *Module) validateJWT(token string) (*User, error) {
	userID, err := ValidateJWT(m.config.JWTSecret, token)
	if err != nil {
		m.notify(SecurityEvent{Type: EventJWTTampered, Timestamp: time.Now().Unix()})
		return nil, err
	}
	u, err := m.GetUser(userID)
	if err != nil {
		return nil, err
	}
	if u.Status != "active" {
		m.notify(SecurityEvent{Type: EventNonActiveAccess, UserID: u.ID, Timestamp: time.Now().Unix()})
		return nil, ErrSuspended
	}
	return &u, nil
}

func (m *Module) validateSession(r *http.Request) (*User, error) {
	// AuthModeBearer: API/MCP clients — JWT in Authorization header, no cookie.
	if m.config.AuthMode == AuthModeBearer {
		const prefix = "Bearer "
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, prefix) {
			return nil, ErrSessionExpired
		}
		return m.validateJWT(auth[len(prefix):])
	}

	// Cookie modes: browser clients.
	cookie, err := r.Cookie(m.config.CookieName)
	if err != nil {
		return nil, ErrSessionExpired
	}

	if m.config.AuthMode == AuthModeJWT {
		return m.validateJWT(cookie.Value)
	}

	sess, err := m.GetSession(cookie.Value)
	if err != nil {
		return nil, err
	}

	u, err := m.GetUser(sess.UserID)
	if err != nil {
		return nil, err
	}
	return &u, nil
}
