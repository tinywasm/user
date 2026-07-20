---
PLAN: "refactor: user como módulo reutilizable — IDGenerator/events inyectados, ops neutrales, codec agnóstico y vista admin de usuarios"
TAG: v0.1.0
STATUS: review
SESSION: 12999216777245032472
PR: https://github.com/tinywasm/user/pull/19
---

> This plan is dispatched via the CodeJob workflow. See skill: agents-workflow.
> **Contexto:** la fase previa (drift `orm v0.11`) ya cerró y publicó como `v0.0.37` (PR #18) —
> este repo YA está en `orm v0.11.1`/`ddl v0.0.4`/`unixid v0.2.24`. Este plan es el siguiente
> dispatch, **rompe API de consumidores** y publica `v0.1.0`.

# PLAN — tinywasm/user: alinearse al patrón de módulos reutilizables + vista admin

Eres un agente **sin contexto previo** y **solo tienes este repositorio** (`tinywasm/user`). Este
plan es autocontenido. Patrón de referencia (lectura opcional, el contrato está inline):
[`REUSABLE_MODULES_MASTER_PLAN.md`](https://github.com/tinywasm/app/blob/main/docs/REUSABLE_MODULES_MASTER_PLAN.md)
e implementación de referencia [`veltylabs/item_catalog`](https://github.com/veltylabs/item_catalog).

## 1. Objetivo y naturaleza del módulo

`user` es un módulo **híbrido**: dominio (identidad, RBAC, sesiones, IPs LAN) **y** transporte de
autenticación (flujos HTTP de login/logout/OAuth con cookies — esa ES su responsabilidad única de
transporte, no una violación). Debe poder enchufarse en cualquier app del ecosistema — cuya parte
compartida compila a **WASM/TinyGo** — sin arrastrar implementaciones concretas, **y debe entregar
su vista de administración de usuarios** (`NewView`) para que la app la monte como cualquier otro
módulo. Hoy `authority/` viola el patrón en cuatro puntos, todos verificados:

| Violación | Dónde (exacto) | Destino |
|---|---|---|
| Construye `unixid.NewUnixID()` internamente | `authority/identities.go`, `lan.go`, `oauth.go`, `sessions.go`, `users.go` (1 vez c/u) | `model.IDGenerator` inyectado vía `user.Config.IDs` |
| Callback ad-hoc `Config.OnSecurityEvent func(SecurityEvent)` | `user.go` + `authority/` (`m.notify`) | `events.Publisher` inyectado (`Config.Events`) + topic constante |
| Tools MCP concretas: `Tools() []mcp.Tool` con `json.Encode` y `tinywasm/context` | `authority/tools.go` | `router.OpModule` — `MountOps(router.OpRegistry)`, respuesta vía `ctx.Encode` |
| Codec concreto en transporte propio: `json.Decode(string(ctx.Body()), …)` | `authority/mount.go` | `router.Context.Decode` (el host inyecta el codec) |

Y le falta una capacidad que el consumidor (`mjosefa-cms`) requiere: **no expone vista** — ni ops
CRUD de usuarios montables — así que ninguna app puede administrar usuarios con el patrón estándar.
Este plan la añade (§3.6–§3.7).

**Excepciones documentadas — NO las "arregles"** (anti-footgun; son server-only y legítimas):

- `MountAPI`/`router.APIModule` en `authority/mount.go` **se queda**: los flujos cookie
  (login/logout/callback OAuth) son rutas HTTP reales, no ops neutrales. `user` es el módulo de
  transporte auth del ecosistema; el master plan lo lista explícitamente como `APIModule`.
- `tinywasm/jwt` y `golang.org/x/crypto` (`api_token.go`, `middleware.go`, hashing): formatos y
  criptografía **son el dominio** de este módulo. Server-only, jamás importados por el paquete raíz.
- `tinywasm/fetch` + `tinywasm/json` en `authority/oauth.go`: hablan el **protocolo externo** del
  proveedor OAuth (JSON literal en el wire, como `postgres` habla SQL). No es el codec interno del
  ecosistema. Quedan confinados a `oauth.go`.
- El paquete **raíz** (`user`) es la parte compartida WASM/TinyGo: hoy importa solo
  `model`/`orm`/`form/input`/`fmt`; este plan añade `events`/`router`/`view` — **contratos puros,
  wasm-safe, permitidos**. Ninguna etapa puede añadirle stdlib, `reflect`, mapas nuevos ni
  implementaciones concretas.

## 2. Contratos (referencia congelada — no los reimplementes)

```go
// tinywasm/model v0.0.16
type IDGenerator interface { NewID() string }

// tinywasm/events v0.0.2
type Event struct { Topic string; Payload model.Encodable }
type Publisher interface { Publish(e Event) }   // fire-and-forget

// tinywasm/router v0.1.15
type OpRegistry interface { Op(name string, h HandlerFunc) Route }
type OpModule interface { model.ModuleNaming; MountOps(reg OpRegistry) }
// Route (cadena): .Requires(resource model.Resource, action model.Action) Route
//                 .Authenticated() Route   // cualquier identidad, sin chequeo de permiso
//                 .Accepts(args model.Fielder) Route  // opcional; sin llamada = sin args
// Context: .Decode(into model.Decodable) error / .Encode(from model.Encodable) error / .UserID() string

// tinywasm/view v0.1.1 (API PUBLICADA — si un checkout local difiere, manda la publicada)
func New(
	caller router.Caller,
	record model.Model,
	listOp string,
	newList func() model.FielderSlice,
	project func(list model.FielderSlice) []Item,
	opts ...Option,
) Presenter
// Item{ID, Label, Description string}
// Options: WithTitle, WithSearchPlaceholder, WithSaveOp, WithDeleteOp, WithFill(func(id) model.Model), WithArgs
// Save envía el record completo al saveOp; Delete envía el RECORD COMPLETO (no un {id}) al deleteOp.
```

Referencia completa de vista + ops: `view.go` y `mcp.go` de
[`veltylabs/item_catalog`](https://github.com/veltylabs/item_catalog) — copia la forma exacta.

## 3. Cambios exactos

### 3.1 `user.go` (raíz, compartido WASM) — costuras en `Config`, topic y nombres de op

`Config` gana **dos campos** (decisión explícita: una sola estructura de parámetro, no un par
`cfg, deps`):

```go
type Config struct {
	// … campos existentes sin cambios …

	// IDs mints primary keys for every record this module creates (users, sessions,
	// oauth states, identities, LAN ips). REQUIRED: authority.New fails if nil —
	// an auth module must never silently pick its own generator.
	IDs model.IDGenerator

	// Events receives security events (user.TopicSecurity). Optional: nil = events
	// are dropped (fire-and-forget contract), never an error.
	Events events.Publisher
}
```

- **Elimina** `OnSecurityEvent func(SecurityEvent)` de `Config` (breaking; ver §5 impacto).
- Topic y **nombres de op en la raíz** (vocabulario compartido wasm↔server — la vista wasm los
  necesita y NO puede importar `authority/`, que arrastra `jwt`/`x/crypto` al binario):
  ```go
  // TopicSecurity is the events topic every SecurityEvent is published on.
  const TopicSecurity = "user.security"

  // Op names — shared vocabulary between the wasm view and the server module.
  const (
  	OpMe         = "me"           // authenticated caller's profile
  	OpListUsers  = "list_users"   // admin: list users
  	OpUpsertUser = "upsert_user"  // admin: create (Id=="") or update
  	OpDeleteUser = "delete_user"  // admin: delete by record
  )
  ```
- `SecurityEvent` debe implementar `model.Encodable` para viajar como `Event.Payload`: añade a mano
  `IsNil() bool` y `EncodeFields(w model.FieldWriter)` (escribe `Type` como número, resto strings,
  `Timestamp` como int64). Sin `reflect`, sin stdlib — el archivo es compartido WASM.
- Import nuevo en la raíz: `github.com/tinywasm/events` (contrato puro, wasm-safe).

### 3.2 `authority/module.go` — guardar las costuras y publicar

- `New` valida: si `cfg.IDs == nil` retorna error (mensaje verbatim: `fmt.Err("user:", "Config.IDs",
  "is", "required")`). Fail-fast: sin generador no hay auth — nunca un default silencioso.
- Guarda `m.ids = cfg.IDs` y `m.events = cfg.Events` en `Module`.
- `m.notify(e user.SecurityEvent)` pasa de invocar el callback a:
  ```go
  func (m *Module) notify(e user.SecurityEvent) {
  	if m.events == nil {
  		return
  	}
  	e.Timestamp = time.Now().Unix()
  	m.events.Publish(events.Event{Topic: user.TopicSecurity, Payload: &e})
  }
  ```
  (si `notify` hoy vive en otro archivo, edítalo donde esté — búscalo con `grep -rn "func.*notify" authority/`).

### 3.3 Los 5 constructores internos de `unixid` → `m.ids`

En `authority/identities.go`, `lan.go`, `oauth.go`, `sessions.go`, `users.go`: cada bloque

```go
u, err := unixid.NewUnixID()
if err != nil { … }
id := u.NewID()
```

se reemplaza por `id := m.ids.NewID()` (sin error que manejar — el generador ya fue validado en
`New`). Quita el import `"github.com/tinywasm/unixid"` de los cinco archivos. Si alguna de esas
funciones no es método de `Module` (helper suelto, p. ej. `createUser(db, …)`), pásale
`ids model.IDGenerator` como parámetro desde el método que la llama — no un global.

### 3.4 `authority/tools.go` → `authority/ops.go` — transporte neutral

Borra `tools.go` completo y crea `ops.go` (los nombres de op vienen de la raíz, §3.1):

```go
package authority

import (
	"github.com/tinywasm/model"
	"github.com/tinywasm/router"
	"github.com/tinywasm/user"
)

var _ router.OpModule = (*Module)(nil)

func (m *Module) MountOps(reg router.OpRegistry) {
	reg.Op(user.OpMe, m.opMe).Authenticated()
	reg.Op(user.OpListUsers, m.opListUsers).Requires("users", model.Read)
	reg.Op(user.OpUpsertUser, m.opUpsertUser).Requires("users", model.Create|model.Update).Accepts(&user.User{})
	reg.Op(user.OpDeleteUser, m.opDeleteUser).Requires("users", model.Delete).Accepts(&user.User{})
}

func (m *Module) opMe(ctx router.Context) {
	userID := ctx.UserID()
	if userID == "" {
		ctx.WriteStatus(401)
		return
	}
	u, err := m.GetUser(userID)
	if err != nil {
		ctx.WriteStatus(404)
		return
	}
	profile := user.ProfileDTO{Id: u.Id, Name: u.Name, Email: u.Email}
	for _, r := range u.Roles {
		profile.Roles = append(profile.Roles, r.Code)
	}
	profile.Permissions = permissionsOf(u)
	if err := ctx.Encode(&profile); err != nil {
		ctx.WriteStatus(500)
	}
}
```

- `.Authenticated()` preserva la semántica actual del tool `me` (`model.AccessAuthenticated`:
  cualquier identidad, sin chequeo de recurso) — **no** uses `.Requires("user", model.Read)`: eso
  denegaría a todo usuario no-admin leer su propio perfil.
- Las ops de admin usan el recurso **`"users"`** — el MISMO `HandlerName()` que el `userCRUD` de
  `authority/crud.go` ya registra vía RBAC (`registerRBAC` siembra `users:read`, `users:create`,
  …); no inventes un recurso nuevo.
- `permissionsOf` se conserva tal cual (muévelo a `ops.go`).
- La identidad se lee de `ctx.UserID()` (canal tipado del router) — ya no existe `mcp.CtxKeyUserID`.
- `user.ProfileDTO` debe implementar `model.Encodable` — verifica; si le falta `EncodeFields`,
  añádelo a mano en la raíz (mismas restricciones WASM de §3.1).

### 3.5 `authority/mount.go` — codec vía `Context`

`json.Decode(string(ctx.Body()), data)` → `ctx.Decode(data)` (todas las ocurrencias del archivo).
Quita el import `"github.com/tinywasm/json"` de `mount.go`. El resto del archivo (rutas, cookies,
redirects, `Write`/`WriteStatus`) no cambia.

### 3.6 `authority/ops.go` — handlers CRUD de usuarios (reusa los helpers existentes)

Los helpers ya existen en `authority/users.go` (`createUser`/`updateUser`/`deleteUser`/`listUsers`)
y `authority/crud.go` los envuelve para el RBAC seed — **no dupliques lógica**, llama los mismos:

```go
func (m *Module) opListUsers(ctx router.Context) {
	us, err := listUsers(m.db)
	if err != nil {
		ctx.WriteStatus(500)
		return
	}
	list := make(user.UserList, 0, len(us))
	for i := range us {
		list = append(list, &us[i])
	}
	if err := ctx.Encode(&list); err != nil {
		ctx.WriteStatus(500)
	}
}

func (m *Module) opUpsertUser(ctx router.Context) {
	var u user.User
	if err := ctx.Decode(&u); err != nil {
		ctx.WriteStatus(400)
		return
	}
	if u.Id == "" {
		if _, err := createUser(m.db, u.Email, u.Name, u.Phone); err != nil {
			ctx.WriteStatus(500)
		}
		return
	}
	if err := updateUser(m.db, m.cache, u.Id, u.Name, u.Phone); err != nil {
		ctx.WriteStatus(500)
	}
}

func (m *Module) opDeleteUser(ctx router.Context) {
	var u user.User
	if err := ctx.Decode(&u); err != nil {
		ctx.WriteStatus(400)
		return
	}
	if err := deleteUser(m.db, m.cache, u.Id); err != nil {
		ctx.WriteStatus(500)
	}
}
```

- Ajusta los receptores a los campos reales del `Module` (los mismos que `userCRUD` recibe en
  `crud.go` — `db`, `cache`; verifica sus nombres en `module.go`). Si `createUser` gana el
  parámetro `ids` en §3.3, pásalo aquí también.
- Semántica upsert = `item_catalog`: `Id==""` crea, si no actualiza. `updateUser` NO toca email
  (decisión existente del dominio) — no lo "arregles".
- **Seguridad, verificado:** `user.User{Id, Email, Name, Phone, Status, CreatedAt, Roles,
  Permissions}` **no** transporta credenciales (viven en `Identity`) — puede viajar a una vista
  admin gated por `Requires("users", …)`. No añadas campos de password a estas ops.

### 3.7 Vista — `view.go` en la RAÍZ (paquete `user`, compartido WASM)

Archivo nuevo `view.go` en la raíz. Es la vista que `mjosefa-cms` montará (el renderer lo pone la
app, vía `crudview` — aquí solo el `Presenter`). Copia la forma de `item_catalog/view.go`:

```go
package user

import (
	"github.com/tinywasm/model"
	"github.com/tinywasm/router"
	"github.com/tinywasm/view"
)

// NewView builds the user-administration Presenter — the tech-agnostic engine a
// renderer (crudview, or any other) wraps. The app decides which renderer draws it.
func NewView(caller router.Caller) view.Presenter {
	var byID []*User
	record := &User{}

	return view.New(
		caller,
		record,
		OpListUsers,
		func() model.FielderSlice { return &UserList{} },
		func(list model.FielderSlice) []view.Item {
			l := list.(*UserList)
			items := make([]view.Item, l.Len())
			byID = make([]*User, l.Len())
			for i := 0; i < l.Len(); i++ {
				it := l.At(i).(*User)
				byID[i] = it
				items[i] = view.Item{ID: it.Id, Label: it.Name, Description: it.Email}
			}
			return items
		},
		view.WithTitle("Usuarios"),
		view.WithSaveOp(OpUpsertUser),
		view.WithDeleteOp(OpDeleteUser),
		view.WithFill(func(id string) model.Model {
			if id == "" {
				return nil
			}
			for _, it := range byID {
				if it != nil && it.Id == id {
					return it
				}
			}
			return nil
		}),
	)
}
```

- **Widgets del formulario:** `_schemaUser` en `models_orm.go` usa `model.Text()` plano; para que
  el form del renderer dibuje widgets correctos, los campos editables pasan a Kind de
  `form/input` (el import ya existe en la raíz): `name` → `input.Text()`, `email` →
  `input.Email()`, `phone` → `input.Phone()`. `id`/`status`/`created_at` quedan como están (el
  form no edita PK ni timestamps). Mismo patrón que `CatalogItemModel` en `item_catalog/model.go`.
- `UserList` ya cumple lo que `view`/codec necesitan (`Len/At/Append/IsNil/EncodeFields` —
  verificado, idéntico a `CatalogItemList`). No lo regeneres.

### 3.8 `go.mod`

- **Quita**: `github.com/tinywasm/mcp`, `github.com/tinywasm/unixid`, `github.com/tinywasm/context`
  (mueren con `tools.go` y §3.3).
- **Añade**: `github.com/tinywasm/events v0.0.2`, `github.com/tinywasm/view v0.1.1`.
- **Bump**: `github.com/tinywasm/router` → `v0.1.15` (necesario por `Context.Decode/Encode` y
  `OpRegistry`). `orm v0.11.1`/`ddl v0.0.4`/`model v0.0.16` ya están (los trajo `v0.0.37`) — no
  los toques.
- `github.com/tinywasm/json` y `github.com/tinywasm/fetch` **se quedan** (solo `oauth.go`, §1).
- `go mod tidy` (también en `tests/go.mod`).

## 4. Criterios de aceptación

- `grep -rn "tinywasm/unixid" . --include="*.go"` **vacío** (tests incluidos: usa un stub
  `func literal` que satisface `model.IDGenerator`).
- `grep -rn "tinywasm/mcp\|tinywasm/context" . --include="*.go"` **vacío**.
- `grep -rn "tinywasm/json" . --include="*.go"` → **solo** `authority/oauth.go`.
- `grep -rn "OnSecurityEvent" . --include="*.go"` **vacío**.
- `var _ router.OpModule = (*Module)(nil)` y `var _ router.APIModule = (*Module)(nil)` compilan
  ambos (mount.go intacto).
- Las 4 ops montadas: `me` (`Authenticated`), `list_users`/`upsert_user`/`delete_user`
  (`Requires("users", …)`) — un test con `router/mock` verifica op enrutada + gate RBAC.
- `user.NewView(caller)` devuelve un `Presenter` que lista/selecciona/guarda/elimina contra un
  `router.Caller` falso (o `view/conformance.FakeCaller`) — test en el repo, ambos targets
  (stdlib + `//go:build wasm`).
- El paquete raíz `user` importa únicamente `model`/`orm`/`form/input`/`fmt`/`events`/`router`/
  `view` — compilable por TinyGo; jamás `authority`, `jwt`, `crypto`, `json`, `fetch`.
- Tests de eventos con `events/mock.Broker`: un login fallido publica en `user.TopicSecurity`.
- `go build ./...` y tests verdes (raíz y `tests/`).

## 5. Impacto en consumidores (documentar en CHANGELOG del PR, no ejecutar aquí)

`mjosefa-cms` (y cualquier app): (a) pasa `IDs`/`Events` en `user.Config` — los mismos que ya
inyecta a sus módulos de dominio; (b) reemplaza `OnSecurityEvent: fn` por
`broker.Subscribe(user.TopicSecurity, fn)`; (c) deja de añadir `auth` como `mcp.ToolProvider`
aparte — entra por la misma cosecha `mcp.HarvestOps` que los demás `OpModule`; (d) monta la vista
de administración como cualquier módulo: `crudview.New(Config{ParentID: "users", Presenter:
user.NewView(caller)})`, con `"users"` en `config.Modules()` y permisos `users:*` para el rol
admin (ya sembrados por `registerRBAC`).

## 6. Etapas

| # | Etapa | Archivos | Criterio |
|---|---|---|---|
| 1 | Costuras + topic + ops consts + Encodable | `user.go` | `IDs`/`Events` en Config; sin `OnSecurityEvent`; `TopicSecurity` + `Op*`; `SecurityEvent` Encodable |
| 2 | Módulo guarda costuras | `authority/module.go` | `New` falla con `IDs` nil; `notify` publica |
| 3 | IDs inyectados | `authority/{identities,lan,oauth,sessions,users}.go` | `grep unixid` vacío |
| 4 | Ops neutrales (`me`) | `authority/tools.go` → `ops.go` | `OpModule` compila; sin mcp/context/json |
| 5 | Ops CRUD usuarios | `authority/ops.go` | 3 ops gated `Requires("users", …)`; reusa helpers |
| 6 | Codec en transporte | `authority/mount.go` | `ctx.Decode`; json solo en oauth.go |
| 7 | Vista + widgets | `view.go` (raíz), `models_orm.go` | `NewView` estilo item_catalog; Kinds en name/email/phone |
| 8 | Deps | `go.mod`, `go.sum`, `tests/go.mod` | −mcp −unixid −context +events +view; router v0.1.15 |
| 9 | Verde | tests | mock.Broker + stub IDGenerator + FakeCaller; `go build ./...` |
