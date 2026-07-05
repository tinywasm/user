# Plan — `user`: ser la autoridad de identidad/RBAC que consumen `httpd` y `mcp`

> Autocontenido, en español. Rige el **arnés de construcción** (reglas en el
> `AGENTS.md` de la raíz de esta librería): nada de prosa que el compilador no
> imponga; una sola forma de hacer cada cosa.
>
> Hermano de [`ROUTER_REFACTOR_PLAN.md`](ROUTER_REFACTOR_PLAN.md) (que ya migra el
> borde HTTP a `router.Middleware`). Este plan cubre la **cara de permiso** que
> consumen [`mcp`](../../mcp/docs/PLAN.md) y `httpd`, y limpia una afirmación falsa.
> Depende de [`router/docs/PLAN.md`](../../router/docs/PLAN.md) (`Context.UserID()`).

---

## El problema (tres huecos)

1. **Comentario que miente.** [`user.go`](../user.go) declara que el módulo
   *"Structurally implements mcp.Authorizer via InjectIdentity + CanExecute"*, pero:
   - `user` **no importa `mcp`** (ninguna verificación del compilador lo respalda),
   - las firmas **no coinciden** con `mcp.Authorizer`
     (`Authorize(token)`+`Can(...byte)`),
   - `InjectIdentity`/`CanExecute` usan `*http.Request`/`context.Context` (stdlib), no
     el `router.Context` isomórfico.
   Es prosa que el arnés prohíbe: si algo "implementa" un contrato, que lo imponga el
   compilador (`var _ T = x`), no un comentario.

2. **Acoplamiento a `net/http`.** La identidad viaja por `*http.Request` — rompe la
   promesa isomórfica y obliga a los consumidores a estar en nativo. (La migración del
   borde a `router.Middleware` la cubre el plan hermano; aquí se **completa** la cara
   de permiso.)

3. **`action byte` vs la fuente de verdad `string`.**
   [`rbac.go`](../server/rbac.go) expone `HasPermission(userID, resource string,
   action byte)` y luego re-castea con `string(action)`. La fuente de verdad es
   `user.Permission.Action string`; el `byte` es una costura innecesaria (decisión del
   Plan Maestro: `HasPermission` migra `byte → string`).

---

## La corrección — `user` es la única autoridad, expuesta como tipos de `router`

`user/server.Module` publica **dos productos tipados**, reutilizando los tipos ya
declarados en `router` (no inventa interfaces):

```go
// QUIÉN: valida credencial (Bearer/cookie según AuthMode) y deja la identidad
// tipada en el Context para el resto de la cadena.
func (m *Module) Authenticate() router.Middleware // func(HandlerFunc) HandlerFunc
//   éxito → ctx.SetUserID(userID) ; fallo → deja anónimo (UserID()=="") y sigue.

// PUEDE: decisión de permiso RBAC. Es, por firma, un router.Authorize.
func (m *Module) Can(userID, resource, action string) bool
```

- **`Can` ES un `router.Authorize`.** Se fija con `var _ router.Authorize =
  m.Can` — el compilador garantiza el contrato que `httpd` (RBAC de ruta) y `mcp`
  (RBAC por-tool) consumen. Adiós al comentario falso.
- **`Authenticate` usa `ctx.SetUserID`** (del plan de `router`) para propagar identidad
  — un solo canal tipado, sin clave mágica ni `*http.Request`.
- **`HasPermission` migra a `action string`** (quita el `string(action)` interno);
  `Can` la envuelve. Un único carril de tipos para el permiso.

> `user` no importa `mcp`: expone tipos de `router`, y `mcp`/`httpd` los consumen. El
> acoplamiento es por contrato, no por dependencia directa (SRP).

---

## Perfil para el front — tool `me` (`user` es el dueño del dato)

El `router.Context` lleva **solo el `userID`** (identidad para autorizar, server-side).
El front necesita **datos de perfil** (nombre, email, avatar, roles, locale) para pintar
el header y hacer *gating* de UI — y eso es **dato de dominio de `user`**, no del
contrato de transporte. Por eso `user/server` expone una tool que devuelve el perfil del
usuario autenticado:

```go
// mcp.ToolProvider: una tool "me" que lee ctx.UserID() y devuelve el perfil propio.
//   Name: "me"  Resource: "profile"  Action: 'r'  Public: false (anónimo denegado)
//   Devuelve un ProfileDTO: {ID, Name, Email, Avatar, Roles, Locale} — subconjunto
//   seguro de user.User (nunca hash de password ni datos sensibles).
func (m *Module) Tools() []mcp.Tool
```

- **Auto-lectura:** la tool ignora cualquier `userID` de los argumentos y usa **solo**
  `ctx.UserID()` — un usuario solo lee **su** perfil. Anónimo (`""`) ⇒ denegado.
- **Los roles/permisos que devuelve son para UX** (mostrar/ocultar botones). El servidor
  **siempre** revalida con `Can` en cada tool — el gating de UI no es control de acceso
  (encaja con "cerrado por defecto"; el front es manipulable).
- **Una sola fuente de verdad:** el perfil vive en `user` (DB + tool `me`), no duplicado
  en el `Context` ni cacheado en el servidor por petición.

**Test:** `me` con `ctx.UserID()=="u1"` → devuelve el `ProfileDTO` de `u1` (sin campos
sensibles); anónimo (`UserID()==""`) → denegado.

---

## Cambios

| Archivo | Cambio |
|---|---|
| `user.go` | Borrar el comentario "implements mcp.Authorizer". |
| `server/middleware.go` | `InjectIdentity`/`CanExecute` (stdlib) → `Authenticate() router.Middleware` que hace `ctx.SetUserID`. Quitar firmas con `*http.Request` de la superficie (coordinar con `ROUTER_REFACTOR_PLAN.md`). |
| `server/rbac.go` | `HasPermission(... action byte)` → `... action string`; quitar el cast. |
| `server/*.go` | Añadir `func (m *Module) Can(userID, resource, action string) bool` + `var _ router.Authorize = (&Module{}).Can`. |
| `server/*.go` | Añadir `Tools() []mcp.Tool` con la tool `me` (`profile:r`, `Public:false`) que lee `ctx.UserID()` y devuelve un `ProfileDTO` seguro. |

---

## Estrategia de pruebas y criterios de aceptación

- `var _ router.Authorize = m.Can` y `var _ router.Middleware = m.Authenticate()`
  compilan — el contrato queda fijado por tipos, no por prosa.
- Middleware por contrato: un `router.Context` de mentira con credencial válida →
  `ctx.UserID()` queda seteado tras `Authenticate`; sin credencial → `UserID()==""` y el
  RBAC posterior niega.
- `HasPermission`/`Can` deciden con `action string`; ningún cast `string(action)` queda
  en el código.
- Ninguna firma exportada de `user/server` nombra `mcp.*` ni (en la cara de permiso)
  `http.*`.

---

## Endurecimiento de seguridad (cerrado por defecto) — cada punto con test

`user` es la autoridad de identidad/permiso; el default de sus respuestas debe ser
**negar**, nunca conceder por omisión.

- **Anónimo = cero permisos.** `Can("", resource, action)` devuelve `false` siempre
  (salvo recurso público, que se decide fuera de `Can`). Ninguna rama concede a la
  identidad vacía.
  **Test:** `Can("", "cualquier", "r")` → `false`, para todo recurso/acción.
- **Fail-fast de configuración.** `userserver.New` **rechaza** un `AuthMode` que exige
  `JWTSecret` (`AuthModeJWT`/`AuthModeBearer`) cuando está vacío, en vez de degradar en
  silencio a "todos anónimos". Credencial malformada ⇒ diagnóstico ruidoso.
  **Test:** `New(db, Config{AuthMode: AuthModeBearer, JWTSecret: nil})` → error; con
  secreto presente → ok.
- **RBAC vacío = denegar (nunca "si no hay permisos, permitir todo").** Una DB sin roles/
  permisos hace que `HasPermission`/`Can` devuelvan `false`. El sembrado del rol admin es
  un paso **explícito** (migración/seed), no una regla implícita de apertura.
  **Test:** contra una DB sin permisos, `Can(userID, "x", "r")` → `false`; tras sembrar
  el permiso explícitamente → `true`.

---

## Relación con el ecosistema

- [`router`](../../router/docs/PLAN.md): aporta `Context.UserID()`/`SetUserID` y el tipo
  `Authorize` que `Can` satisface.
- [`mcp`](../../mcp/docs/PLAN.md): recibe `m.Can` como `Config.Authorize`; lee la
  identidad que `Authenticate` dejó en el `Context`.
- El consumidor de referencia `veltylabs/mjosefa-cms` inyecta `Authenticate()` y `Can`
  vía `app.Deps`.
