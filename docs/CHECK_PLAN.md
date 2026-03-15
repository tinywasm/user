# Migrate to tinywasm/orm v2 API (fmt.Field)

## Context

The ORM code generator (`ormc`) now produces `Schema() []fmt.Field` (from `tinywasm/fmt`) with individual bool constraint fields instead of the old `[]orm.Field` with bitmask constraints. The `Values()` method is removed; consumers use `fmt.ReadValues(schema, ptrs)` instead.

### Key API Changes

| Old (current) | New (target) |
|---|---|
| `import "github.com/tinywasm/orm"` in generated code | `import "github.com/tinywasm/fmt"` |
| `[]orm.Field{Name, Type: orm.TypeText, Constraints: orm.ConstraintPK}` | `[]fmt.Field{Name, Type: fmt.FieldText, PK: true}` |
| `orm.TypeText`, `orm.TypeInt64`, `orm.TypeBool` | `fmt.FieldText`, `fmt.FieldInt`, `fmt.FieldBool` |
| `orm.ConstraintPK \| orm.ConstraintUnique` (bitmask) | `PK: true, Unique: true` (bool fields) |
| `m.Values() []any` | `fmt.ReadValues(m.Schema(), m.Pointers())` |
| `var User_ = struct{...}` (some) | `var User_ = struct{...}` (standardized `_` suffix) |

### Target fmt.Field Struct (`tinywasm/fmt`)

```go
type Field struct {
    Name    string
    Type    FieldType // FieldText, FieldInt, FieldFloat, FieldBool, FieldBlob, FieldStruct
    PK      bool
    Unique  bool
    NotNull bool
    AutoInc bool
    Input   string // UI hint for form layer
    JSON    string // JSON key ("email,omitempty"). Empty = use Field.Name
}
```

### ORM Model Interface (new)

```go
type Model interface {
    fmt.Fielder           // Schema() []fmt.Field + Pointers() []any
    TableName() string
}
```

### Generated Code per Struct (`ormc`)

- `TableName() string`, `FormName() string`
- `Schema() []fmt.Field`, `Pointers() []any`
- `T_` metadata struct with typed column constants
- `ReadOneT(qb *orm.QB, model *T) (*T, error)`
- `ReadAllT(qb *orm.QB) ([]*T, error)`

### ORM Core API (unchanged)

- `DB`: `Create`, `Update(m, cond, rest...)`, `Delete(m, cond, rest...)`, `Query`, `Tx`, `CreateTable`, `DropTable`
- `QB` (Fluent): `.Where("col")`, `.Limit(n)`, `.Offset(n)`, `.OrderBy("col")`, `.GroupBy("cols...")`
- `Clause`: `.Eq()`, `.Neq()`, `.Gt()`, `.Gte()`, `.Lt()`, `.Lte()`, `.Like()`, `.In()`
- Safety: `Update` and `Delete` require at least one condition (compile-time enforced)

---

## Stage 1 — Regenerate ORM Code

**File**: `models_orm.go` (auto-generated)

1. Ensure `ormc` CLI is updated: `go install github.com/tinywasm/orm/cmd/ormc@latest`
2. Run `ormc` from project root to regenerate `models_orm.go`
3. Verify the generated file now uses `fmt.Field` with bool constraints
4. Verify `Values()` method is no longer generated
5. Verify meta struct uses `_` suffix consistently (e.g., `User_`, `Session_`, `Role_`)

---

## Stage 2 — Update Handwritten Code

**Files**: `crud.go`, `user_back.go`, `sessions.go`, `oauth.go`, `identities.go`, `lan_ips.go`, `migrate.go`, `user_rbac_mutations.go`, `cache.go`, `auth.go`, `models_crud_back.go`

1. Search for any `.Values()` calls on model instances — replace with `fmt.ReadValues(m.Schema(), m.Pointers())`
2. Search for direct `orm.Field` references in handwritten code — replace with `fmt.Field`
3. Search for `orm.TypeText`, `orm.TypeInt64`, etc. — replace with `fmt.FieldText`, `fmt.FieldInt`, etc.
4. Search for `orm.Constraint*` bitmask references — replace with bool field access (e.g., `f.PK`, `f.Unique`)
5. Add `"github.com/tinywasm/fmt"` import where needed
6. Remove unused `"github.com/tinywasm/orm"` imports (keep if `orm.DB`, `orm.QB`, `orm.Eq` etc. are still used)

> **Note**: Query builder API (`db.Query()`, `.Where()`, `.Eq()`, `orm.Eq()`, `ReadAll*`, `ReadOne*`) is unchanged. Only field metadata changed.

---

## Stage 3 — Update go.mod

1. Run `go mod tidy`
2. Ensure `tinywasm/fmt` is at latest version
3. Ensure `tinywasm/orm` is at latest version

---

## Verification

```bash
gotest
```

## Linked Documents

- [ARCHITECTURE.md](ARCHITECTURE.md)
- [SKILL.md](SKILL.md)
