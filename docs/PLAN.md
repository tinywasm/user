---
message: "fix: mark the pre-identity auth routes Public — under a conformant router the whole login flow answers 403"
---

> Este plan se despacha vía el flujo CodeJob. Ver skill: agents-workflow.

# PLAN — cola de ejecución de `user`

> Si te han dicho *"ejecuta el plan descrito en docs/PLAN.md"*, ejecuta el **primer plan
> pendiente** de la tabla. Es autocontenido.

| Orden | Plan | Estado | Asunto |
|-------|------|--------|--------|
| 1 | [PLAN_ROUTER_PUBLIC_ROUTES.md](PLAN_ROUTER_PUBLIC_ROUTES.md) | ☐ **PENDIENTE** — la suite está en rojo por esto | `MountAPI` nunca marcó `.Public()` sus rutas: en un router conforme (cerrado por defecto), `/login`, `/logout` y OAuth responden **403 a quien aún no tiene identidad**. Con `router v0.1.10` el mock destapa 3 suites en rojo. |
| 2 | [PLAN_JWT_FROMBEARER.md](PLAN_JWT_FROMBEARER.md) | ☐ **PENDIENTE** — desbloqueado: `jwt v0.1.0` ya está publicado | Borrar el parseo manual de `Bearer ` en `server/middleware.go` y llamar a `jwt.FromBearer` (case-insensitive, RFC 6750). Ordenado por el plan de `tinywasm/jwt`. |
| 3 | [PLAN_EVENT_RATELIMITED.md](PLAN_EVENT_RATELIMITED.md) | ⛔ **BLOQUEADO** — hasta completar 1 (su test vive en `owasp_test.go`, hoy rojo) | Resto del endurecimiento OWASP (A09): el corte por `Config.RateLimit` responde 429 pero **no emite evento** — `EventRateLimited` nunca se añadió al enum. La única defensa anti-fuerza-bruta es invisible para `OnSecurityEvent`. |
| 4 | [PLAN_AUTHORITY_RENAME.md](PLAN_AUTHORITY_RENAME.md) | ⛔ **BLOQUEADO** — hasta completar 1, 2 y 3 (todos editan `server/`) | Rename `user/server` → `user/authority`: el paquete nombra su rol de confianza (custodia secreto + store, autentica, emite, autoriza), no un lugar — hoy corre igual en nativo que en un Worker. Elimina el hack `userserver` y la colisión con el repo `tinywasm/server`. Breaking a propósito; sin consumidores externos (verificado 2026-07-14). |

## Planes históricos — ejecutados y borrados (2026-07-14)

Verificados contra el código por sus propios criterios de aceptación y eliminados
(los planes son efímeros): `PLAN_POLICY` (política al consumidor vía `Bootstrap(Seed)`),
`PLAN_STAGE_1..6` (Kind API, JSON-only, purga stdlib, `tests/go.mod`, OWASP, docs) y
`PLAN_TINYGO_OAUTH` (purga oauth2). Únicos restos vivos, ya recogidos arriba o
aceptados: `EventRateLimited` (→ plan 3); `sync` sigue importado en
`server/{cache_users,module,sessions}.go` — costo edge aceptado, sin equivalente en el
ecosistema (TinyGo lo soporta); el hit de "urlencoded" en `oauth.go` es el header
**saliente** del token endpoint OAuth, exigido por la spec — legítimo.
| ~~—~~ | [PLAN_TINYGO_OAUTH.md](PLAN_TINYGO_OAUTH.md) | ✅ **COMPLETADO** (commit `d243ad3`) | Purga de `golang.org/x/oauth2`. Cierre manual posterior: su criterio TinyGo no se había verificado de verdad — `api_token.go` desbordaba int32 (`365*24*3600*100`); corregido a 50 años. `tinygo build` del módulo `server/` ahora pasa (verificado vía main de prueba, ya que `tinygo build ./server/` a secas exige paquete main). |

## Cómo se prueba en este repo (aplica a TODOS los planes de la tabla)

La única herramienta de pruebas es `gotest` — nunca `go test` a pelo. Si no está en el
sandbox:

```bash
go install github.com/tinywasm/devflow/cmd/gotest@latest
gotest          # desde la raíz del repo: vet + race + tests + wasm
```

## Lección vigente

**En este repo TinyGo es el compilador que decide** (int de 32 bits incluido: una
constante de segundos puede desbordar). Un criterio de aceptación "compila a wasm"
verificado con `go build` — o con un comando que ni siquiera corre sobre una librería,
como `tinygo build ./server/` sin paquete main — no verifica nada. La verificación
honesta es `gotest -tinygo`, o un `main` desechable que importe el paquete compilado
con `tinygo build -target=wasm`.

**Si `tinygo` no existe en tu sandbox: NO intentes instalarlo.** Deja la verificación
TinyGo declarada como **PENDIENTE** en tu resumen final — jamás marques ese criterio
como cumplido con `go build`/`gotest` a secas; el desarrollador local la ejecuta antes
de cerrar el loop.
