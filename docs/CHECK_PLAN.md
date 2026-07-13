# PLAN — Fix production wiring: typed model Definitions, no views in this library, browser-real login POST

> This plan is dispatched via the CodeJob workflow. See skill: agents-workflow.

## Context (zero-context summary)

The previous delivery (`docs/CHECK_PLAN.md`, commit `f7aef51`) implemented
`MountAPI` (login/logout/oauth routes), `me` with `Permissions`, and
`Bootstrap`. `Bootstrap` and `me` are verified green. The login flow has real
functional bugs, and two architectural violations must be corrected:

1. **`GET /login` renders a form with no fields.** `tinywasm/form` binds
   inputs exclusively via `model.Field.Widget`; the form DTOs (`LoginData`,
   `RegisterData`, `ProfileData`, `PasswordData`) are hand-written structs
   with no widget metadata, so every generated schema field has
   `Widget: nil` and `form.New` skips them all. The `form.RegisterInput(...)`
   workaround in `server/init.go` is dead machinery (form does no
   name-matching).
2. **This library must NOT own any view.** How a login page looks is the
   **consumer's** job (each app brands and lays out its own pages). The
   delivery wired `GET /login` to render HTML (`loginModule.RenderHTML` +
   `wrapSSR` + `renderLoginError` HTML). That rendering responsibility leaves
   this library.
3. **`POST /login` only decodes JSON** (`server/mount.go`), but a plain HTML
   form submits `application/x-www-form-urlencoded`. A real browser could
   never log in. The test passes because it posts JSON and only asserts
   status codes.

**How models are authored in this ecosystem (mandatory pattern — reference,
web: <https://github.com/tinywasm/model/blob/main/README.md> and
<https://github.com/tinywasm/orm/blob/main/docs/ARQUITECTURE.md>; the rule is
restated fully below, reading them is optional):** the source
of truth is a **typed `model.Definition` literal** (`var XxxModel =
model.Definition{...}`, the var name MUST end in `Model`). The generator
`ormc` (`tinywasm/orm/ormc`, invoked via `go generate` → `//go:generate ormc`)
parses that literal — including typed `Widget: input.Email()` expressions —
and **generates the concrete struct** plus `Schema()`, `Pointers()`,
`Validate()`, `EncodeFields()`, `DecodeFields()`. Only these generated codecs
make `tinywasm/json` work. **Struct tags are NOT the pattern. Stdlib
`encoding/json` is forbidden everywhere.**

**Ecosystem rules that apply:** no stdlib in shared code (`tinywasm/fmt`), no
`any`/`map` in public APIs, typed constants for repeated strings, errors
propagate, `gotest` only
(`go install github.com/tinywasm/devflow/cmd/gotest@latest`).

## Stage 1 — form DTOs become typed `model.Definition`s (ormc generates the rest)

Replace the four hand-written DTO structs in `models.go` with Definition
literals (new file `definitions_forms.go`, or alongside existing definitions
if the repo already groups them — do NOT leave the hand-written structs, ormc
generates them and names would collide):

```go
import (
	"github.com/tinywasm/form/input"
	"github.com/tinywasm/model"
)

var LoginDataModel = model.Definition{
	Name: "login_data",
	Fields: model.Fields{
		{Name: "email",    Type: model.FieldText, NotNull: true, Widget: input.Email()},
		{Name: "password", Type: model.FieldText, NotNull: true, Widget: input.Password()},
	},
}

var RegisterDataModel = model.Definition{
	Name: "register_data",
	Fields: model.Fields{
		{Name: "name",     Type: model.FieldText, NotNull: true, Widget: input.Text()},
		{Name: "email",    Type: model.FieldText, NotNull: true, Widget: input.Email()},
		{Name: "password", Type: model.FieldText, NotNull: true, Widget: input.Password()},
		{Name: "phone",    Type: model.FieldText, Widget: input.Phone()},
	},
}

var ProfileDataModel = model.Definition{
	Name: "profile_data",
	Fields: model.Fields{
		{Name: "name",  Type: model.FieldText, NotNull: true, Widget: input.Text()},
		{Name: "phone", Type: model.FieldText, Widget: input.Phone()},
	},
}

var PasswordDataModel = model.Definition{
	Name: "password_data",
	Fields: model.Fields{
		{Name: "current", Type: model.FieldText, NotNull: true, Widget: input.Password()},
		{Name: "new",     Type: model.FieldText, NotNull: true, Widget: input.Password()},
		{Name: "confirm", Type: model.FieldText, NotNull: true, Widget: input.Password()},
	},
}
```

- These are form/codec models, **no `Field.DB`** (they are not tables). If
  `ormc` cannot generate a table-less Definition, **STOP and report** — that
  fix belongs in `tinywasm/orm`, never worked around here.
- Regenerate with `go generate ./...` (runs `ormc`, latest published version).
  Verify the generated schemas carry the widgets and the codec methods, and
  that existing call sites (`user.LoginData{}` etc.) still compile against the
  generated structs (field names `Email`, `Password`, … must round-trip).
- **Delete `server/init.go`** (`form.RegisterInput` block): widgets now come
  from the Definition. If some other code path genuinely needs the registry,
  keep only that registration with a comment naming the path.
- Replace `server/export.go` (`ExportGetUserByEmail`, a test-only export) with
  a legitimate public method `Module.GetUserByEmail(email string)
  (user.User, error)`; update the test to use it.
- Do NOT migrate `User`/`Session`/`Identity` (DB models) in this plan — out of
  scope, they work today.

## Stage 2 — MountAPI serves flows, never views

The consumer builds and serves its own login page (with `tinywasm/form` over
`user.LoginData` and its own branding) and posts to this module's endpoints.
Rework `server/mount.go`:

- **Remove all HTML rendering** from this library's mount path: no
  `GET /login` page, no `wrapSSR`, no `renderLoginError` HTML. Delete
  `server/mount_ssr.go` (and `mount_wasm.go` if it only supported that
  rendering); remove the now-unused `loginUI()` plumbing from `mount.go` if
  nothing else needs it.
- `MountAPI` mounts **only**:
  - `POST user.PathLogin` — decode credentials by `Content-Type`:
    `application/x-www-form-urlencoded` (plain HTML form) parsed privately
    over `tinywasm/fmt` (stdlib `net/url` forbidden), or `application/json`
    via `tinywasm/json`; anything else → `400`. On success: `SetCookie` +
    `302` to `AfterLoginPath`. On failure: **`401`** + a short `text/plain`
    error body + the existing `SecurityEvent` — the consumer's page decides
    how to display it (e.g. re-rendering with the error via
    `?err=` on its own page; this library does not decide that).
  - `POST user.PathLogout` — unchanged behavior (destroy session, clear
    cookie), redirect to `user.PathLogin`.
  - OAuth begin/callback routes — unchanged flow, but failure paths return
    `401`/`400` text, never HTML.
- Path constants (`PathLogin`, `PathLogout`, `PathAfterLogin`,
  `Config.AfterLoginPath`) stay as delivered.

## Stage 3 — tests that would have caught the bugs

Rework `tests/production_wiring_test.go`:

- **Widget regression:** `form.New` over `&user.LoginData{}` yields exactly 2
  inputs (email + password) — fails fast if a future regeneration loses the
  widgets. Same one-liner check for the other three DTOs.
- **Consumer-view simulation:** render `form.New(&user.LoginData{}).SetSSR(true)`
  in the test (playing the consumer's role) and assert the produced HTML posts
  the field names the handler expects — the contract between consumer-built
  views and this module's endpoints.
- **`POST /login`** success + failure via **urlencoded** bodies (browser
  semantics): cookie + `302` on success; `401` + SecurityEvent on bad
  password. Keep one JSON case for the API path. `POST /logout` clears the
  cookie.
- Remove any test asserting this library renders HTML pages.
- All green under `gotest ./...` (native + wasm suites).

## Stage 4 — documentation (deferred by the previous delivery; not optional)

- `README.md`: "Production wiring" — `New` → `MountAPI` (flow endpoints only)
  → inject `Authenticate`/`Can`; `Bootstrap` from env; `me` permissions for
  cosmetic gating; **"views belong to the consumer"** with a short example:
  the app builds its login page with `form.New(&user.LoginData{})` and posts
  to `user.PathLogin`.
- `docs/ARCHITECTURE.md`: routes table (`POST /login`, `POST /logout`,
  oauth), bootstrap flow, and the model-authoring rule (typed
  `model.Definition` + ormc; widgets in the Definition).
- `docs/SKILL.md`: consumers never touch private flow handlers; DTOs are
  Definitions; no HTML in this library.

## Harness checklist (mandatory)

- Typed `model.Definition` literals only — no struct tags, no reflection, no
  stdlib `encoding/json`/`net/url`.
- Route paths / role codes / wildcard markers remain exported typed constants;
  no new string literals in logic.
- No `any`/`map` in public API; errors propagate; every auth failure emits its
  `SecurityEvent`.
- No unrelated refactors; `Bootstrap`/`me`/RBAC stay as delivered (green).

## Acceptance criteria

1. `gotest ./...` green; the widget-regression and urlencoded tests fail if
   either original bug is reintroduced.
2. `grep -rn "RenderHTML\|wrapSSR" server/mount*.go` → empty: the mount
   surface serves flows, not views.
3. The four DTOs exist only as generated code from their `model.Definition`;
   `form.New` over each yields all its fields with widgets.
4. Browser-real flow proven by test: consumer-style form HTML → urlencoded
   POST with bootstrap credentials → `302` + session cookie → logout clears
   it; wrong password → `401` + SecurityEvent.
5. README/ARCHITECTURE/SKILL updated as specified.

## Stages

| Stage | File(s) | Action |
|---|---|---|
| 1 | `definitions_forms.go` (new), `models.go` (remove hand-written DTOs), `models_orm.go` (regenerated), `server/init.go` (delete), `server/export.go` → `Module.GetUserByEmail` | typed Definitions + ormc regen; remove dead registry and test-only export |
| 2 | `server/mount.go`, `server/mount_ssr.go` (delete) | flows only: urlencoded+JSON POST, 401/400 text errors, no HTML |
| 3 | `tests/production_wiring_test.go` | widget regression, consumer-view contract, urlencoded round-trip |
| 4 | `README.md`, `docs/ARCHITECTURE.md`, `docs/SKILL.md` | production wiring + "views belong to the consumer" |
