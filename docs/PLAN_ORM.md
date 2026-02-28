# ORM Plan: Database Abstraction & Hydration (`user`)

## Goal
Refactor database interactions to exclusively use `tinywasm/orm`. Eliminate raw SQL strings, rely on generated typed matchers, and hydrate the `User` struct seamlessly.

## Execution Steps

### Phase 2.5: Integrate `tinywasm/orm`
**Target Files:**
- `user.go`, `user_roles.go`, `user_permissions.go`
- `sql.go` (Modify `Store` struct)

**Instructions (Eliminating Hardcoded SQL):**
1. **Current Problem:** `crud.go` contains hardcoded SQL strings, which prevents the package from being database-agnostic.
2. **Solution:** Use `github.com/tinywasm/orm` to completely abstract database interactions.
3. Add the `//go:generate ormc -struct <Name>` directive to your model structs (`User`, `Role`, `Permission`).
4. Run `go generate ./...` (or use the `ormc` CLI directly) to create the typed ORM accessors (like `ReadOneUser`, `UserMeta`, etc.).
5. **Injection:** Replace any direct `database/sql` or raw `Executor` references in the `Store` struct with `db *orm.DB` from `tinywasm/orm`.

### Phase 3: Fluent ORM Queries (Unification)
**Target Files:**
- `crud.go`
- `sql.go`

**Instructions (Database Agnosticism):**
1. **Using the ORM:** Now that the `*orm.DB` is injected, rewrite `GetUser` and `GetUserByEmail` in `crud.go` to use the ORM's fluent API (`store.db.Query(model).Where(UserMeta.ID).Eq(id)`).
2. **Hydration Strategy:** To return a `User` struct completely hydrated with its `[]Role` and `[]Permission`, you can write sequential ORM queries within the `GetUser` function:
   - Fetch the User via `ReadOneUser()`.
   - Fetch their Roles using `ReadAllRole()` with the respective `Where` filters.
   - Fetch their Permissions using `ReadAllPermission()`.
   *(Note: Sequential queries are perfectly acceptable and often cleaner than massive JOINs when using the ORM).*

---

## 📟 Annex A: ORM Injection Example

In `user_back.go`, the `Init()` function must receive an already instantiated `*orm.DB` directly from `main.go` via dependency injection:

```go
// user_back.go (pseudo-code)
func Init(db *orm.DB) (*Store, error) {
	if db == nil {
		return nil, fmt.Errorf("database connection required")
	}

	store = &Store{
		db:        db,
		userCache: NewUserCache(1000), // Phase 2
	}
	return store, nil
}
```

This ensures that:
- The same `Store` struct works with any database dialect transparently via the ORM.
- No hardcoded SQL strings in `crud.go`.
- Database engine selection (`sqlite`, `postgres`, `indexdb`) happens once in `cmd/<app>/main.go` and its returned `*orm.DB` is simply passed down to `user.Init()`.

---

## 📟 Annex D: ORM Reference Guide

For your reference, here is the official `tinywasm/orm` API syntax you must use for all database operations inside the `user` package:

```markdown
# ORM Skill

## Installation

```bash
go get github.com/tinywasm/orm
go install github.com/tinywasm/orm/cmd/ormc@latest
```

## Public API Contract

### Interfaces
- `Model`: `TableName()`, `Columns()`, `Values()`, `Pointers()` *(auto-implemented by `ormc`)*
- `Compiler`: `Compile(Query, Model) (Plan, error)`
- `Executor`: `Exec()`, `QueryRow()`, `Query()`
- `TxExecutor`: `BeginTx()`
- `TxBoundExecutor`: Embeds `Executor`, `Commit()`, `Rollback()`

### Auto-Generated Code (`cmd/ormc`)
To auto-generate the `orm.Model` interface and typed definitions, place the `//go:generate` directive above an standard Go struct (no special tags needed):

```go
//go:generate ormc -struct User
type User struct {
    ID       string // Automatically detected as PK by tinywasm/fmt
    Username string
    Age      int
}
```

For a `struct User`, the `ormc` compiler generates:
- `UserMeta` struct containing table and typed column names (e.g. `UserMeta.Username`).
- `ReadOneUser(qb *orm.QB) (*User, error)`
- `ReadAllUser(qb *orm.QB) ([]*User, error)`

### Core Structs
- `DB`: `New(Executor, Compiler)`, `Create`, `Update`, `Delete`, `Query`, `Tx`
- `QB` (Fluent API): `Where("col")`, `Limit(n)`, `Offset(n)`, `OrderBy("col")`, `GroupBy("cols...")`
- `Clause` (Chainable): `.Eq()`, `.Neq()`, `.Gt()`, `.Gte()`, `.Lt()`, `.Lte()`, `.Like()`
- `OrderClause` (Chainable): `.Asc()`, `.Desc()`
- `Plan`: `Mode`, `Query`, `Args`

### Constants
- `Action`: `Create`, `ReadOne`, `Update`, `Delete`, `ReadAll`

## Usage Snippet

```go
// 1. Where clauses use generated Meta descriptors (no magic strings)
// 2. Query builder uses a Fluent API chain
// 3. Results are executed and cast by auto-generated typed functions
qb := db.Query(m).
    Where(UserMeta.Age).Eq(18).
    Or().Where(UserMeta.Name).Like("A%").
    OrderBy(UserMeta.CreatedAt).Desc().
    Limit(10)

users, err := models.ReadAllUser(qb)
```
```
