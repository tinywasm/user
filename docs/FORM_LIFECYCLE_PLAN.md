# tinywasm/user — Plan: Remove Form Lifecycle Delegation

> **Master:** tinywasm/docs/PLAN.md · **Depends on:** tinywasm/form/docs/PLAN.md, tinywasm/dom/docs/PLAN.md
> **Module:** `github.com/tinywasm/user`
> **Type:** Breaking-aligned migration (mostly deletion).

---

## Prerequisites

```bash
# Canonical test runner (WASM tests run against a real DOM). Required: external agents have no global gotest.
go install github.com/tinywasm/devflow/cmd/gotest@latest
```

## Development Rules

- **Documentation First:** update `docs/ARCHITECTURE.md` before code.
- **WASM only:** UI lifecycle code under `ui/` is `//go:build wasm`.
- **DOM access only via `tinywasm/dom`.** No `syscall/js`.
- **Tests:** `gotest` (never `go test`); stdlib only; dual WASM/stdlib. Publish with `gopush 'msg'`.
- **Minimal public API:** export only what a component *user* types; unexport anything only this package uses (helpers, field models, single-use constants). State lives in unexported fields exposed via signals.

## Signals API recap (self-contained)

`form` no longer has lifecycle hooks: it binds inputs to signals and patches surgically on its own.
Modules just embed forms in their render tree. State a module shows lives in a `dom.Signal`; one-time
setup goes in `Init(ctx dom.Ctx)`. No `Update()`, no `OnMount`/`OnUpdate`.

---

## Context

The `ui/*` modules implement `OnMount` only to **delegate** to the form lifecycle:

| File | Current `OnMount` |
|------|-------------------|
| ui/login.go | `m.form.OnMount()` |
| ui/profile.go | `m.form.OnMount(); m.passwordForm.OnMount()` |
| ui/register.go | `m.form.OnMount()` |
| ui/lan.go | own add/remove IP-rows logic |

With signal-bound forms, this delegation is dead.

---

## Change

1. **Delete `OnMount`** from `login.go`, `profile.go`, `register.go`. Ensure each form is part of the
   module's render tree (returned directly from `Render()`), so the engine wires its bindings.
   No replacement hook; `Children() []dom.Component` was part of the old architecture and is removed.

2. **`lan.go`:** the add/remove IP-rows logic becomes a `rows *dom.SignalNodes` bound with
   `container.BindChildren(rows)`. The add/remove buttons rebuild the rows (`rows.Set(buildRows())`,
   each row keyed with `.Key(...)`) → only the affected row is inserted/removed. One-time setup → `Init(ctx)`.

---

## Documentation (do FIRST)

- **`docs/ARCHITECTURE.md`**: update the module lifecycle section — modules no longer delegate
  `OnMount`; forms self-manage via signals; `lan.go` rows are a `SignalNodes` list.
- **`docs/diagrams/`**: if a module-lifecycle diagram exists, update it (`flowchart TD`, no `subgraph`)
  to show `Init (once) → Render → bindings`; remove the `OnMount` delegation arrows.
- **`README.md`**: re-index `docs/`.

## Tests — frequent use cases (`gotest`)

Stdlib assertions only; dual WASM/stdlib:

- **stdlib:** module data/validation logic.
- **wasm (real DOM):** login/register/profile render with working forms **after construction, with no
  lifecycle hook**; `lan` add/remove patches single IP rows (keyed reconcile keeps others identical).
- **In-browser (tinywasm MCP):** submit/validate each form; add/remove IP rows; `browser_get_errors` clean.

## Done When

- `ui/*` modules implement no lifecycle hooks except an optional `Init`; forms self-manage.
- `lan.go` IP rows are a `SignalNodes` list bound with `BindChildren`.
- **Docs:** ARCHITECTURE.md + diagrams updated; `README.md` re-indexed. **Tests:** use-case tests pass under `gotest`.
