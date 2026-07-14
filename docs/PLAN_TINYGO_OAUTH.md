---
message: "fix!: purge golang.org/x/oauth2 so the module actually compiles for the edge (TinyGo)"
---

> Este plan se despacha vía el flujo CodeJob. Ver skill: agents-workflow.
> Orquestado por `tinywasm/docs/DEMO_FOUR_APIS_MASTER_PLAN.md` — **Fase E (compuerta)**.

# PLAN — `user` compila de verdad para el borde: fuera `golang.org/x/oauth2`

Autocontenido, en español.

## El problema: "edge-ready" se verificó con el compilador equivocado

El commit `b67a8cc` ("edge-ready auth") dejó este módulo compilando a wasm. Con **el
compilador de Go**. Pero **el borde de Cloudflare se compila con TinyGo**, y ahí no compila:

```
# net/http
/home/cesar/.local/tinygo/src/net/http/roundtrip_js.go:73:12: t.roundTrip undefined
```

La cadena:

```
user/server/oauth.go  →  golang.org/x/oauth2  →  net/http  →  TinyGo NO lo soporta
```

**`user` no puede entrar en un Worker.** El módulo entero —login, sesiones, RBAC— queda fuera
del borde por una dependencia que solo usa **una función**.

Esto no es un detalle de empaquetado: es la diferencia entre una librería que sirve y una que
no. Y pasó desapercibido porque `go build` y `GOOS=js GOARCH=wasm go build` **dan verde los
dos** sobre código que TinyGo rechaza.

> **La regla, y es el corazón de este plan:** en este repo, **TinyGo es el compilador que
> decide**. Un criterio de aceptación que diga "compila a wasm" y lo verifique con `go build`
> no está verificando nada.

## El daño está acotado (medido, no supuesto)

- `golang.org/x/crypto/bcrypt` **compila con TinyGo**. No se toca.
- `tinywasm/fetch` **compila con TinyGo**. Es la herramienta.
- `GetUserInfo` **ya usa `tinywasm/fetch`** (`server/oauth.go:198`). Ya está bien.
- El **único** que arrastra `net/http` es `ExchangeCode`, vía `oauth2.Config.Exchange`.

O sea: sobra **una** dependencia, y se sustituye por **un POST**.

## Cambios

### 1. Tipos propios, en `user.go` — adiós a `oauth2.Config` y `oauth2.Token`

`golang.org/x/oauth2` aporta aquí dos structs y una llamada HTTP. Los structs son triviales;
la llamada es un POST. Declara los tipos en el paquete raíz:

```go
// OAuthToken is what a provider returns when it exchanges the code. It replaces
// oauth2.Token: that type dragged net/http in, and net/http does not exist under TinyGo —
// which put this whole module out of the edge for one function call.
type OAuthToken struct {
	AccessToken string
	TokenType   string
	ExpiresIn   int
}

// OAuthConfig is the provider's registration: what the app declares in the provider console.
type OAuthConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
	Scopes       []string
	AuthURL      string // provider's authorization endpoint
	TokenURL     string // provider's token endpoint
}
```

Y la interfaz `OAuthProvider` deja de nombrar `*oauth2.Token`:

```go
ExchangeCode(code string) (OAuthToken, error)      // sin context.Context: no hay net/http que cancelar
GetUserInfo(token OAuthToken) (OAuthUserInfo, error)
```

**ROMPE API** a propósito: cualquier proveedor de fuera implementaba la firma vieja.

### 2. `ExchangeCode` con `tinywasm/fetch`

El intercambio OAuth2 es un `POST` con `Content-Type: application/x-www-form-urlencoded` al
`TokenURL`, con estos campos: `code`, `client_id`, `client_secret`, `redirect_uri` y
`grant_type=authorization_code`. La respuesta es JSON con `access_token`, `token_type` y
`expires_in`.

Escríbelo **una vez** en un helper compartido por Google y Microsoft — hoy los dos
`ExchangeCode` son el mismo código con otra URL:

```go
// exchangeCode swaps the authorization code for a token. It is a plain form POST: that is
// all oauth2.Config.Exchange ever did here, and it cost us net/http — and with it, the edge.
func exchangeCode(cfg OAuthConfig, code string) (OAuthToken, error)
```

Reglas, obligatorias:

- **Sin `net/http`, sin `net/url`, sin `context`.** El cuerpo urlencoded se arma con
  `tinywasm/fmt`; el escapado de los valores es tuyo (`url.QueryEscape` es stdlib).
- **`tinywasm/fetch` es asíncrono** (callback). El resto de `user` es síncrono. Usa el mismo
  patrón que ya usa `GetUserInfo` en este archivo para esperar el resultado — cópialo, no
  inventes otro.
- **Decodifica con `tinywasm/json`**, a través del codec generado. `encoding/json` está
  prohibido y además no compila aquí.
- **Nunca loguees el `client_secret` ni el `access_token`.** Ni en un error.

### 3. `AuthCodeURL` es construcción de string

No necesita `oauth2`: es `AuthURL` + query params (`client_id`, `redirect_uri`,
`response_type=code`, `scope`, `state`). Escríbelo con `tinywasm/fmt`.

Los endpoints, hoy heredados de `google.Endpoint` y `microsoft.*`, pasan a ser constantes
tipadas en el proveedor:

```go
const (
	googleAuthURL  = "https://accounts.google.com/o/oauth2/auth"
	googleTokenURL = "https://oauth2.googleapis.com/token"
)
```

### 4. Purga

Borra de `go.mod` `golang.org/x/oauth2`. Borra el import de `context` de `server/oauth.go` y
de `user.go` **si queda huérfano** — no toques otros usos legítimos.

## Anti-footguns

- **NO purgues `golang.org/x/crypto/bcrypt`.** Compila con TinyGo, está medido, y es el
  hashing de contraseñas. La regla "nada de stdlib/dependencias pesadas" existe para lo que
  **no compila en el borde**; bcrypt sí.
- **NO cambies el flujo OAuth ni el modelo de sesiones.** Este plan cambia **de qué manera se
  hace una llamada HTTP**, nada más. Si te ves tocando `rbac.go` o `middleware.go`, párate.
- **NO "arregles" `GetUserInfo`**: ya usa `fetch` y está bien.

## Criterios de aceptación

- **El de verdad**, y no se sustituye por ningún otro:
  ```bash
  tinygo build -target=wasm -o /dev/null ./server/   # → pasa
  ```
- `gotest` pasa (incluida la suite wasm).
- La dependencia ha desaparecido del todo:
  ```bash
  grep -rn "golang.org/x/oauth2" --include=*.go --include=go.mod .   # → vacío
  grep -rn "net/http" --include=*.go . | grep -v _test               # → vacío
  ```
- El intercambio de código está escrito **una vez**, no una por proveedor:
  ```bash
  grep -rn "func exchangeCode" --include=*.go .   # → exactamente 1
  ```
- Login con Google sigue funcionando de extremo a extremo (test de integración existente).
