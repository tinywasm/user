---
PLAN: "satisfy platformd.Identity — expose the logged-in user to the shell"
TAG: v0.1.0
EXECUTOR: pending
REVIEWER: none
STATUS: completed
SESSION: 14909466989555226837
---

# Plan — `user`: expose the logged-in identity to the application shell

## The problem

`github.com/tinywasm/layout/platformd` renders the application chassis. Its header
and its nav drawer both show who is logged in, and it takes that through a read
contract it declares itself (`platformd/platformd.go`):

```go
type Identity interface {
	UserName() string // who is logged in
	UserArea() string // the area they work in — a department, a tenant, a role
}
```

It asks for facts and nothing else. The glyph beside the name in the collapsed
rail belongs to `platformd`, which owns the sprite and the styling; an
authentication package has no business choosing one.

**This repository does not satisfy it today.** The demo in
`layout/platformd/web/client.go` mocks it with a two-method stub, and that stub is
the only implementation that exists. Since `user` is the package a real
application will authenticate against, the shell should be able to hand it a
session and get the chrome filled in — not have every consumer write the adapter
again.

### What is missing, precisely

`ProfileDTO` (`user.go:248-262`) is the closest thing and it is close:

| Contract wants | `ProfileDTO` has | Gap |
|---|---|---|
| `UserName() string` | `Name string` | field, not a method |
| `UserArea() string` | — | **no such concept anywhere in the package** |

Two consequences follow from that table:

1. **Fields are not methods.** A struct with the right field names still does not
   satisfy an interface. Either `ProfileDTO` grows the two methods, or a small
   adapter type does.
2. **`area` does not exist.** Not in `UserModel` (`models.go:8-20`), not in
   `ProfileDTO`, not in `RoleModel`. It has to be introduced deliberately — see
   §2, which is the only part of this plan that needs a design decision rather
   than mechanical work.

`Avatar` (`user.go:252`) is untouched by this plan. It is a URL to an image and
the shell does not ask for one; wiring it into the chrome is a separate problem
with its own loading and fallback behaviour.

### A naming collision to be careful about

This package **already exports `Identity`** (`models_orm.go`, `IdentityStore` at
`user.go:130`): the ORM row that ties a user to an auth provider. It is unrelated
to `platformd.Identity` and must not be renamed or overloaded. Whatever new type
this plan adds must carry a different name — `ShellProfile` is suggested below —
and the two must never appear in the same sentence of documentation without
saying which is which.

---

## 1. Where the implementation belongs

`platformd` runs in the browser. Whatever satisfies the contract must therefore
compile under `GOOS=js GOARCH=wasm` and must not drag `router`, `orm` or the
storage ports in with it.

Check first, before writing anything:

```
GOOS=js GOARCH=wasm go build github.com/tinywasm/user
```

`ProfileDTO` itself only touches `model.FieldWriter`/`FieldReader`, so it is
probably already clean — but `user.go` also declares `Config`, `Authenticator`
and every port interface in the same file, and those reach `router`. **If the
package does not build for wasm, do not force it**: put the shell-facing type in
its own file with no backend imports, or in a `user/shell` subpackage. Report
which of the two the build forced, with the error, rather than choosing silently.

## 2. `area` — the one design decision

The contract's `UserArea()` is "the area they work in — a department, a tenant, a
role". This package has roles (`RoleModel`, `models.go:45-53`) and nothing else
resembling it.

Three candidates, in the order I would try them:

**(a) Derive it from the primary role.** No schema change: `UserArea()` returns
the display `Name` of the user's first role, or `""` when they have none.
Cheapest, and truthful for an application whose "area" really is the role.
Weakness: a user with several roles has no defined primary one, so the answer
depends on row order.

**(b) A new `area` field on `UserModel`.** One `model.Text()` field, one
migration, and `ProfileDTO` carries it through. Unambiguous and independent of
roles. Weakness: every existing deployment gains a column that starts empty, and
something has to decide who fills it.

**(c) Leave it to the consumer.** The adapter takes the area as a constructor
argument and this package never stores it.

**Recommendation: (b), with (a) as the fallback when the field is empty.** An
area and a role are different things — a person can be an administrator *of*
Operations — and conflating them now is the kind of shortcut that is expensive to
unpick once data exists. The fallback keeps existing deployments rendering
something sensible on day one.

**Do not decide this alone.** Confirm with the maintainer before writing the
migration; the rest of the plan does not depend on which option wins.

## 3. The type

```go
// ShellProfile is the read-only view of a session that an application shell
// renders. It satisfies github.com/tinywasm/layout/platformd.Identity.
//
// NOT to be confused with Identity in this package, which is the ORM row tying
// a user to an auth provider.
type ShellProfile struct {
	Name string
	Area string
}

func (p ShellProfile) UserName() string { return p.Name }
func (p ShellProfile) UserArea() string { return p.Area }
```

Plus one constructor from what the package already produces:

```go
// Shell converts a profile into the shape an application shell renders.
func (p ProfileDTO) Shell() ShellProfile
```

**Do not** make `ProfileDTO` implement the interface directly. It is a wire DTO
with `EncodeFields`/`DecodeFields`; hanging presentation methods on it couples
the transport shape to one consumer's chrome, and the next shell will want a
third method.

`ShellProfile` carries no styling of any kind, which is the point: it is two
strings the shell is free to render however it likes.

## 4. Acceptance criteria

Each is checkable.

1. `GOOS=js GOARCH=wasm go build github.com/tinywasm/user/...` succeeds, and the
   package holding `ShellProfile` does not import `router` or `orm` —
   `go list -deps` proves it.
2. A compile-time assertion exists and is exercised by a test:
   ```go
   var _ platformd.Identity = ShellProfile{}
   ```
   ⚠️ This makes `user` depend on `layout`. **That may be the wrong direction** —
   a backend auth package importing a UI package. If it is, drop the assertion
   and put the equivalent one in `layout` instead, or in `user/tests`, which
   already exists as a separate module. Decide this explicitly and record why.
3. `ProfileDTO.Shell()` round-trips: name and area survive.
4. Whichever `area` option §2 lands on is documented in `docs/ARCHITECTURE.md`,
   including what fills it and what an empty value means.
5. `docs/SKILL.md` gains a snippet showing a consumer wiring a session into a
   platform shell end to end.
6. `README.md` re-indexes every file under `docs/`.
7. `gotest` green in the root module and in `tests/`.

## 5. Out of scope

- Avatar images. `ProfileDTO.Avatar` stays where it is; the shell asks for no
  imagery, and rendering one would be a separate problem with its own loading
  and fallback behaviour.
- Any change to `platformd`. The contract is already published and the demo
  already mocks it; this plan makes the real package fit the contract, not the
  contract fit the package.
- Multi-tenant area switching. Reading the current area is this plan; letting a
  user change it is a feature with its own UI.
