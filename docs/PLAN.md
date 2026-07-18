---
PLAN: "fix: migrar authority a orm v0.11 (ddl.Sync + ErrNotFound) y SchemaExt a model.FieldExt"
TAG: v0.0.37
---

> This plan is dispatched via the CodeJob workflow. See skill: agents-workflow.

# PLAN — tinywasm/user: adoptar la API actual de `orm`/`ddl`/`model`

Eres un agente **sin contexto previo** y **solo tienes este repositorio** (`tinywasm/user`). Este
plan es autocontenido.

> **Fase 1 de 2.** `user` es un módulo del ecosistema de módulos reutilizables
> ([`REUSABLE_MODULES_MASTER_PLAN.md`](https://github.com/tinywasm/app/blob/main/docs/REUSABLE_MODULES_MASTER_PLAN.md))
> y debe terminar alineado a sus cinco contratos (IDs/events inyectados, ops neutrales, codec
> agnóstico, WASM/TinyGo-safe). Esa alineación **rompe API** y está en cola como
> [`PLAN_MODULO_REUTILIZABLE.md`](PLAN_MODULO_REUTILIZABLE.md) (`v0.1.0`), gated hasta que ESTA
> fase (`v0.0.37`, solo el drift de compilación) cierre y el consumidor `mjosefa-cms` elimine sus
> `replace` locales. No adelantes nada de la Fase 2 aquí.

## 1. Problema

Este repo compila hoy contra `github.com/tinywasm/orm v0.9.28` y `model v0.0.14` (su propio
`go.mod`), pero sus consumidores (p. ej. `veltylabs/mjosefa-cms`) resuelven `orm v0.11.1` y
`model v0.0.16`. En `orm v0.11.x` la migración de esquema **salió del ORM** hacia el paquete
hermano `github.com/tinywasm/ddl`, y el error de "sin filas" se renombró. Resultado: cualquier
consumidor con el orm nuevo ve estos errores de compilación, literales:

```
authority/migrate.go:19:16: db.CreateTable undefined (type *orm.DB has no field or method CreateTable)
authority/rbac.go:193:19: undefined: orm.ErrNoRows
authority/rbac.go:237:44: undefined: orm.ErrNoRows
```

Gravedad extra: el consumidor lo está parcheando con un fork local del orm cuyo `CreateTable` es un
**no-op**, lo que desactiva en silencio la creación del esquema de autenticación. Este plan elimina
la causa raíz.

Además `models_orm.go` usa `ddlc.FieldExt` para las FKs; ese contrato se movió a
`model.FieldExt` (idéntica forma: `{Field, Ref, RefColumn, OnDelete}`) en `model v0.0.15+`, y la
dependencia `ddlc` deja de ser necesaria.

## 2. Contratos nuevos (referencia congelada — no los reimplementes)

`github.com/tinywasm/ddl` (usar `v0.0.4`+):

```go
func New(conn storage.Conn, ddlCompiler Compiler) *DB     // dos argumentos
func (d *DB) CreateTable(m model.Model) error             // CREATE TABLE idempotente
func (d *DB) Sync(models ...model.Model) error            // crea tablas faltantes + ADD COLUMN aditivo, transaccional si el backend lo soporta
func TopologicalSort(models []model.Model) ([]model.Model, error) // ordena padres antes que hijos por SchemaExt()
```

- `ddl.Compiler` es una **capacidad opcional del backend**: la implementan los backends SQL
  (`tinywasm/postgres`, `tinywasm/sqlt`) pero **no** `storage/mem` (el backend de los tests), que
  crea tablas perezosamente en el primer `Exec` y no necesita DDL. El idioma correcto es un *type
  assertion* sobre `db.RawConn()` — si el backend no compila DDL, migrar es un no-op **legítimo**
  (mismo idioma que ya usa `veltylabs/item_catalog` y que `storage.TxExecutor` usa para
  transacciones opcionales).
- `orm v0.11.x`: `*orm.DB` conserva `Create/Update/Delete/Query/Tx/Close/RawConn/SetLog`. El error
  de "sin filas" es **`orm.ErrNotFound`** (`ErrNoRows` ya no existe).
- `model v0.0.16`: `model.FieldExt{Field model.Field; Ref, RefColumn, OnDelete string}` — mismo
  significado que el viejo `ddlc.FieldExt`.

## 3. Cambios exactos

### 3.1 `go.mod`

- Bump: `github.com/tinywasm/orm` → `v0.11.1`, `github.com/tinywasm/model` → `v0.0.16`.
- **Añade** `github.com/tinywasm/ddl v0.0.4` (dependencia directa nueva).
- **Quita** `github.com/tinywasm/ddlc` (directa) — tras 3.3 nadie la importa.
- `go mod tidy`. Criterio: `grep -n "ddlc" go.mod` **vacío**.

### 3.2 `authority/migrate.go` — reescribir `initSchema`

Sustituye el cuerpo completo por la versión `ddl` (la lista de modelos y el condicional de
`Session` se preservan tal cual):

```go
package authority

import (
	"github.com/tinywasm/ddl"
	"github.com/tinywasm/model"
	"github.com/tinywasm/orm"
	"github.com/tinywasm/user"
)

func initSchema(db *orm.DB, mode user.AuthMode) error {
	models := []model.Model{
		&user.User{}, &user.Role{}, &user.Permission{},
		&user.Identity{}, &user.LANIP{},
		&user.OAuthState{}, &user.UserRole{}, &user.RolePermission{},
	}
	if mode == user.AuthModeCookie {
		models = append(models, &user.Session{})
	}
	ddlCompiler, ok := db.RawConn().(ddl.Compiler)
	if !ok {
		// Backend sin capacidad DDL (p. ej. storage/mem en tests): crea tablas
		// perezosamente en el primer Exec — no-op correcto, NO un error.
		return nil
	}
	sorted, err := ddl.TopologicalSort(models)
	if err != nil {
		return err
	}
	return ddl.New(db.RawConn(), ddlCompiler).Sync(sorted...)
}
```

Anti-footgun: **no** llames `Sync` sin el `TopologicalSort` — `Sync` procesa en el orden dado y
las tablas con FK (`Identity`, `UserRole`, `RolePermission`, `Session`) necesitan sus padres
(`user`, `role`, `permission`) creados antes.

### 3.3 `models_orm.go` — `ddlc.FieldExt` → `model.FieldExt`

Cambio mecánico, sin tocar valores:

- Quita el import `"github.com/tinywasm/ddlc"`.
- Todo `func (m *X) SchemaExt() []ddlc.FieldExt { return []ddlc.FieldExt{...} }` pasa a
  `[]model.FieldExt` (afecta `Session`, `Identity`, `UserRole`, `RolePermission` — verifica con
  `grep -n "SchemaExt" models_orm.go` que no quede ninguno con `ddlc`).
- El contenido de cada `FieldExt` (Field/Ref/RefColumn/OnDelete) queda idéntico.

### 3.4 `authority/rbac.go` — `orm.ErrNoRows` → `orm.ErrNotFound`

Dos sitios, exactos:

- `GetRoleByCode` (~línea 193): `return nil, orm.ErrNoRows` → `return nil, orm.ErrNotFound`.
- `HasPermission` (~línea 237): `if err == user.ErrNotFound || err == orm.ErrNoRows {` →
  `if err == user.ErrNotFound || err == orm.ErrNotFound {`. **Conserva** la comparación con
  `user.ErrNotFound` — son errores distintos de paquetes distintos, ambos válidos.

## 4. Fuera de alcance

- **No** cambies la construcción directa de `unixid.NewUnixID()` en `authority/` — inyectar
  `model.IDGenerator` es la Fase 2 ([`PLAN_MODULO_REUTILIZABLE.md`](PLAN_MODULO_REUTILIZABLE.md));
  este plan solo cierra el drift de compilación.
- **No** toques flujos de auth (login, cookies, OAuth, RBAC semántico), ni `form`/`fetch`/vistas.
- **No** añadas migraciones destructivas ni versionadas — `ddl.Sync` aditivo es el contrato actual.
- **No** re-exportes ni envuelvas `ddl` en un helper local: se usa directo en `migrate.go`.

## 5. Criterios de aceptación

- `grep -rn "CreateTable" . --include="*.go"` solo aparece, si acaso, dentro de la llamada
  `ddl.New(...).Sync(...)`/comentarios — **ninguna** llamada a método `CreateTable` sobre `*orm.DB`.
- `grep -rn "ErrNoRows" . --include="*.go"` **vacío**.
- `grep -rn "ddlc" . --include="*.go"` **vacío**; `go.mod` sin `ddlc`.
- `go build ./...` y los tests del repo verdes con `orm v0.11.1` + `model v0.0.16` + `ddl v0.0.4`.
- Un consumidor con `orm v0.11.1` compila `user/authority` sin `replace` (verificación del
  integrador, no de este repo).

## 6. Etapas

| # | Etapa | Archivos | Criterio |
|---|---|---|---|
| 1 | Bump deps | `go.mod`, `go.sum` | orm v0.11.1, model v0.0.16, +ddl v0.0.4, −ddlc |
| 2 | FieldExt | `models_orm.go` | `grep ddlc` vacío; `SchemaExt() []model.FieldExt` |
| 3 | Migración esquema | `authority/migrate.go` | type-assert `ddl.Compiler` + `TopologicalSort` + `Sync` |
| 4 | Error renombrado | `authority/rbac.go` | `grep ErrNoRows` vacío; `user.ErrNotFound` intacto |
| 5 | Verde | todo | `go build ./...` + tests verdes |
