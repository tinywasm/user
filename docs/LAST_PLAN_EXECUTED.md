---
PLAN: "refactor!: split stable user contracts from authentication and RBAC"
EXECUTOR: jules
REVIEWER: none
---

> This plan is dispatched via the CodeJob workflow. See skill: agents-workflow.

# Plan — Stable identity contracts, isolated authentication, and local role simulation

## Goal

Turn `github.com/tinywasm/user` into the stable, lightweight identity contract
that downstream libraries can depend on without inheriting OAuth providers,
session implementations, persistence, or RBAC. Create exactly two sibling
libraries, both owned by `tinywasm`:

1. `github.com/tinywasm/auth` owns authentication, sessions, credential modes,
   OAuth2 flow, and concrete OAuth providers.
2. `github.com/tinywasm/rbac` owns roles, permissions, role assignment,
   authorization, and its persistence.

The local development path must not require any Google secret. It must present
a selector of preconfigured simulated identities and their roles, then issue a
normal local session for the selected identity. Production continues to use the
real Google OAuth provider unchanged.

This is a breaking migration. Do not retain forwarding packages or type aliases
under `tinywasm/user`: those would keep its dependency graph and release cadence
coupled to the code being extracted.

## Development Rules

- Documentation and plan text are English. Source identifiers and error messages
  are English.
- This is TinyWasm code: no Go standard-library dependency in code reachable
  from WASM/TinyGo. Use `github.com/tinywasm/fmt`; do not introduce `map`,
  `reflect`, `syscall/js`, `defer`, or `recover` in WASM-shared code.
- Use slices plus linear search for the small provider, role, and scenario sets.
- Preserve narrow dependency injection ports. A package may receive an interface
  it needs, never a concrete sibling module or a database handle merely to reach
  another service.
- `cmd/` stays thin: argument parsing, dependency injection, and print/exit
  only. Logic and environment checks belong in exported, testable library code.
- Every repeated string (route, provider name, cookie name, error text, or
  environment key) is a named package constant. Do not duplicate literals.
- Use `gotest`, never `go test`. Tests live in each module's `tests/` directory
  and use real packages with in-memory fakes; no test calls Google.
- Update permanent documentation before source code. `README.md` indexes every
  permanent file in `docs/`; neither permanent documentation nor diagrams link
  to this ephemeral `PLAN.md`.
- Cross-repository plans must restate their contracts and use only GitHub URLs
  for optional external references. Do not use local relative paths across
  repositories.

## Final Ownership and Dependency Direction

```
tinywasm/user              stable values and ports only
      ▲             ▲
      │             │
tinywasm/auth      tinywasm/rbac
      ▲             ▲
      └──── application composition root ────┘
```

Rules of the final graph:

- `user` depends only on the minimal shared TinyWasm value/transport packages
  needed by its exported contracts. It imports neither `auth`, `rbac`, `orm`,
  `fetch`, `jwt`, nor a concrete provider.
- `auth` imports `user`, and may depend on its own persistence/runtime
  dependencies. It never imports `rbac`.
- `rbac` imports `user`, and may depend on its own persistence/runtime
  dependencies. It never imports `auth`.
- The application composition root is the only place allowed to import both
  `auth` and `rbac`. It wires the narrow ports between them.
- There are no compatibility packages at
  `github.com/tinywasm/user/{authority,oauth2,email_password,trusted_ip,session}`
  after the final `user` release.

### `tinywasm/user`: exact retained public contract

The final root has exactly these domain values; it must not become a service or
an adapter package:

```go
type SubjectID string

type Subject struct {
    ID     SubjectID
    Email  string
    Name   string
    Avatar string
}
```

`SubjectID` is the only cross-library reference to a signed-in person. `auth`
owns resolving and creating `Subject`; `rbac` stores and evaluates assignments
by `SubjectID`; applications may display a `Subject` returned by `auth`.

The root may additionally contain value-only encoding helpers required to pass
`Subject` across the existing TinyWasm typed transport. It contains no router
route, authentication interface, session interface, OAuth type, persistence
port, error value, model definition, CRUD presenter, event topic, or policy.
All of those have a single owner in `auth` or `rbac`.

Move ORM definitions, generated ORM code, `ProfileDTO`, `ShellProfile`, concrete
error values, every port currently in `user.go`, and all `authority` behavior
out of the root. Do not leave an import of `input`, `orm`, `router`, `events`,
`fetch`, `jwt`, a session package, or a provider package in `user` merely for
transitional convenience.

## Local Simulator Contract

The simulator is an `auth` development authenticator, not a fake Google HTTP
server and not an OAuth callback bypass. This is deliberate: it exercises the
real session middleware and real RBAC decision path while avoiding Google,
credentials, network access, and redirect registration.

Define a public `auth/local` package with a constructor that receives:

- an explicit slice of immutable `Scenario` values; each has a stable subject
  identifier, display name, email, avatar, and display role labels;
- an `auth.SubjectStore` and `auth.SessionIssuer`;
- an explicit `AfterLogin` route option;
- no environment variables and no implicit development-mode global.

Expose named constants for its provider name and start/selection paths. Its
public route serves a minimal selector, and its public POST selection route:

1. accepts only a scenario identifier from the configured slice;
2. rejects an unknown or empty identifier with a deterministic client error;
3. resolves or creates the configured identity through `SubjectStore`;
4. calls `SessionIssuer.IssueSession` with that subject identifier;
5. redirects to `AfterLogin`.

The simulator does **not** assign roles at login. The consuming app seeds each
scenario's subject and its role assignments through `rbac` before mounting the
routes. Therefore the role labels shown in the selector are only a transparent
description of the already-seeded authorization state; `rbac.Can` remains the
sole permission decision.

The local selector is selected explicitly in a development composition root.
Production builds construct Google OAuth from Cloudflare secrets and never
register `auth/local`. No build tag, hostname test, or missing-secret fallback
may silently select it.

## Stage 0 — Write durable design records before implementation

1. In `tinywasm/user`, replace the current architecture section that calls the
   root package "models, contracts, ports, view, consts" with the target
   three-library graph and its one-way dependency rules.
2. Add a permanent design document in `tinywasm/user/docs/` that records the
   rejected alternatives: keeping subpackages in `user`, forwarding aliases,
   and a fake Google OAuth server. Explain why the direct local authenticator is
   the chosen test seam.
3. Update `tinywasm/user/README.md` to describe the future root contract and
   link the permanent design document. Do not link this plan.
4. Every new sibling repository must receive equivalent permanent
   `ARCHITECTURE.md`, `README.md`, and a dependency-direction diagram before
   source is moved. The READMEs must index those documents.

Acceptance:

- the permanent docs name `auth` and `rbac` as siblings, never as `user`
  subpackages;
- no permanent document links to `PLAN.md`;
- the graph explicitly forbids `auth <-> rbac` dependencies.

## Stage 1 — Create the two modules and their self-contained plans (gate)

From the TinyWasm projects directory, create the repositories with the official
scaffolder:

```text
gonew auth "TinyWasm authentication mechanisms and session runtime" -owner=tinywasm
gonew rbac "TinyWasm role-based authorization runtime" -owner=tinywasm
```

Do not hand-create directories, `go.mod`, repository metadata, or initial tags.
If remote creation fails, stop and report the failure; do not silently use
`-local-only`, because these modules are public ecosystem dependencies.

In each generated repository, write its own `docs/PLAN.md` with the frontmatter
required by CodeJob. Those plans must restate the contracts below and must not
wait for the executor to infer any boundary from the old `user` code.

### `auth/docs/PLAN.md` required scope

- Move the `authority` identity/session responsibilities: subject and identity
  persistence, session persistence/cache, authentication middleware, logout,
  and mode registration.
- Move `email_password`, `trusted_ip`, `oauth2`, cookie and JWT session
  strategies, and Google/Microsoft providers into `auth` under an explicit
  package tree. OAuth exchange helpers move with the OAuth flow, not with a
  provider that happens to share them today.
- Define these exact `auth` boundary types:
  `SubjectStore` (resolve/create a `user.Subject` from a credential),
  `SessionIssuer` (issue a session for `user.SubjectID`), and `Service` (mount
  authentication routes, authenticate a request, and expose its narrow setup
  ports). OAuth state storage, security notification, and session storage ports
  also live in `auth` because no other sibling consumes them.
- Implement `auth/local` exactly to the Local Simulator Contract above, with
  zero `fetch` calls and no Google credentials.
- Do not own roles, permissions, grants, authorization decisions, or a
  `Can`-like method in `auth`.

### `rbac/docs/PLAN.md` required scope

- Move roles, permissions, role/user and role/permission relations, bootstrap
  seeding, CRUD operations for those records, caches, and permission evaluation
  out of `authority`.
- Define a small subject-ID based assignment port that applications use to seed
  roles for identities created by `auth`; it must not import `auth.Service`.
- Define the application-facing `rbac.Service` that owns `Can`, role/permission
  mutations, and cache invalidation. It accepts a stable subject ID, resource,
  and action, never an `auth` concrete type.
- Keep policy in the consumer: no application role or resource constants in
  `rbac`.
- Do not own OAuth, passwords, cookies, JWT, login routes, or session lookup.

Acceptance:

- both repositories are created by `gonew` with `tinywasm` as owner;
- both plans are self-contained and name exact old `user` source files to move;
- neither new module imports the other in its initial dependency graph.

## Stage 2 — Publish the minimal `tinywasm/user` contract (gate)

First add `SubjectID` and `Subject` to the root package and publish a contract
release that both new libraries can import. The old packages and models remain
temporarily in this release only so their current consumers continue to compile;
mark them as migration-only in documentation, but add no new API to them.

Do not call this temporary release the completed lightweight root: it exists
only to give `auth` and `rbac` a stable, published cross-library value type. The
final cleanup in Stage 5 removes the legacy code and its dependencies.

Acceptance:

- new `auth` and `rbac` code can depend exclusively on `user.SubjectID` and
  `user.Subject`, never on any legacy `user` model or port;
- the root contract has no dependency on either new sibling;
- every legacy package is explicitly listed as temporary in the migration
  documentation, with no forwarding package promised after Stage 5.

## Stage 3 — Implement and publish `tinywasm/auth` (gate)

Execute only `auth/docs/PLAN.md` until its acceptance criteria are green and the
module is published. Its tests must include:

- Google OAuth begin/callback routing and error paths using a fake provider;
- session issue, identify, logout, and invalid-session behavior;
- the local selector lists only configured scenarios, rejects unknown choices,
  creates/resolves the selected identity, issues a session, and redirects;
- a no-network assertion for the local selector;
- WASM/TinyGo compilation for packages intended to be edge-reachable.

The local simulator must be usable by an app with no
`GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`, or `GOOGLE_REDIRECT_URL` in its
process environment.

## Stage 4 — Implement and publish `tinywasm/rbac`

Execute `rbac/docs/PLAN.md` after the `user.SubjectID` contract is published.
It may execute in parallel with the internal implementation of `auth`, but both
releases are gates for consumer migration. `rbac` does not import `auth`; if a
needed type would create that dependency, move its storage-neutral definition to
`user` instead.

Its tests must include:

- role/permission create, assignment, revocation, and deletion invalidates the
  affected authorization result;
- permissions remain denied for an empty or unknown subject ID;
- malformed stored action values deny access and surface the documented security
  event rather than being silently ignored;
- a subject seeded for each local scenario receives exactly the permissions
  indicated by that scenario's displayed role labels;
- no test starts Google or uses network access.

## Stage 5 — Migrate consuming applications

Migrate consumers after published `auth` and `rbac` releases exist and before
removing legacy `user` packages. Do not add `replace` directives to production
`go.mod` files.

### `veltylabs/misitio`

1. Replace imports of `user/authority`, `user/oauth2`, and
   `user/oauth2/provider/google` with the published `auth`/`rbac` packages.
2. Split `config.NewAuthority` into explicit composition functions: one builds
   production Google authentication from Cloudflare secrets; one builds local
   simulation from configured scenarios. Neither falls back to the other.
3. In `web/server.go` choose the local composition explicitly. Define at least
   an administrator and a non-administrator scenario, seed their subjects and
   RBAC assignments into the in-memory database, then mount the selector.
4. In `edge/main.go` choose only production Google composition and retain the
   existing fail-fast validation of all three Google secrets.
5. Keep the application as a composition root: no provider protocol, RBAC
   persistence, or local-selector rendering logic belongs in `misitio`.
6. Update the login screen copy/button according to the selected composition:
   production says Google; local says simulated development identity. The
   selection is server-side composition, not a browser hostname heuristic.
7. Extend tests to prove the development server starts with no Google variables,
   both simulated scenarios authenticate, their real permissions differ, and the
   Worker composition still fails when a Google variable is absent.

### Other known consumers

Migrate `tinywasm/layout/platformd` and `veltylabs/mjosefa-cms` in the same
wave. First inventory their imported `user` exports and assign each to `user`,
`auth`, or `rbac`; do not mechanically rename imports. Each consumer gets its
own self-contained plan before code changes.

## Stage 6 — Reduce and publish `tinywasm/user` last among the foundation modules

After every consumer has migrated, remove the old code from `tinywasm/user` in
one breaking release:

1. Move each source file to the owner named in Stages 1–3; preserve history with
   `git mv` only when moving inside the same repository, otherwise copy followed
   by explicit deletion after destination tests pass.
2. Remove `authority/`, `oauth2/`, `email_password/`, `trusted_ip/`, and
   `session/` from `user`.
3. Remove ORM model definitions and generated ORM files that no longer belong to
   the root. Put each schema beside its owning service; cross-service relations
   use stable subject IDs, not imported concrete models.
4. Rewrite `go.mod` and `go.sum` so `user` retains only the dependencies needed
   to transport the two retained value types.
5. Replace all old root documentation examples with composition examples that
   import `user`, `auth`, and `rbac` independently.

Acceptance:

- `find . -type d` shows none of `authority`, `oauth2`, `email_password`,
  `trusted_ip`, or `session` beneath the `user` module;
- `go mod graph` for `user` contains no `orm`, `fetch`, or `jwt` dependency;
- `grep -R "github.com/tinywasm/user/authority\|github.com/tinywasm/user/oauth2\|github.com/tinywasm/user/session\|github.com/tinywasm/user/email_password\|github.com/tinywasm/user/trusted_ip" .`
  returns no production import;

- `gotest` and the TinyGo/WASM check pass in all three foundation modules;
- republish `auth` and `rbac` with their `go.mod` pinned to the reduced `user`
  release, then run each suite again. This dependency-only release proves that
  neither sibling used a legacy root export.

## Stage 7 — Release and regression verification

1. Publish in this strict order: temporary `user` contract, `auth` and `rbac`,
   consumers, reduced `user`, then the dependency-only `auth`/`rbac` releases.
   `auth` and `rbac` may be planned in parallel after the contract release.
2. For every release, run its documented `gotest` suite and TinyGo/WASM check
   before publishing.
3. In `misitio`, use the TinyWasm MCP after migration: verify `CLIENT` and
   `SERVER` build logs, navigate to the local selector, choose both scenarios,
   inspect the session-protected routes, and confirm browser console/errors are
   clean.
4. Confirm production remains fail-closed: an edge startup with any missing
   Google variable returns the documented configuration error; it never exposes
   the local selector.

## Dispatch Order

| Order | Repository | Work | Gate |
|---:|---|---|---|
| 0 | `tinywasm/user` | Permanent design records and this plan | Required before code |
| 1 | `tinywasm/auth` + `tinywasm/rbac` | Create with `gonew` and write self-contained plans | Blocks source migration |
| 2 | `tinywasm/user` | Publish `SubjectID`/`Subject` contract; retain legacy temporarily | Blocks 3–4 |
| 3 | `tinywasm/auth` | Auth/session/providers/local selector | Blocks consumer migration |
| 4 | `tinywasm/rbac` | Roles/permissions/authorizer | Blocks consumer migration |
| 5 | `veltylabs/misitio` | Explicit local simulator vs production Google composition | Waits for 2–4 |
| 6 | Other consumers | `layout/platformd`, `mjosefa-cms`, then discovered consumers | One plan per repository |
| 7 | `tinywasm/user` | Remove legacy behavior and publish lean root | Waits for 5–6 |
| 8 | `tinywasm/auth` + `tinywasm/rbac` | Pin final minimal `user` release and republish | Verifies final graph |

## Non-Goals

- Do not copy Cloudflare Google secrets into local `.env` files.
- Do not create a local Google OAuth HTTP server, mock Google endpoints, or
  weaken production OAuth state validation.
- Do not auto-enable local authentication based on a missing secret, a hostname,
  or a build failure.
- Do not keep compatibility wrappers in `tinywasm/user`.
- Do not change application-specific role policy while extracting the framework;
  only move its mechanism and preserve behavior.
