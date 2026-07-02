# Agent Guide — `tinywasm/user`

Constraints for agents working on user UI modules. Read this before any change.

---

## Construction Harness — typed & explicit (the TinyWasm approach)

This library is part of TinyWasm's **construction harness**: the typed, explicit API is what keeps an
agent that doesn't know the library from building wrong code. The compiler must reject mistakes; what
it can't catch becomes a `devMode` warning — never a silent failure.

- **Typed over `any`** — no generic slots; typed builder methods (like `tinywasm/json`), reusing `fmt` types. Anything reactive goes only through a signal binding (`BindText`/`Bind*`), which requires a signal.
- **Explicit names** — `Text` (static) vs `BindText` (reactive); reading the call states intent.
- **Illegal states unrepresentable** — dynamic content has ONE path, typed to require a signal.
- **Minimal public surface** — export only what the author types; engine plumbing stays unexported.
- **Docs are minimal "how" instructions, not long skills** — if a rule must be *remembered*, close it
  with types, not prose.

(Ecosystem rationale: `tinywasm/docs/ARNES_DE_CONSTRUCCION.md`.)

---

## Component Contract — ONE way (signals)

Modules and views implement **only** `Render() *dom.Element` (+ optional `Init(ctx dom.Ctx)`).
There is **NO** `OnMount`/`OnUpdate`/`OnUnmount` and **NO** manual `Update()` (unexported in `dom`).

- Do **not** delegate `form.OnMount()` — `tinywasm/form` self-manages via signal bindings. A module
  that embeds a form just includes it in its render tree (expose sub-components via
  `Children() []dom.Component` so their `Init` runs).
- Dynamic lists (e.g. `lan.go` IP rows) are a `dom.SignalNodes` bound with `container.BindChildren(s)`;
  add/remove rebuilds `[]*dom.Element` (each `.Key(id)`) and only the changed row patches.

State the UI shows lives in **typed signals**; changing one patches only the bound node — never
re-render a whole element, never a Virtual DOM.

## No Generics

Zero generic functions (follow `tinywasm/fmt` codec rule "cero any, cero map"). Use concrete typed
signals: `SignalString`/`SignalBool`/`SignalNodes`, `DeriveString`/`DeriveBool`, `Bind*`, `Show`.
Never `Signal[T]`.

## Minimal Public Surface

Export only what other packages consume (module entry types). Unexport view helpers, field models,
and anything only this package uses. Struct fields stay unexported.

## WASM / TinyGo

- UI lifecycle/reactive code in `//go:build wasm` files; `!wasm` stubs where called from tag-less code.
- No Go stdlib: use `github.com/tinywasm/fmt`. DOM only via `github.com/tinywasm/dom`, never
  `syscall/js`. `switch` not `map`. No `defer/recover`. Embed `dom.Element` by value.

## Testing

```bash
go install github.com/tinywasm/devflow/cmd/gotest@latest
gotest
```

- `gotest`, never `go test`. Stdlib assertions only. Dual WASM/stdlib; real DOM in WASM tests.
- Cover frequent use cases: login/register/profile forms work after construction with **no lifecycle
  hook**; `lan` add/remove patches single IP rows (others keep identity). Publish with `gopush 'message'`.

## Documentation First

Update docs **before** code and before `gopush`: `docs/ARCHITECTURE.md` (remove `OnMount` delegation
from the module-lifecycle description), update `docs/diagrams/` (`flowchart TD`, no `subgraph`,
`<br/>` for breaks), and re-index `README.md` so every `docs/` file is linked.
