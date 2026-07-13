← [Stage 5](PLAN_STAGE_5_OWASP.md) | [Master](PLAN.md)

# Stage 6 — documentation

Write the docs **last**, describing what actually landed. English only.

## `README.md`

- **Production wiring** (update the existing section): `New` → `MountAPI`
  (flow endpoints only) → inject `Authenticate` / `Can`; `Bootstrap` from env;
  the `me` MCP tool for cosmetic permission gating.
- **Views belong to the consumer**, and **forms travel as JSON**: the app
  builds its login page with `form.New("login", &user.LoginData{})` and submits
  the `Fielder` as JSON to `user.PathLogin`. State plainly that the library
  parses no HTML form encoding and renders no page.
- **Security model** (new section): the library is stateless on the edge; its
  anti-automation contract is `Config.RateLimit` (the consumer injects an
  edge-native limiter) plus `Config.OnSecurityEvent` (the signal stream). Note
  that **all** login failures return an identical `401` (anti-enumeration).
  Add a short table mapping the test suite to the OWASP Top 10 items it covers.

## `docs/ARCHITECTURE.md`

- **Edge posture** (new): every package in this repo compiles to `js/wasm`;
  there are no build tags. The dependency budget is therefore part of the
  design — `tinywasm/fmt`, `tinywasm/time`, `tinywasm/json`, `tinywasm/fetch`,
  `tinywasm/crypto`, `tinywasm/orm`. Name the one deliberate exception,
  `golang.org/x/crypto/bcrypt` in `server/auth.go`, and why it survives (stored
  credential format; replacing it needs a data migration).
- **Model authoring**: typed `model.Definition` literals with the kind in the
  `Type:` slot (`input.Email()`, `model.Text()`, `model.StructSlice(&Def)`);
  `ormc` generates the structs and codecs; `Field.Widget` no longer exists;
  generated struct fields use `Id`, not `ID`.
- **Routes table**: `POST /login` (JSON), `POST /logout`, OAuth begin/callback.
- **Two-module layout**: root = agnostic library; `tests/` = its own module,
  carrying the SQLite driver. Explain why (the root module must not carry a DB
  driver).
- Add `RateLimit` / `EventRateLimited` to the security-flow description.

## `docs/SKILL.md`

- Consumers never touch the private flow handlers.
- DTOs are `model.Definition`s; no struct tags; no `encoding/json`.
- No HTML in this library — views belong to the consumer.
- Forms are submitted as JSON; there is no urlencoded path.
- Test deps live in `tests/go.mod`, never in the root module.
- Rate limiting is the consumer's responsibility, via the hook.

## Acceptance

- The three files describe the delivered code, with no reference to removed
  machinery (`Widget`, urlencoded, `ui/`, SSR rendering).
- `README.md` links to `docs/ARCHITECTURE.md` and `docs/SKILL.md`.
