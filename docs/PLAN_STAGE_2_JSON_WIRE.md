← [Stage 1](PLAN_STAGE_1_KIND_MIGRATION.md) | Next → [Stage 3](PLAN_STAGE_3_EDGE_STDLIB.md)

# Stage 2 — one wire format: JSON. Delete urlencoded parsing.

## Why this is a deletion, not a fix

`POST /login` in [server/mount.go](../server/mount.go) currently branches on
`Content-Type` and hand-parses `application/x-www-form-urlencoded` bodies by
splitting on `&` and `=`. That parser is **doubly wrong** and the correct
response is to remove it:

- **It never runs in this ecosystem.** A tinywasm form is never submitted as a
  native browser POST: `form.Render()` binds a submit handler that calls
  `e.PreventDefault()` and routes through `Form.Submit()` →
  `OnSubmit(data model.Fielder, done)`. The consumer sends the `Fielder`, and
  the ecosystem codec is `tinywasm/json` over the ormc-generated
  `EncodeFields`/`DecodeFields`.
- **It is edge weight.** Every byte in `server/` reaches the WASM binary
  (no build tags). A parser for a transport nobody uses is pure bloat.

It is also broken (it never percent-decodes, so a real urlencoded body with
`user%40test.com` would fail anyway) — but **do not fix it. Delete it.**

## Work

1. In [server/mount.go](../server/mount.go), `POST user.PathLogin` decodes
   **only** JSON:

   ```go
   r.Post(user.PathLogin, func(ctx router.Context) {
       data := &user.LoginData{}
       if err := json.Decode(string(ctx.Body()), data); err != nil {
           ctx.WriteStatus(400)
           ctx.Write([]byte(err.Error()))
           return
       }
       // ... unchanged: Login, SetCookie, 302 to afterLogin
   })
   ```

   - Remove the `Content-Type` branching entirely, and with it the `strings`
     and `tinywasm/fmt` imports if nothing else in the file needs them.
   - A malformed/absent body fails `json.Decode` → `400`. There is no
     `415`/"unsupported content type" branch and no content-type sniffing:
     this endpoint speaks one language.

2. **Validation stays in the kinds.** Do **not** add field checks (non-empty
   email, password length, format) to the handler. `input.Email()` /
   `input.Password()` already implement `Validate(value string) error`, and
   `model.Field.Validate` is the input-boundary floor. The handler decodes and
   calls `m.Login`; bad credentials come back as `user.ErrInvalidCredentials`.

3. `POST user.PathLogout` and the OAuth begin/callback routes keep their
   current behavior in this stage (their transport is dealt with in Stage 3).

4. Update the tests in [tests/production_wiring_test.go](../tests/production_wiring_test.go)
   and [tests/hardening_test.go](../tests/hardening_test.go): every
   `POST /login` becomes a JSON body built with `json.Encode(&user.LoginData{…})`
   and `Content-Type: application/json`. Delete the urlencoded cases and the
   `login` helper's urlencoded body in `hardening_test.go`.

   Keep the **consumer-view contract test** (`testConsumerViewSSR`): it renders
   `form.New("login", &user.LoginData{})` and asserts the field names — that is
   the contract between a consumer-built page and this module's DTO, and it
   still holds. Just stop implying the page POSTs urlencoded.

## Acceptance

- `grep -rni "urlencoded" .` → **empty** (source and tests).
- `grep -rn "GetHeader(\"Content-Type\")" server/` → **empty**.
- `gotest ./...` green.
- A `POST /login` with a valid JSON body returns `302` + session cookie; with a
  malformed body, `400`; with wrong credentials, `401`.
