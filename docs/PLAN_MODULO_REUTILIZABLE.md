---
PLAN: "refactor: user como módulo reutilizable — IDGenerator/events inyectados, ops neutrales, codec agnóstico"
TAG: v0.1.0
---

> This plan is dispatched via the CodeJob workflow. See skill: agents-workflow.
> **Gate:** se despacha DESPUÉS de que [`docs/PLAN.md`](PLAN.md) (fix de drift, `v0.0.37`) cierre en
> verde y su consumidor (`mjosefa-cms`) haya eliminado sus `replace` locales. No fusionar ambos
> planes en un dispatch: este rompe API de consumidores; aquel es el desbloqueo urgente.

# PLAN — tinywasm/user: alinearse al patrón de módulos reutilizables

Eres un agente **sin contexto previo** y **solo tienes este repositorio** (`tinywasm/user`). Este
plan es autocontenido. Patrón de referencia (lectura opcional, el contrato está inline):
[`REUSABLE_MODULES_MASTER_PLAN.md`](https://github.com/tinywasm/app/blob/main/docs/REUSABLE_MODULES_MASTER_PLAN.md)
e implementación de referencia [`veltylabs/item_catalog`](https://github.com/veltylabs/item_catalog).

## 1. Objetivo y naturaleza del módulo

`user` es un módulo **híbrido**: dominio (identidad, RBAC, sesiones, IPs LAN) **y** transporte de
autenticación (flujos HTTP de login/logout/OAuth con cookies — esa ES su responsabilidad única de
transporte, no una violación). Debe poder enchufarse en cualquier app del ecosistema — cuya parte
compartida compila a **WASM/TinyGo** — sin arrastrar implementaciones concretas. Hoy `authority/`
viola el patrón en cuatro puntos, todos verificados:

| Violación | Dónde (exacto) | Destino |
|---|---|---|
| Construye `unixid.NewUnixID()` internamente | `authority/identities.go`, `lan.go`, `oauth.go`, `sessions.go`, `users.go` (1 vez c/u) | `model.IDGenerator` inyectado vía `user.Config.IDs` |
| Callback ad-hoc `Config.OnSecurityEvent func(SecurityEvent)` | `user.go` + `authority/` (`m.notify`) | `events.Publisher` inyectado (`Config.Events`) + topic constante |
| Tools MCP concretas: `Tools() []mcp.Tool` con `json.Encode` y `tinywasm/context` | `authority/tools.go` | `router.OpModule` — `MountOps(router.OpRegistry)`, respuesta vía `ctx.Encode` |
| Codec concreto en transporte propio: `json.Decode(string(ctx.Body()), …)` | `authority/mount.go` | `router.Context.Decode` (el host inyecta el codec) |

**Excepciones documentadas — NO las "arregles"** (anti-footgun; son server-only y legítimas):

- `MountAPI`/`router.APIModule` en `authority/mount.go` **se queda**: los flujos cookie
  (login/logout/callback OAuth) son rutas HTTP reales, no ops neutrales. `user` es el módulo de
  transporte auth del ecosistema; el master plan lo lista explícitamente como `APIModule`.
- `tinywasm/jwt` y `golang.org/x/crypto` (`api_token.go`, `middleware.go`, hashing): formatos y
  criptografía **son el dominio** de este módulo. Server-only, jamás importados por el paquete raíz.
- `tinywasm/fetch` + `tinywasm/json` en `authority/oauth.go`: hablan el **protocolo externo** del
  proveedor OAuth (JSON literal en el wire, como `postgres` habla SQL). No es el codec interno del
  ecosistema. Quedan confinados a `oauth.go`.
- El paquete **raíz** (`user`) ya cumple: solo `model`/`orm`/`form/input`/`fmt` — es la parte que
  comparte WASM/TinyGo. Ninguna etapa puede añadirle stdlib, `reflect`, mapas nuevos ni imports
  concretos.

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
// Context: .Decode(into model.Decodable) error / .Encode(from model.Encodable) error
```

Referencia de `MountOps` (así lo hace `item_catalog`, cópiale la forma):

```go
func (m *Module) MountOps(reg router.OpRegistry) {
	reg.Op(OpListItems, m.opListItems).Requires("catalog_item", model.Read).Accepts(&ListItemsArgs{})
}
var _ router.OpModule = (*Module)(nil)
```

## 3. Cambios exactos

### 3.1 `user.go` (raíz, compartido WASM) — costuras en `Config`, no un segundo struct

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
- Topic constante junto a `SecurityEvent`:
  ```go
  // TopicSecurity is the events topic every SecurityEvent is published on.
  const TopicSecurity = "user.security"
  ```
- `SecurityEvent` debe implementar `model.Encodable` para viajar como `Event.Payload`: añade a mano
  `IsNil() bool` y `EncodeFields(w model.FieldWriter)` (escribe `Type` como número, resto strings,
  `Timestamp` como int64). Sin `reflect`, sin stdlib — el archivo es compartido WASM.
- Import nuevo en la raíz: `github.com/tinywasm/events` (contrato puro, wasm-safe).

### 3.2 `authority/module.go` — guardar las costuras y publicar

- `New` valida: si `cfg.IDs == nil` retorna error (mensaje verbatim: `fmt.Err("user:", "Config.IDs",
  "is required")`). Fail-fast: sin generador no hay auth — nunca un default silencioso.
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
funciones no es método de `Module` (helper suelto), pásale `ids model.IDGenerator` como parámetro
desde el método que la llama — no un global.

### 3.4 `authority/tools.go` → `authority/ops.go` — transporte neutral

Borra `tools.go` completo y crea `ops.go`:

```go
package authority

import (
	"github.com/tinywasm/model"
	"github.com/tinywasm/router"
	"github.com/tinywasm/user"
)

// OpMe returns the authenticated caller's profile.
const OpMe = "me"

var _ router.OpModule = (*Module)(nil)

func (m *Module) MountOps(reg router.OpRegistry) {
	reg.Op(OpMe, m.opMe).Authenticated()
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

- `.Authenticated()` preserva la semántica actual (`model.AccessAuthenticated`: cualquier identidad,
  sin chequeo de recurso) — **no** uses `.Requires("user", model.Read)`: eso denegaría a todo usuario
  no-admin leer su propio perfil.
- `permissionsOf` se conserva tal cual (muévelo a `ops.go`).
- La identidad se lee de `ctx.UserID()` (canal tipado del router) — ya no existe `mcp.CtxKeyUserID`.
- `user.ProfileDTO` debe implementar `model.Encodable` — verifica; si le falta `EncodeFields`,
  añádelo a mano en la raíz (mismas restricciones WASM de §3.1).

### 3.5 `authority/mount.go` — codec vía `Context`

`json.Decode(string(ctx.Body()), data)` → `ctx.Decode(data)` (todas las ocurrencias del archivo).
Quita el import `"github.com/tinywasm/json"` de `mount.go`. El resto del archivo (rutas, cookies,
redirects, `Write`/`WriteStatus`) no cambia.

### 3.6 `go.mod`

- **Quita**: `github.com/tinywasm/mcp`, `github.com/tinywasm/unixid`, `github.com/tinywasm/context`
  (muere con `tools.go`).
- **Añade**: `github.com/tinywasm/events v0.0.2`.
- **Bump**: `github.com/tinywasm/router` → `v0.1.15` (necesario por `Context.Decode/Encode` y
  `OpRegistry`).
- `github.com/tinywasm/json` y `github.com/tinywasm/fetch` **se quedan** (solo `oauth.go`, §1).
- `go mod tidy`.

### 3.7 Vista — decisión explícita, no una omisión

Por el principio *"views belong to the consumer — libraries render no pages"*: la página de login,
branding y layout los compone la app consumidora con `tinywasm/form` sobre los modelos tipados que
este módulo **ya** exporta (`user.LoginData` y DTOs con `Kind` de `form/input` en `models.go`). Ese
**es** el contrato de vista de `user` en esta ola — no añadas ningún `NewView`/HTML/CSS aquí.

Un presenter de administración (CRUD de usuarios/roles vía `view.New(caller, …)`, como el de
`item_catalog`) exigiría primero exponer las ops CRUD administrativas (`authority/crud.go` hoy son
`RBACObject` server-side, no ops montables) — es un plan futuro separado, **fuera de este dispatch**.

## 4. Criterios de aceptación

- `grep -rn "tinywasm/unixid" . --include="*.go"` **vacío** (tests incluidos: usa un stub
  `func literal` que satisface `model.IDGenerator`).
- `grep -rn "tinywasm/mcp\|tinywasm/context" . --include="*.go"` **vacío**.
- `grep -rn "tinywasm/json" . --include="*.go"` → **solo** `authority/oauth.go`.
- `grep -rn "OnSecurityEvent" . --include="*.go"` **vacío**.
- `grep -rn "Tools() \[\]mcp.Tool" .` **vacío**; `var _ router.OpModule = (*Module)(nil)` compila.
- `var _ router.APIModule = (*Module)(nil)` **sigue** compilando (mount.go intacto).
- El paquete raíz `user` sigue importando únicamente `model`/`orm`/`form/input`/`fmt`/`events`
  (verificable: `go list -deps` o revisión de imports) — compilable por TinyGo.
- `go build ./...` y tests verdes; tests de eventos usan `events/mock.Broker` y verifican que un
  login fallido publica en `user.TopicSecurity`.

## 5. Impacto en consumidores (documentar en CHANGELOG del PR, no ejecutar aquí)

`mjosefa-cms` (y cualquier app): (a) pasa `IDs`/`Events` en `user.Config` — los mismos que ya
inyecta a sus módulos de dominio; (b) reemplaza `OnSecurityEvent: fn` por
`broker.Subscribe(user.TopicSecurity, fn)`; (c) deja de añadir `auth` como `mcp.ToolProvider`
aparte — ahora entra por la misma cosecha `mcp.HarvestOps` que los demás `OpModule`.

## 6. Etapas

| # | Etapa | Archivos | Criterio |
|---|---|---|---|
| 1 | Costuras en Config + topic + Encodable | `user.go` | `IDs`/`Events` en Config; sin `OnSecurityEvent`; `TopicSecurity`; `SecurityEvent` Encodable |
| 2 | Módulo guarda costuras | `authority/module.go` | `New` falla con `IDs` nil; `notify` publica |
| 3 | IDs inyectados | `authority/{identities,lan,oauth,sessions,users}.go` | `grep unixid` vacío |
| 4 | Ops neutrales | `authority/tools.go` → `ops.go` | `OpModule` compila; sin mcp/context/json |
| 5 | Codec en transporte | `authority/mount.go` | `ctx.Decode`; json solo en oauth.go |
| 6 | Deps | `go.mod`, `go.sum` | −mcp −unixid −context +events; router v0.1.15 |
| 7 | Verde | tests | mock.Broker + stub IDGenerator; `go build ./...` |
