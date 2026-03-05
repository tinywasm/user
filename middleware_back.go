//go:build !wasm

package user

import (
	"context"
	"net/http"
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

func (m *Module) validateSession(r *http.Request) (*User, error) {
	cookie, err := r.Cookie(m.config.CookieName)
	if err != nil {
		return nil, ErrSessionExpired
	}

	if m.config.AuthMode == AuthModeJWT {
		userID, err := validateJWT(m.config.JWTSecret, cookie.Value)
		if err != nil {
			return nil, err
		}
		u, err := m.GetUser(userID)
		if err != nil {
			return nil, err
		}
		return &u, nil
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
