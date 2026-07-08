package userserver

import (
	"github.com/tinywasm/context"
	"github.com/tinywasm/json"
	"github.com/tinywasm/mcp"
	"github.com/tinywasm/user"
)

// Tools returns the list of MCP tools provided by the user module.
func (m *Module) Tools() []mcp.Tool {
	return []mcp.Tool{
		{
			Name:        "me",
			Description: "Get the profile of the currently authenticated user.",
			Resource:    "profile",
			Action:      'r',
			Execute: func(ctx *context.Context, req mcp.Request) (*mcp.Result, error) {
				userID := ctx.Value(mcp.CtxKeyUserID)
				if userID == "" {
					return nil, user.ErrInvalidCredentials
				}

				u, err := m.GetUser(userID)
				if err != nil {
					return nil, err
				}

				profile := user.ProfileDTO{
					ID:    u.ID,
					Name:  u.Name,
					Email: u.Email,
				}
				for _, r := range u.Roles {
					profile.Roles = append(profile.Roles, r.Code)
				}
				profile.Permissions = permissionsOf(u)

				var out string
				if err := json.Encode(profile, &out); err != nil {
					return nil, err
				}

				return &mcp.Result{Content: out}, nil
			},
		},
	}
}

func permissionsOf(u user.User) []string {
	var perms []string
	for _, p := range u.Permissions {
		perms = append(perms, p.Resource+":"+p.Action)
	}
	return perms
}
