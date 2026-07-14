# PLAN (EJECUTADO 2026-07-14, LOCAL) — recompilado contra `model` v0.0.14 / `orm` v0.9.28

> Ejecutado directamente por el mantenedor (LOCAL, sin codejob). Fase D (propagación) de la
> ola CRUD Harness: https://github.com/tinywasm/app/blob/main/docs/CRUD_HARNESS_MASTER_PLAN.md

## El problema

`model` v0.0.14 amplía `model.Model` a `Fielder + ModuleNaming + Encodable + Decodable`. El
paquete raíz de este repo ya compilaba limpio contra el contrato ampliado — sus modelos
(`User`, `Session`, `Identity`, `Role`, `UserRole`) ya están generados por `ormc` y por tanto
ya implementaban las cinco capacidades. El único bloqueo era transitivo: el submódulo
`tests/` depende de `tinywasm/sqlite`, que sí tenía fixtures escritos a mano sin
`Encodable`/`Decodable` (arreglado en `sqlite` v0.2.6).

## Cambios ejecutados

| Archivo | Cambio |
|---|---|
| `go.mod` | `tinywasm/model` → v0.0.14, `tinywasm/orm` → v0.9.28 |
| `tests/go.mod` | ídem + `tinywasm/sqlite` → v0.2.6 |

Ningún cambio de código: este repo no tenía implementadores propios de `model.Model` sin las
capacidades nuevas.

`gotest ./...` verde (raíz + submódulo `tests/`, incluye race). Publicado con gopush como
v0.0.35.
