package authority

import (
	"github.com/tinywasm/router"
)

// Module is an APIModule: it mounts its own routes and carries its own identity.
var _ router.APIModule = (*Module)(nil)

// ModelName is the module's identity (model.ModuleNaming), used as the RBAC resource
// and as the key by which a host registers it.
func (m *Module) ModelName() string { return "user" }

// MountAPI publishes the authentication flows on the host router. The module
// owns its routes; consumers just Mount it like any other APIModule.
func (m *Module) MountAPI(r router.Router) {
	for _, auth := range m.config.Authenticators {
		auth.Mount(r, m)
	}
}
