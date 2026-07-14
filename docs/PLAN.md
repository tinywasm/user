---
message: "fix!: purge golang.org/x/oauth2 so the module actually compiles for the edge (TinyGo)"
---

> Este plan se despacha vía el flujo CodeJob. Ver skill: agents-workflow.

# PLAN — cola de ejecución de `user`

> Si te han dicho *"ejecuta el plan descrito en docs/PLAN.md"*, ejecuta el **primer plan
> pendiente** de la tabla. Es autocontenido.

| Orden | Plan | Estado | Asunto |
|-------|------|--------|--------|
| 1 | [PLAN_TINYGO_OAUTH.md](PLAN_TINYGO_OAUTH.md) | ✅ **COMPLETADO** | Purgar `golang.org/x/oauth2`: el intercambio de código pasa a `tinywasm/fetch`, y `Token`/`Config` a tipos locales. Sin esto el módulo **no entra en un Worker**. |

## Estado actual — "edge-ready" no lo era

El commit `b67a8cc` ("edge-ready auth") verificó el borde con **el compilador equivocado**.
`go build` y `GOOS=js GOARCH=wasm go build` dan verde, pero **el borde de Cloudflare se
compila con TinyGo**, y ahí el módulo no compila:

```
# net/http
tinygo/src/net/http/roundtrip_js.go:73:12: t.roundTrip undefined
```

`server/oauth.go` importa `golang.org/x/oauth2`, que arrastra `net/http`, que TinyGo no
soporta. Todo el módulo —login, sesiones, RBAC— queda fuera del borde **por una sola llamada
HTTP** (`oauth2.Config.Exchange`).

Lo destapó `goflare-demo` al intentar montar autenticación real en su Worker.

**La regla que deja este episodio: en este repo TinyGo es el compilador que decide.** Un
criterio de aceptación que diga "compila a wasm" y lo verifique con `go build` no verifica
nada. Ver `tinywasm/docs/DEMO_FOUR_APIS_MASTER_PLAN.md`.
