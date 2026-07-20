package authority

import (
	"github.com/tinywasm/model"
	"github.com/tinywasm/router"
	"github.com/tinywasm/time"
	"github.com/tinywasm/user"
)

var _ model.Authorizer = (*Module)(nil).Can

// Authenticate returns a router.Middleware that asks the active SessionStrategy
// to identify the caller. If valid, sets UserId in the context via
// ctx.SetUserID(id). If invalid, UserId remains empty (anonymous).
func (m *Module) Authenticate() router.Middleware {
	return func(next router.HandlerFunc) router.HandlerFunc {
		return func(ctx router.Context) {
			if userID, err := m.strategy.Identify(ctx); err == nil && userID != "" {
				ctx.SetUserID(userID)
			}
			next(ctx)
		}
	}
}

// Can checks if the userID has permission for the resource/action, notifying on
// failure — unchanged from before.
func (m *Module) Can(userID string, resource model.Resource, action model.Action) bool {
	if userID == "" {
		return false
	}
	ok, err := m.HasPermission(userID, resource, action)
	if err != nil {
		m.notify(user.SecurityEvent{
			Type: user.EventPermissionCorrupt, UserID: userID,
			Resource: string(resource), Timestamp: time.Now() / 1e9,
		})
		return false
	}
	if !ok {
		m.notify(user.SecurityEvent{
			Type: user.EventAccessDenied, UserID: userID,
			Resource: string(resource), Timestamp: time.Now() / 1e9,
		})
	}
	return ok
}
