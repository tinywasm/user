# Design — Stable identity, isolated auth and RBAC

## Goal

Turn `github.com/tinywasm/user` into the stable, lightweight identity contract
that downstream libraries can depend on without inheriting OAuth providers,
session implementations, persistence, or RBAC. Create two sibling libraries:

- `github.com/tinywasm/auth` — authentication, sessions, credential modes,
  OAuth2 flow, and concrete providers.
- `github.com/tinywasm/rbac` — roles, permissions, assignments, and
  authorization.

The local development path must not require any Google secret. It must present
a selector of preconfigured simulated identities and their roles, then issue a
normal local session for the selected identity. Production continues to use the
real Google OAuth provider.

## Rejected Alternatives

### 1. Keeping subpackages in `user`

Embedding `authority`, `oauth2`, `email_password`, `trusted_ip`, and `session`
inside `user` would keep the dependency graph and release cadence coupled. A
competing OAuth provider change would force a new `user` release even though the
stable `Subject` contract is untouched. Keeping the root as a facade centralizes
risk for every consumer.

**Decision:** Move each area to the single owner that naturally evolves it.

### 2. Forwarding packages or type aliases under `user`

Re-exporting `auth` or `rbac` types as `user.Authority` or type aliases would
preserve compile compatibility for one minor release but keep the import graph
coupled. Consumers would still transitively pull `orm`, `fetch`, `jwt`, and
provider dependencies through the root, defeating the lightweight goal.

**Decision:** Breaking migration with no forwarding packages. Consumers import
`user`, `auth`, and `rbac` independently.

### 3. Fake Google OAuth HTTP server / OAuth callback bypass

Simulating Google by running a local HTTP server that mimics the provider or by
skipping `ConsumeState` would require weakening OAuth state validation, handling
network access in tests, and registering redirect URIs for development. It would
also exercise a different code path from production, hiding session-middleware and
RBAC bugs.

**Decision:** Direct local authenticator `auth/local`. It exercises the real
session middleware and the real `rbac.Can` decision path, has zero `fetch` calls,
no credentials, no network, and is selected explicitly in the development
composition root. Production builds construct Google OAuth from Cloudflare secrets
and never mount `auth/local`. No build tag, hostname test, or missing-secret
fallback silently selects it.
