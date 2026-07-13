← [Master](PLAN.md) | Next → [Stage 2](PLAN_STAGE_2_JSON_WIRE.md)

# Stage 1 — `models.go` to the Kind API, regenerate with `ormc` (GATE)

This stage is a **gate**: it renames generated struct fields, so the repo does
not compile until every call site is updated. Land it complete or not at all.

## The contract change

`tinywasm/model` removed `Field.Widget`. The kind now lives in the single
`Type:` slot as a **constructor expression**. The generator refuses the old
form with this exact error:

```
field <name>: Field.Widget was removed (Kind unification): declare the kind in Type — e.g. Type: input.Email()
```

Kind vocabulary (reference:
<https://github.com/tinywasm/ormc/blob/main/README.md>):

| Old | New |
|---|---|
| `Type: model.FieldText` | `Type: model.Text()` |
| `Type: model.FieldInt` | `Type: model.Int()` |
| `Type: model.FieldText, Widget: input.Email()` | `Type: input.Email()` |
| `Type: model.FieldStructSlice, Ref: &RoleModel` | `Type: model.StructSlice(&RoleModel)` |

Rules that follow from it:

- A **form kind** (`input.Email()`, `input.Password()`, `input.Text()`,
  `input.Phone()`) both validates and renders. A **base kind**
  (`model.Text()`, `model.Int()`) validates only. Choose the form kind wherever
  the old field had a `Widget`.
- `Ref:` now has **exactly one** meaning: a **scalar foreign key**
  (`{Name: "user_id", Type: model.Text(), DB: &model.FieldDB{RefColumn: "id"}, Ref: &UserModel}`).
  For composition, the `*Definition` goes **inside** the constructor
  (`model.StructSlice(&RoleModel)`), never in `Ref:`.
- `Exclude: true` stays as-is.

## Work

1. **Rewrite every `model.Definition` in [models.go](../models.go)** to the Kind
   API. All eleven models: `UserModel`, `SessionModel`, `IdentityModel`,
   `RoleModel`, `UserRoleModel`, `PermissionModel`, `RolePermissionModel`,
   `LANIPModel`, `OAuthStateModel`, and the four form DTOs (`LoginDataModel`,
   `RegisterDataModel`, `ProfileDataModel`, `PasswordDataModel`).

   The DTOs keep their form kinds — this is what makes `form.New` work:

   ```go
   var LoginDataModel = model.Definition{
       Name: "login_data",
       Fields: model.Fields{
           {Name: "email", Type: input.Email(), NotNull: true},
           {Name: "password", Type: input.Password(), NotNull: true},
       },
   }
   ```

   The DB models keep their `DB:` role and lose `Widget` (they never had one):

   ```go
   var UserModel = model.Definition{
       Name: "user",
       Fields: model.Fields{
           {Name: "id", Type: model.Text(), DB: &model.FieldDB{PK: true}},
           {Name: "email", Type: model.Text(), DB: &model.FieldDB{Unique: true}},
           // ...
           {Name: "roles", Type: model.StructSlice(&RoleModel), Exclude: true},
           {Name: "permissions", Type: model.StructSlice(&PermissionModel), Exclude: true},
       },
   }
   ```

   The four DTOs are **form/codec models with no `Field.DB`** — they are not
   tables. If `ormc` cannot generate a table-less Definition: **STOP and
   report** (the fix belongs in `tinywasm/ormc`).

2. **Bump the deps** to the versions that carry the new contract, then
   regenerate:

   ```bash
   go get github.com/tinywasm/model@latest github.com/tinywasm/orm@latest \
          github.com/tinywasm/form@latest github.com/tinywasm/json@latest \
          github.com/tinywasm/router@latest github.com/tinywasm/mcp@latest
   go install github.com/tinywasm/ormc/cmd/ormc@latest
   go generate ./...   # generate.go declares //go:generate ormc
   ```

   `ormc` now lives in **its own module** (`github.com/tinywasm/ormc`), not
   under `tinywasm/orm`.

3. **`ID` → `Id` is the accepted contract.** The generator pascal-cases column
   names without an initialism special case, so `models_orm.go` emits `Id`, not
   `ID` — for the struct field **and** the typed query helper (`User_.Id`).
   This is a deliberate breaking change to this library's public API. Update
   every call site: `server/` (all files), `web/client.go`, `tests/`. Grep
   afterwards:

   ```
   grep -rn "\.ID\b\|_\.ID\b" --include=*.go .   # → empty
   ```

   Do **not** hand-edit `models_orm.go` — it is generated (`// DO NOT EDIT`).
   If the generated code is wrong, the bug is in `ormc`: **STOP and report.**

4. `models_orm.go` will now import `github.com/tinywasm/form/input`, because
   the generated `Schema()` re-emits every `Type:` constructor **verbatim**.
   That import is **correct and expected** — do not remove it. (If it appears
   while the definitions still use `Widget:`, it is unused and the build
   breaks — that is the symptom of an un-migrated `models.go`, not an `ormc`
   bug.)

## Acceptance

- `grep -rn "Widget:\|model.FieldText\|model.FieldInt\|model.FieldStructSlice" .`
  → **empty**.
- `grep -rn "\.ID\b" --include=*.go .` → **empty**.
- `go build ./...` and `GOOS=js GOARCH=wasm go build ./...` succeed.
- `gotest ./...` green: the four DTOs still yield their inputs through
  `form.New` (the widget-regression test in
  `tests/production_wiring_test.go` must keep passing unchanged — 2/4/2/3
  inputs for Login/Register/Profile/Password).
