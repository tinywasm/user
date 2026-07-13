← [Stage 2](PLAN_STAGE_2_JSON_WIRE.md) | Next → [Stage 4](PLAN_STAGE_4_TESTS_MODULE.md)

# Stage 3 — purge the Go stdlib from the edge binary

Every file in this repo compiles to `js/wasm` (`grep -rn "go:build" server/` →
empty). Each stdlib import below is therefore payload in the edge binary.

## 3a — mechanical substitutions

Replace, file by file. Do not change behavior.

| Import | Files | Replacement |
|---|---|---|
| `encoding/json` | `server/jwt.go`, `server/oauth.go` | `tinywasm/json` (`json.Encode` / `json.Decode` over `model.Fielder`) |
| `strings` | `server/jwt.go`, `server/lan.go`, `server/middleware.go`, `server/users.go` | `tinywasm/fmt` (`fmt.Split`, `fmt.Contains`, …) |
| `time` | `server/identities.go`, `server/jwt.go`, `server/lan.go`, `server/middleware.go`, `server/module.go`, `server/sessions.go`, `server/users.go` | `tinywasm/time` |
| `strconv` | `server/lan.go` | `tinywasm/fmt` |
| `errors` | `server/api_token.go` | `tinywasm/fmt` (`fmt.Err`) |
| `database/sql` | `server/rbac.go`, `server/sql.go` | `tinywasm/orm` — the repo is DB-agnostic; a raw `*sql.DB`/`sql.ErrNoRows` must not appear |
| `sync` | `server/cache_users.go`, `server/module.go`, `server/sessions.go` | keep **only** if `tinywasm` has no equivalent; if it must stay, **report it** in the final summary as the remaining edge cost |
| `crypto/hmac`, `crypto/sha256` | `server/jwt.go` | `tinywasm/crypto` — see the gate below |
| `encoding/base64` | `server/jwt.go` | `github.com/tinywasm/base64` (`URLEncode`/`URLDecode`) — already published |
| `net` | `server/lan.go` | IP parsing/compare over `tinywasm/fmt`; if a real `net.ParseIP` equivalent is missing ecosystem-wide, **STOP and report** — the primitive belongs upstream, not here |

### Carve-out — do NOT "fix" this one

`golang.org/x/crypto/bcrypt` in [server/auth.go](../server/auth.go) **stays**.
Replacing the password hash changes the stored credential format and needs a
data migration with a compatibility window — that is its own plan. Leave
`bcrypt`, `PasswordHashCost`, `getDummyHash` and the timing-safe dummy-compare
logic exactly as they are.

### Gate — `tinywasm/crypto`

`server/jwt.go` needs this API, which does not exist yet in
`github.com/tinywasm/crypto` (it ships AES/ECDSA only):

```go
// github.com/tinywasm/crypto — the gate: not published yet
func HMACSHA256(key, message []byte) []byte
func HMACEqual(mac1, mac2 []byte) bool        // constant-time
```

It is specified in `tinywasm/crypto`'s own `docs/PLAN.md`. If it is not
published when this stage runs: **STOP and report.** Do not vendor a local HMAC
here, and do not leave `crypto/*` imports in place "for now".

Base64 is **not** part of that gate — it already ships as
`github.com/tinywasm/base64` (v0.0.2, zero dependencies):

```go
func URLEncode(src []byte) string             // RFC 4648 §5, unpadded — what JWT uses
func URLDecode(s string) ([]byte, error)
```

Use it for the JWT header/payload/signature segments and drop `encoding/base64`.

`ValidateJWT` must keep its constant-time signature comparison (today
`hmac.Equal`) — use `crypto.HMACEqual`. A byte-by-byte `==` over the signature
reintroduces a timing oracle and fails Stage 5.

The JWT payload (`jwtHeader`, `jwtPayload`) is currently marshalled with
struct tags and `encoding/json`. Struct tags are not the ecosystem pattern:
re-express both as `model.Definition` literals in `models.go` (Stage 1's Kind
API) so `ormc` generates their codecs and `tinywasm/json` can encode them.

## 3b — OAuth: the stdlib reaches the public API

This is not confined to `server/`. [user.go](../user.go) — a **WASM-shared**
file — declares:

```go
type OAuthProvider interface {
    Name() string
    AuthCodeURL(state string) string
    ExchangeCode(ctx context.Context, code string) (*oauth2.Token, error)
    GetUserInfo(ctx context.Context, token *oauth2.Token) (OAuthUserInfo, error)
}
```

so `context` (stdlib) and `golang.org/x/oauth2` (which pulls `net/http`) are in
the library's public surface, and `server/oauth.go` adds `net/http` + `net/url`
on top. Rework:

- Redefine `OAuthProvider` with ecosystem types: `tinywasm/context` instead of
  stdlib `context`, and a **typed token struct owned by this library**
  (e.g. `type OAuthToken struct { AccessToken, RefreshToken, TokenType string; ExpiresAt int64 }`)
  instead of `*oauth2.Token`. No `any`, no `map`.
- `server/oauth.go` performs the code exchange and the userinfo call over
  **`tinywasm/fetch`** with `tinywasm/json` codecs, not `net/http` +
  `encoding/json`. `tinywasm/fetch` is callback-based
  (`doRequest(r, func(resp, err))`) — adapt the flow to it; do not reintroduce
  a synchronous stdlib client to keep the current shape.
- URL/query building that used `net/url` moves to `tinywasm/fmt`.
- `golang.org/x/oauth2` leaves `go.mod` entirely.
- The OAuth **flow** (state generation, single-use state consumption, provider
  mismatch detection, identity linking, the `EventOAuthReplay` /
  `EventOAuthExpiredState` / `EventOAuthCrossProvider` events) is unchanged.
  Only the transport and the types change.

Consumers implementing `OAuthProvider` (Google/Microsoft) break. That is
accepted and must be called out in the final summary.

## Acceptance

- `grep -rn "\"encoding/json\"\|\"net/http\"\|\"net/url\"\|\"database/sql\"\|\"strings\"\|\"time\"\|\"strconv\"\|\"errors\"\|\"context\"\|\"crypto/\|\"encoding/base64\"\|golang.org/x/oauth2" --include=*.go .`
  → **empty**.
- `golang.org/x/crypto` is the only `golang.org/x/*` left in `go.mod`, used
  solely by `server/auth.go`.
- `GOOS=js GOARCH=wasm go build ./...` succeeds.
- `gotest ./...` green — JWT round-trip, OAuth begin/callback and LAN login
  keep their existing test coverage passing.
- If `sync` or an IP primitive had to stay, the final summary names the file,
  the reason, and what upstream API would remove it.
