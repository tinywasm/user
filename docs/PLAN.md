---
PLAN: "satisfy platformd.Identity — expose the logged-in user to the shell"
TAG: v0.3.0
STATUS: running
SESSION: 4415263291472933684
---

# Plan — `user`: expose the logged-in identity to the application shell

## The problem

`github.com/tinywasm/layout/platformd` renders the application chassis. Its header
and its nav drawer both show who is logged in, and it takes that through a read
contract it declares itself (`platformd/platformd.go`):

```go
type Identity interface {
	UserName() string    // who is logged in
	UserAvatar() string  // URL of their picture; empty is normal
	UserRoles() []string // display names, not authorization codes
}
```

It asks for facts and nothing else. The glyph drawn when there is no avatar
belongs to `platformd`, which owns the sprite and the styling; an authentication
package has no business choosing one.

**Roles, plural, because they are plural here.** An earlier draft of this plan
asked for a singular `area` and claimed the concept was missing. That was wrong
on both counts — see §0.

**This repository does not satisfy it today.** The demo in
`layout/platformd/web/client.go` mocks it with a two-method stub, and that stub is
the only implementation that exists. Since `user` is the package a real
application will authenticate against, the shell should be able to hand it a
session and get the chrome filled in — not have every consumer write the adapter
again.

## 0. What an earlier draft got wrong

Recorded so nobody re-derives it:

- It said **"`area` does not exist and has to be introduced"** and offered three
  ways to derive it. `area` was the wrong concept. What exists is **roles**, and
  the shell now asks for those. §2 of that draft is gone; there is no pending
  design decision.
- It listed **§1 "check whether the package builds for wasm"** as work.
  Already checked: `GOOS=js GOARCH=wasm go build ./...` **passes clean** across
  all nine packages. No split, no `shell` subpackage.
- It put **`Avatar` out of scope**. It is back in scope — the contract asks for
  it — and it is worse than a gap: it is a **dead field** (§3).

### What is missing, precisely

`ProfileDTO` (`user.go:247-256`) is the closest thing:

| Contract wants | `ProfileDTO` has | Gap |
|---|---|---|
| `UserName() string` | `Name string` | field, not a method |
| `UserRoles() []string` | `Roles []string` | holds **codes**, not display names |
| `UserAvatar() string` | `Avatar string` | **never populated by anything** |

**Fields are not methods.** A struct with the right field names still does not
satisfy an interface. Either `ProfileDTO` grows the three methods, or a small
adapter type does — §4 argues for the adapter.

### A naming collision to be careful about

This package **already exports `Identity`** (`models_orm.go`, `IdentityStore` at
`user.go:130`): the ORM row that ties a user to an auth provider. It is unrelated
to `platformd.Identity` and must not be renamed or overloaded. Whatever new type
this plan adds must carry a different name — `ShellProfile` is suggested below —
and the two must never appear in the same sentence of documentation without
saying which is which.

---

## 1. Roles are already here, and they are N:M

Nothing to introduce. What exists, verified:

- `user_role` (`models.go:56-62`) is a join table with a **composite primary
  key** on `user_id` + `role_id`. Uniqueness is on the pair, so one user holds
  many rows.
- `UserModel.roles` is a `model.StructSlice`, generating `Roles []Role`
  (`models_orm.go:18`).
- `AssignRole` swallows duplicate-key violations and `GetUserRoles` returns a
  slice (`authority/rbac.go:108-163`).

Two facts that shape the work:

**`User.Roles` is `Exclude: true`** (`models.go:17`), so it never crosses the
wire and the ORM never reads or writes it. Exactly one function fills it:
`hydrateUser` (`authority/users.go:34-101`), server-side, with a four-step
fan-out. A `User` decoded from a payload always has `Roles == nil`.

**There is no primary, current or active role.** No ordering, no rank, no flag;
`HasPermission` is a flat OR over the union of every role's permissions
(`authority/rbac.go:230-258`). This is why the shell shows roles only inside its
menu and never picks one to display at rest — there is nothing to pick.

## 2. `Roles` holds codes, and the shell wants names

`ProfileDTO.Roles` is filled in exactly one place, `authority/ops.go:29-33`:

```go
for _, r := range u.Roles {
	profile.Roles = append(profile.Roles, r.Code)
}
```

`r.Code`, not `r.Name`. Consumers use those codes to **authorize**.

**Do not repurpose the field.** Changing it to names would silently break every
authorization check reading it. Add display names alongside:

```go
RoleNames []string // parallel to Roles; r.Name, for humans
```

filled in the same loop. `ShellProfile` reads `RoleNames`; nothing else changes.

⚠️ The bootstrap seed makes code and name coincide for one role
(`authority/bootstrap.go:48-49` passes the same `RoleCode` as both). **Do not
take that as licence to use one for the other** — it is an accident of the seed,
and `GetRoleByCode` (`rbac.go:185-195`) is the only place the two are meant to
meet.

## 3. `Avatar` is a dead field

`ProfileDTO.Avatar` (`user.go:252`) is declared, encoded and decoded, and
**nothing ever assigns it**. `opMe` builds `ProfileDTO{Id, Name, Email}` and
leaves it `""`. There is no `avatar` column on `UserModel` and the OAuth identity
mapping does not carry one, though the providers return a picture URL
(`OAuthUserInfo`, `user.go:60`).

Satisfying `UserAvatar()` is therefore real work, not wiring:

1. An `avatar` field on `UserModel` (`model.Text()`), plus the migration.
2. Populate it from `OAuthUserInfo` where the provider supplies one.
3. Carry it in `opMe`.

An empty avatar is a **normal, expected outcome** — the shell falls back to its
own glyph — so none of this blocks the contract being satisfied. Ship the type
first, fill the column second.

`Locale` (`user.go:255`) is in the same dead state. Out of scope; noted so the
next reader does not assume it works.

## 4. The type

```go
// ShellProfile is the read-only view of a session that an application shell
// renders. It satisfies github.com/tinywasm/layout/platformd.Identity.
//
// NOT to be confused with Identity in this package, which is the ORM row tying
// a user to an auth provider.
type ShellProfile struct {
	Name   string
	Avatar string
	Roles  []string // display names, never codes
}

func (p ShellProfile) UserName() string    { return p.Name }
func (p ShellProfile) UserAvatar() string  { return p.Avatar }
func (p ShellProfile) UserRoles() []string { return p.Roles }
```

Plus one constructor from what the package already produces:

```go
// Shell converts a profile into the shape an application shell renders.
func (p ProfileDTO) Shell() ShellProfile
```

**Do not** make `ProfileDTO` implement the interface directly. It is a wire DTO
with `EncodeFields`/`DecodeFields`; hanging presentation methods on it couples
the transport shape to one consumer's chrome, and the next shell will want a
fourth method.

`ShellProfile` carries no styling of any kind, which is the point: it is facts
the shell is free to render however it likes.

## 4. Acceptance criteria

Each is checkable.

1. `GOOS=js GOARCH=wasm go build ./...` still succeeds. It already does today —
   this is a regression guard, not an investigation.
2. A compile-time assertion exists and is exercised by a test:
   ```go
   var _ platformd.Identity = ShellProfile{}
   ```
   ⚠️ This makes `user` depend on `layout`. **That may be the wrong direction** —
   a backend auth package importing a UI package. If it is, drop the assertion
   and put the equivalent one in `layout` instead, or in `user/tests`, which
   already exists as a separate module. Decide this explicitly and record why.
3. `ProfileDTO.Shell()` round-trips: name, avatar and role NAMES survive, and
   `Roles` (the codes) is left untouched.
4. `docs/ARCHITECTURE.md` records that `Roles` are codes and `RoleNames` are
   labels, and that an empty avatar is expected rather than an error.
5. `docs/SKILL.md` gains a snippet showing a consumer wiring a session into a
   platform shell end to end.
6. `README.md` re-indexes every file under `docs/`.
7. `gotest` green in the root module and in `tests/`.

## 5. Out of scope

- Rendering the avatar. Producing the URL is §3; what a shell does with it —
  loading, sizing, failure — is the shell's problem, and `platformd` already
  falls back to its own glyph.
- `ProfileDTO.Locale`, dead in the same way as `Avatar` was.
- Any change to `platformd`. The contract is already published and the demo
  already mocks it; this plan makes the real package fit the contract, not the
  contract fit the package.
- Multi-tenant scoping. RBAC here is global — there is no department, tenant or
  organisation in any model — and introducing one is a schema project, not a
  contract adapter.
