---
message: "feat: emit EventRateLimited — the 429 gate was invisible to the security-event stream"
---

> Este plan se despacha vía el flujo CodeJob. Ver skill: agents-workflow.
>
> **COMPUERTA: ejecutar después de [PLAN_ROUTER_PUBLIC_ROUTES.md](PLAN_ROUTER_PUBLIC_ROUTES.md).**
> El test de este plan vive en `tests/owasp_test.go`, que hoy está en rojo por los 403
> de rutas sin anotar; sobre una suite roja no se puede fijar nada.

# PLAN — `user`: el rechazo por rate limit emite su evento de seguridad

Autocontenido, en español.

## El problema: la única defensa anti-fuerza-bruta no deja rastro

El endurecimiento OWASP de este módulo dejó el hook y el corte:

```go
// user.go
RateLimit func(remoteAddr string) error   // el consumidor inyecta su limitador (KV, DO, WAF)

// server/mount.go — POST /login, antes de bcrypt
if m.config.RateLimit != nil {
    if err := m.config.RateLimit(ip); err != nil {
        ctx.WriteStatus(429)
        ctx.Write([]byte(err.Error()))
        return
    }
}
```

Pero el evento que ese diseño especificaba **nunca se implementó**: `EventRateLimited`
no existe en el enum `SecurityEventType` (`user.go`) ni se emite en el handler. El
resto de fallos de seguridad de este módulo señalizan (`EventAccessDenied`,
`EventJWTTampered`, `EventNonActiveAccess`…); el corte por fuerza bruta — OWASP A09,
*Security Logging and Monitoring Failures* — es el único mudo. Un consumidor que
monitoriza por `Config.OnSecurityEvent` no puede ver el ataque que su propio limitador
está parando.

## Cambios

### 1. `user.go` — el valor nuevo del enum, AL FINAL

Añade `EventRateLimited` como **último** valor del bloque `iota` de
`SecurityEventType`, con el mismo estilo de comentario que sus vecinos:

```go
EventPermissionCorrupt // HasPermission: permissions.action is not a CRUD string
EventRateLimited       // POST /login: Config.RateLimit rejected the attempt before bcrypt
```

**Nunca en medio.** Los valores existentes se persisten y comparan por número: insertar
uno desplaza a todos los siguientes y corrompe silenciosamente cualquier dato ya
guardado.

### 2. `server/mount.go` — emitir antes del 429

En el bloque de `RateLimit` del handler de `POST user.PathLogin`, antes de escribir el
429 (calca el patrón de `m.notify` que ya usa el 401 del mismo handler, con
`Timestamp` si los demás lo llevan):

```go
m.notify(user.SecurityEvent{
    Type:   user.EventRateLimited,
    IP:     ip,
    UserID: data.Email, // el email intentado, igual que en EventAccessDenied
})
```

Nota: `ip` ya está extraída en el handler con `extractClientIP(ctx, m.config.TrustProxy)`
— reúsala; no vuelvas a llamar a la extracción.

### 3. `tests/owasp_test.go` — fijar la emisión

El subtest de rate limit ya prueba el 429 con credenciales válidas (el gate dispara
antes que bcrypt). Extiéndelo: registra `Config.OnSecurityEvent` como ya hace el
subtest A09 de eventos, y afirma que `EventRateLimited` está **presente** en la
colección tras el 429 — presente, no único: otros eventos legítimos pueden convivir.
Y el caso contrario: con hook `nil` o hook que devuelve `nil`, ese evento **no**
aparece.

## Anti-footguns

- **NO cambies la firma del hook.** `RateLimit func(remoteAddr string) error` es el
  contrato entregado (una versión anterior del diseño llevaba también el email;
  se descartó — no lo "restaures").
- **NO toques las demás emisiones** ni su orden en el enum.
- **NO conviertas el error del hook en un error genérico**: el body del 429 sigue
  siendo `err.Error()` del hook — el consumidor decide su mensaje (p. ej. Retry-After
  en prosa). El anti-enumeración aplica al 401, no al 429.
- Nunca llames `gopush` ni `codejob`.

## Criterios de aceptación

La única herramienta de pruebas es `gotest` — nunca `go test` a pelo. Si no está en el
sandbox: `go install github.com/tinywasm/devflow/cmd/gotest@latest`.

1. `grep -n "EventRateLimited" user.go server/mount.go tests/owasp_test.go` → 1 hit en
   cada archivo como mínimo, y en `user.go` es el **último** valor del bloque `iota`.
2. `gotest` en verde desde la raíz (incluye la suite de `tests/` y el subtest nuevo).
3. Este plan no añade imports: la verificación TinyGo no cambia. Si `tinygo` no existe
   en tu sandbox, no intentes instalarlo.

## Ciclo de vida de este archivo

No borres ni renombres este archivo: el flujo CodeJob lo gestiona.
