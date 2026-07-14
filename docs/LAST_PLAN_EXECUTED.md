# PLAN (EJECUTADO 2026-07-14, LOCAL) — recompilado contra `form` v0.2.14

> Ejecutado directamente por el mantenedor (LOCAL, sin codejob). Fase D (propagación) de la
> ola CRUD Harness: https://github.com/tinywasm/app/blob/main/docs/CRUD_HARNESS_MASTER_PLAN.md

## El problema

Tras publicar `form` v0.2.14 (fase B: `LoadValues` + `New` falla si no vincula ni un input),
el cascade automático de `gopush` reportó `user → tests ❌` al intentar bumpear la dependencia.
Reproducido a mano: el bump compila y **`gotest ./...` pasa limpio** en ambos módulos (raíz y
`tests/`). El fallo del cascade fue transitorio (paralelismo entre 34 repos actualizándose a
la vez), no un defecto real — confirmado además porque este repo **no llama a `form.New` en
ningún código de producción** (es un módulo de auth backend, sin vistas), así que no hay
exposición a la regla nueva de `New`.

## Cambios ejecutados

| Archivo | Cambio |
|---|---|
| `go.mod` | `tinywasm/form` → v0.2.14 |
| `tests/go.mod` | ídem |

Ningún cambio de código.

`gotest ./...` verde (raíz + submódulo `tests/`, incluye race). Publicado con gopush como
v0.0.36.

## Historial de esta fase D

- v0.0.35: `model` v0.0.14, `orm` v0.9.28, `sqlite` v0.2.6 — sin defecto propio, solo esperaba
  a `sqlite` (ver commit de esa versión).
- v0.0.36 (este): `form` v0.2.14 — sin defecto propio, cascade transitorio.
