---
message: "refactor!: rename user/server → user/authority — the package names its trust role, not a venue"
---

> Este plan se despacha vía el flujo CodeJob. Ver skill: agents-workflow.
>
> **COMPUERTA: ejecutar SOLO después de que [PLAN_ROUTER_PUBLIC_ROUTES.md](PLAN_ROUTER_PUBLIC_ROUTES.md),
> [PLAN_JWT_FROMBEARER.md](PLAN_JWT_FROMBEARER.md) y
> [PLAN_EVENT_RATELIMITED.md](PLAN_EVENT_RATELIMITED.md) estén completados** — los tres
> editan archivos de `server/` y sus instrucciones nombran rutas que este plan mueve.

# PLAN — `user`: el subpaquete `server` pasa a llamarse `authority`

Autocontenido, en español.

## Por qué (decisión ya tomada — no la reabras)

El paquete `server/` es la mitad **con autoridad** del dominio: custodia la base de
datos y el secreto JWT, autentica (password, LAN, OAuth), emite tokens, gestiona
sesiones y autoriza (RBAC). Tres defectos del nombre actual:

1. **"server" nombra el lugar, no el rol** — y el lugar ya es falso: este paquete
   compila con TinyGo y corre dentro de un Worker de Cloudflare (edge). Lo que no
   cambia según dónde corra es su rol: la *autoridad* sobre las identidades.
2. **La discordancia directorio/paquete lo confiesa**: el directorio es `server/` pero
   el paquete tuvo que llamarse `userserver`, porque `server` como identificador
   colisiona con el repo `github.com/tinywasm/server` del propio ecosistema. Cuando el
   path obliga a rebautizar el identificador, el que está mal es el path. `userserver`
   además tartamudea con su padre (`user/…/userserver`).
3. **No discrimina**: en un ecosistema lleno de cosas que sirven HTTP, "server" no dice
   qué hace este paquete que nadie más hace.

El nombre nuevo es **`authority`**: término estándar de seguridad (certificate
authority, token authority), agnóstico al despliegue, path e identificador coinciden,
y cero colisiones en el ecosistema.

## Cambios — mecánicos, sin decisiones de diseño

### 1. Mover el directorio

```bash
git mv server authority
```

### 2. Renombrar el paquete en TODOS los archivos movidos

En cada `.go` de `authority/` (17 archivos: `api_token.go`, `auth.go`, `bootstrap.go`,
`cache_users.go`, `crud.go`, `export.go`, `identities.go`, `lan.go`, `middleware.go`,
`migrate.go`, `module.go`, `mount.go`, `oauth.go`, `rbac.go`, `sessions.go`,
`tools.go`, `users.go`):

```go
package userserver   →   package authority
```

### 3. Actualizar los consumidores internos

No hay consumidores externos (verificado con grep sobre todo el ecosistema el
2026-07-14); los únicos importadores son los tests de este repo (`tests/*.go`, ~12
archivos). En cada uno:

- import: `github.com/tinywasm/user/server` → `github.com/tinywasm/user/authority`
- selector: `userserver.` → `authority.` (p. ej. `userserver.New(` → `authority.New(`,
  `userserver.Seed{` → `authority.Seed{`)

Identifica los archivos por su import, no de memoria:
`grep -rln "tinywasm/user/server" tests/`.

### 4. Documentación

- `README.md`: el ejemplo de import y toda mención `user/server`/`userserver`.
- `docs/ARCHITECTURE.md`: las menciones en prosa Y el diagrama mermaid (nodo
  `SERVER["tinywasm/user/server\n(auth backend)"]` → `AUTHORITY["tinywasm/user/authority\n(auth authority)"]`,
  y la arista `userserver.New(...)` → `authority.New(...)`). Aprovecha para corregir
  "Backend logic" por "Authority logic (native or edge)" donde describa al paquete.
- Cualquier otra mención: `grep -rn "userserver\|user/server" --include="*.md" .` y
  actualiza lo que sea de este repo.

## Anti-footguns

- **NO toques el paquete raíz `user`** (tipos agnósticos): solo se renombra el
  subpaquete.
- **NO toques el repo `github.com/tinywasm/server`**: es otro proyecto; la colisión de
  nombres con él es una de las razones de este rename, no un objetivo de cambio.
- **NO cambies ninguna firma, tipo ni comportamiento.** Este plan es un rename puro:
  si te ves editando lógica, párate. Comentarios que digan "server" refiriéndose a un
  servidor HTTP genérico se quedan; solo cambian los que nombren al *paquete*.
- **Es un cambio breaking a propósito** (import path nuevo); el `!` del mensaje de
  commit ya lo declara. No añadas un paquete `server/` de compatibilidad que reexporte:
  mantener dos paths es exactamente la deuda que este plan cierra.
- Nunca llames `gopush` ni `codejob`: herramientas locales, fuera del agente.

## Criterios de aceptación

La única herramienta de pruebas es `gotest` — nunca `go test` a pelo. Si no está en el
sandbox: `go install github.com/tinywasm/devflow/cmd/gotest@latest`.

1. `grep -rn "userserver" .` → vacío (código y docs).
2. `grep -rn "tinywasm/user/server" .` → vacío.
3. `ls server/ 2>/dev/null` → el directorio no existe; `authority/` sí.
4. `gotest` en verde desde la raíz (incluye la suite de `tests/`).
5. TinyGo sigue compilando el paquete: `gotest -tinygo` en verde. **Si `tinygo` no
   existe en tu sandbox, NO intentes instalarlo**: declara la verificación TinyGo como
   PENDIENTE en el resumen final — nunca la marques cumplida con `gotest` a secas ni
   con `go build`. (Verificación manual alternativa para el desarrollador local: un
   `main` desechable que importe `github.com/tinywasm/user/authority`, compilado con
   `tinygo build -target=wasm -o /dev/null .` — un `tinygo build ./authority/` a secas
   NO corre sobre una librería.)

## Ciclo de vida de este archivo

No borres ni renombres este archivo: el flujo CodeJob lo gestiona.
