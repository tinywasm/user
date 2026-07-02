# PLAN — Orquestador de `tinywasm/user`

> `docs/PLAN.md` es el **orquestador** de esta librería: coordina los planes activos,
> no contiene sus pasos. Cada plan es autocontenido en su propio archivo. Los dos
> planes tocan **partes disjuntas** de la librería (UI wasm vs server auth).

---

## Reglas de Desarrollo

Las reglas del arnés viven en el **`AGENTS.md` de la raíz de esta librería** — léelo
antes de cualquier cambio.

---

## Planes coordinados

1. **Quitar delegación de lifecycle en la UI** → `docs/FORM_LIFECYCLE_PLAN.md`
   Elimina los `OnMount` de `ui/login.go`/`profile.go`/`register.go` (los forms se
   auto-gestionan por señales) y convierte las filas IP de `lan.go` en un
   `SignalNodes`. Toca **solo `ui/*` (wasm)**. *(Pendiente: `OnMount` sigue en los
   cuatro `ui/*`.)*

2. **Refactor de auth a middleware de `tinywasm/router`** → `docs/ROUTER_REFACTOR_PLAN.md`
   Migra `user/server` (`Middleware`, `RegisterMCP`, funciones con `*http.Request`)
   del `net/http` al concepto de `router.Middleware`/`router.Context`. Toca **solo
   `user/server/`**.

---

## Orden de ejecución y por qué

Los dos planes son **independientes** (archivos y `build tags` disjuntos: `ui/*` es
`//go:build wasm`; `user/server/*` es servidor). Pueden ejecutarse en **cualquier
orden o en paralelo**; ninguno bloquea al otro.

Única dependencia externa: el plan (2) asume que el contrato `router` ofrece
*middleware* con valores de ámbito de petición — prerrequisito en el PLAN de
`github.com/tinywasm/router`.

---

## Criterio de cierre

Cuando un plan quede **ejecutado**, se elimina su archivo y se retira de esta lista.
Este orquestador desaparece cuando ambos planes estén cerrados.
