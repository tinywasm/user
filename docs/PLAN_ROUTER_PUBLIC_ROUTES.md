---
message: "fix: mark the pre-identity auth routes Public — under a conformant router the whole login flow answers 403"
---

> Este plan se despacha vía el flujo CodeJob. Ver skill: agents-workflow.

# PLAN — `user`: las rutas de autenticación nunca se marcaron públicas

Autocontenido, en español.

## El problema: el módulo de login es inaccesible en un router honesto

`tinywasm/router` es **cerrado por defecto**: una ruta registrada sin anotación
(`.Public()` o `.Requires(recurso, acción)`) **deniega con 403** a quien no tiene
identidad. `server/mount.go` registra `POST /login`, `POST /logout` y las rutas OAuth a
pelo, sin anotación ninguna:

```go
r.Post(user.PathLogin, func(ctx router.Context) { ... })   // ← sin .Public()
```

Un usuario **sin sesión** — exactamente el usuario de `/login` — recibe 403 antes de
llegar al handler. **El módulo de autenticación entero es inalcanzable para quien
necesita autenticarse.**

Pasó desapercibido porque el mock de `router` ≤ v0.1.9 era una grabadora: `Invoke`
llamaba al handler directamente, saltándose la puerta. El mock de **v0.1.10** aplica el
mismo contrato que las implementaciones desplegadas (pasa `router/conformance`), y con
él la suite lo destapa:

```
--- FAIL: TestProductionWiring/MountAPI
    production_wiring_test.go:142: POST /login (JSON) status: 403
    production_wiring_test.go:172: POST /logout status: 403
```

## Cambios

### 1. `go.mod` y `tests/go.mod` — router v0.1.10

Sube `github.com/tinywasm/router` a `v0.1.10` en **ambos** módulos (`go.mod` raíz y
`tests/go.mod`; puede que `tests/go.mod` ya lo tenga). `go mod tidy` en ambos.

### 2. `server/mount.go` — anotar TODAS las rutas del flujo pre-identidad

`r.Post`/`r.Get` devuelven `router.Route`; encadena `.Public()` en el registro:

- `r.Post(user.PathLogin, ...).Public()` — login es, por definición, anterior a la identidad.
- `r.Post(user.PathLogout, ...).Public()` — logout debe funcionar también con sesión ya
  caducada o cookie corrupta: su trabajo es precisamente limpiar; gatearlo deja al
  usuario atrapado con una cookie inválida.
- `r.Get("/oauth/"+providerName, ...).Public()` — inicia el flujo, no hay identidad aún.
- `r.Get("/oauth/callback/"+providerName, ...).Public()` — el proveedor redirige aquí a
  un navegador que todavía no tiene sesión nuestra.

**No anotes nada más.** En particular, NO añadas `.Public()` a ninguna ruta que otro
archivo registre con `.Requires(...)`: cerrado por defecto es el contrato, y estas
cuatro son las únicas rutas legítimamente pre-identidad de este módulo. Verifica que no
haya otros registros olvidados: `grep -rn "r.Post\|r.Get\|r.Handle" server/` — todo lo
que aparezca debe quedar o bien `.Public()` (solo el flujo de auth de arriba) o bien
`.Requires(...)`, nunca a pelo.

### 3. Test que fija el contrato

Añade al test de `MountAPI` (en `tests/production_wiring_test.go`) una comprobación
sobre `r.Routes()`: cada ruta montada por este módulo debe satisfacer
`info.IsPublic() == true`. Así, si mañana alguien registra una ruta nueva a pelo en
`MountAPI`, el test la nombra en rojo en vez de esperar al 403 de producción.

## Fuera de alcance

- No toques `server/middleware.go` (la migración a `jwt.FromBearer` es
  [PLAN_JWT_FROMBEARER.md](PLAN_JWT_FROMBEARER.md), otro plan con su propia compuerta).
- No configures `Authn`/`Authorize` del mock para "hacer pasar" nada: las cuatro rutas
  deben responder **sin identidad**. Si un caso solo pasa configurando identidad, la
  anotación está mal.
- Nunca llames `gopush` ni `codejob`: herramientas locales del desarrollador, fuera del
  agente.

## Criterios de aceptación

1. `go test ./...` dentro de `tests/` en verde, con `router v0.1.10` en ambos `go.mod`.
2. `grep -rn "\.Public()" server/mount.go` → exactamente 4 resultados (login, logout,
   oauth begin, oauth callback).
3. El test de rutas públicas sobre `Routes()` existe y está en verde.
4. `gotest` en verde. (La compuerta TinyGo del módulo es asunto de
   [PLAN_TINYGO_OAUTH.md](PLAN_TINYGO_OAUTH.md); este plan no debe empeorarla ni
   arreglarla.)

## Ciclo de vida de este archivo

No borres ni renombres este archivo: el flujo CodeJob lo gestiona.
