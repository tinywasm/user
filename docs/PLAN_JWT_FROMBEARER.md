---
message: "refactor: delete the hand-rolled Bearer parsing — jwt.FromBearer owns the format"
---

> Este plan se despacha vía el flujo CodeJob. Ver skill: agents-workflow.
>
> **COMPUERTA: no despachar hasta que `github.com/tinywasm/jwt` publique la release que
> incluye `FromBearer` (plan de jwt, tag `v0.1.0`).** Antes de eso este módulo no
> compila con el cambio. Verifica: `go list -m -versions github.com/tinywasm/jwt`.

# PLAN — `user`: borrar el parseo Bearer duplicado y llamar a `jwt.FromBearer`

Autocontenido, en español.

## El problema: el parseo del formato vive en el consumidor

`server/middleware.go` lleva a mano la extracción del token del header `Authorization`:

```go
const prefix = "Bearer "
auth := ctx.GetHeader("Authorization")
if !fmt.HasPrefix(auth, prefix) {
    return nil, user.ErrSessionExpired
}
return m.validateJWT(auth[len(prefix):])
```

Ese parseo es del **formato JWT/RFC 6750**, no de esta app, y ya existe aguas abajo:

```go
// github.com/tinywasm/jwt (≥ v0.1.0)
// FromBearer extracts the token from an Authorization header value.
// A missing or non-Bearer header yields ok == false; the token is never guessed.
func FromBearer(authorizationHeader string) (token string, ok bool)
```

La copia local además es **sensible a mayúsculas**: rechaza `bearer x`, que es legal
según RFC 6750 (el esquema es case-insensitive). `jwt.FromBearer` ya lo resuelve y está
cubierto por tests y fuzzing en su repo. El plan de jwt ordena: *"cuando esté, `user`
debe borrar su copia y llamar a esta"*. Esto es ese borrado.

## Cambios

### 1. `go.mod` — subir `github.com/tinywasm/jwt` a la release con `FromBearer`

En `go.mod` raíz y en `tests/go.mod` (hoy fijan `v0.0.3`, que NO la tiene). `go mod
tidy` en ambos.

### 2. `server/middleware.go` — el borrado

En `validateSession`, sustituye el bloque de `AuthModeBearer` por:

```go
if m.config.AuthMode == user.AuthModeBearer {
    token, ok := jwt.FromBearer(ctx.GetHeader("Authorization"))
    if !ok {
        return nil, user.ErrSessionExpired
    }
    return m.validateJWT(token)
}
```

Borra la constante local `prefix` y la llamada a `fmt.HasPrefix`. Si con eso el import
`github.com/tinywasm/fmt` queda sin usos en el archivo, elimínalo del bloque de imports
(compruébalo: `grep -n "fmt\." server/middleware.go`).

### 3. Test del cambio de comportamiento

El middleware ahora acepta `bearer <token>` y `BEARER <token>` (antes: rechazados).
Añade/ajusta el test del middleware en `tests/` cubriendo: `Bearer x` ✓, `bearer x` ✓,
`Basic x` ✗, header vacío ✗. Sigue el patrón de registro de tests que ya use la suite
de este repo — nada de `TestXxx` sueltos nuevos si la suite registra en un runner.

## Anti-footguns — NO toques esto

- `server/oauth.go` construye headers **salientes** con `"Bearer "+token.AccessToken`
  (dos sitios). Eso es *emitir* el header hacia el proveedor OAuth, no parsearlo:
  **se queda exactamente como está.**
- `server/api_token.go` menciona "Bearer" en comentarios/documentación: se queda.
- No "aproveches" para tocar `validateJWT` ni el manejo de `Outcome`: la separación
  error-del-caller / veredicto-del-token es una invariante de jwt (ver su `AGENTS.md`).

## Criterios de aceptación

La única herramienta de pruebas es `gotest` — nunca `go test` a pelo. Si no está en el
sandbox: `go install github.com/tinywasm/devflow/cmd/gotest@latest`.

1. `grep -n "HasPrefix" server/middleware.go` → vacío.
2. `grep -rn '"Bearer "' server/` → solo los usos **salientes** de `oauth.go` (y
   comentarios); ninguno en `middleware.go`.
3. `gotest` en verde desde la raíz (incluye la suite de `tests/`); el caso `bearer x`
   (minúsculas) autentica.
4. Este plan cambia imports (`jwt` sube de versión): la compatibilidad TinyGo debe
   re-verificarse con `gotest -tinygo`. **Si `tinygo` no existe en tu sandbox, NO
   intentes instalarlo**: declara la verificación TinyGo como PENDIENTE en el resumen
   final — nunca la marques cumplida con `gotest` a secas.
5. Nunca llames `gopush` ni `codejob`.

## Ciclo de vida de este archivo

No borres ni renombres este archivo: el flujo CodeJob lo gestiona.
