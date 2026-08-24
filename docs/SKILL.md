# tinywasm/user Skill

## Description

`tinywasm/user` is the stable identity value shared by `tinywasm/auth` and
`tinywasm/rbac`. It contains no router, no persistence, and no policy.

## Core Concepts

- **Stable values only**: `SubjectID` and `Subject`.
- **No service**: authentication sessions and RBAC decisions live in siblings.
- **WASM-safe**: the root imports no database or provider packages.

## Public API Contract

```go
package user

type SubjectID string

type Subject struct {
    ID     SubjectID
    Email  string
    Name   string
    Avatar string
}

// Encode/Decode helpers for typed transport (model.FieldWriter/Reader) may
// be provided; no other exported API.
```

## Composition

```go
import (
    "github.com/tinywasm/user"
    "github.com/tinywasm/auth/authority"
    "github.com/tinywasm/rbac"
)

 // auth resolves a Subject
 var _ auth.SubjectStore = (*authority.Module)(nil) // returns user.Subject

 // rbac evaluates by SubjectID
 rb.Can(string(s.ID), resource, action)
 // or rb.Can(string(user.SubjectID), ...) when typed
```

`auth.SubjectStore.GetOrCreateSubject` and `auth.SessionIssuer.IssueSession`
use `user.SubjectID`; `rbac.Service.Can` accepts the same identifier as
plain `string` to stay dependency-minimal while representing `user.SubjectID`.

## Cross-library Contracts

- `user.SubjectID` is the only cross-library person reference.
- Auth owns creating/resolving `Subject`; rbac owns storing/evaluating
  assignments by `SubjectID`.
