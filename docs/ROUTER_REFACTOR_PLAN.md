# Plan — Refactor de `user` (server) al middleware de `tinywasm/router`

> El lado servidor de `user` expone su auth como middleware de `net/http`
> (`Middleware(http.Handler) http.Handler`) y funciones que reciben `*http.Request`.
> El refactor lo pasa al concepto de **middleware** del contrato isomórfico
> `github.com/tinywasm/router`. Autocontenido, en español.

---

## Reglas de Desarrollo

Las reglas del arnés viven en el **`AGENTS.md` de la raíz de esta librería** — léelo
antes de cualquier cambio. Este PLAN no las repite; describe solo el *cómo*.

Alcance de este PLAN: **solo `user/server/`** (auth/sesión). El lado UI (`ui/*`,
wasm) lo cubre el plan hermano de lifecycle; no se toca aquí.

---

## El contrato que consume (reexpresado para ser autocontenido)

```go
// package router (github.com/tinywasm/router)
type Context interface { Method() string; Path() string; Body() []byte
	GetHeader(k string) string; SetHeader(k, v string); WriteStatus(code int); Write([]byte) (int, error)
	// Valores de ámbito de petición (para que un middleware entregue datos al handler):
	SetValue(key string, v any); Value(key string) any // ver nota de dependencia
}
type HandlerFunc func(Context)

// Middleware envuelve un handler operando SOLO sobre Context.
type Middleware func(HandlerFunc) HandlerFunc
// Router: Use(m ...Middleware)
```

> **Nota de dependencia:** para que un middleware de auth sirva de algo, debe poder
> **entregar la identidad autenticada al handler siguiente**. Eso exige que el
> `Context` del contrato exponga valores de ámbito de petición (un `SetValue`/`Value`
> mínimo, o un accesor de identidad tipado). Este PLAN asume esa capacidad; si el
> contrato `router` aún no la define, es un prerrequisito a añadir en su PLAN
> (extensión *middleware*).

---

## Estado de partida

```go
func (m *Module) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { … })
}
func (m *Module) RegisterMCP(next http.Handler) http.Handler { … }
func (m *Module) InjectIdentity(ctx context.Context, r *http.Request) context.Context { … }
func (m *Module) validateSession(r *http.Request) (*user.User, error) { … }
func (m *Module) LoginLAN(rut string, r *http.Request) (user.User, error) { … }
// extractClientIP(r *http.Request, …), CompleteOAuth(…, r *http.Request, …)
```

Todo el borde HTTP está atado a `net/http`.

---

## Cambios (antes → después)

| Antes (`net/http`) | Después (`router`) |
|---|---|
| `Middleware(next http.Handler) http.Handler` | `Middleware() router.Middleware` (`func(HandlerFunc) HandlerFunc`) |
| `RegisterMCP(next http.Handler) http.Handler` | `router.Middleware` (o montaje de la ruta MCP vía `Router`) |
| `InjectIdentity(ctx, r *http.Request)` | el middleware pone la identidad en el `router.Context` (`SetValue`) para el handler siguiente |
| `validateSession(r *http.Request)` | `validateSession(ctx router.Context)` (lee cookie/header vía `ctx.GetHeader`) |
| `LoginLAN(rut, r)`, `extractClientIP(r)`, `CompleteOAuth(…, r)` | reciben `router.Context` en vez de `*http.Request` |

`user/server` deja de nombrar `http.Handler`/`http.Request`/`http.ResponseWriter` en
su superficie; la identidad viaja por el `Context` del contrato, no por
`context.Context` + `*http.Request`.

---

## Pasos de implementación

1. Añadir dependencia `github.com/tinywasm/router` en `go.mod`.
2. Convertir `Middleware`/`RegisterMCP` en `router.Middleware`.
3. Migrar las funciones que reciben `*http.Request` a `router.Context`.
4. Sustituir la inyección de identidad por `Context` (valor de ámbito de petición).

---

## Estrategia de pruebas y criterios de aceptación

- **Sin `net/http` en la superficie pública de `user/server`:** ninguna firma
  exportada nombra `http.Handler`/`http.Request`. Verificable por búsqueda.
- **Middleware por contrato:** un `router.Context` de mentira con/ sin sesión válida
  demuestra que el middleware corta (401) o deja pasar y **entrega la identidad** al
  handler siguiente. `var _ router.Middleware = m.Middleware()` fija el contrato.
- **Identidad propagada:** el handler siguiente lee la identidad desde el `Context`,
  no desde un `*http.Request`.

---

## Relación con el otro plan de esta librería

`docs/FORM_LIFECYCLE_PLAN.md` toca **solo `ui/*` (wasm)**; este toca **solo
`user/server/` (auth)**. No se solapan. El `docs/PLAN.md` orquestador fija el orden.
