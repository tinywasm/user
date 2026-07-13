---
message: "feat!: the security policy belongs to the consumer — Bootstrap(Seed), no implicit admin, no invented resource"
---

> Este plan se despacha vía el flujo CodeJob. Ver skill: agents-workflow.
> Orquestado por `tinywasm/docs/AUTH_POLICY_MASTER_PLAN.md` — **Fase E**.
> Autocontenido: el agente que lo ejecuta no tiene contexto previo.
>
> **Es la ETAPA 7 del [`docs/PLAN.md`](PLAN.md) de este repo, no un plan aparte.**
> **Gate: la Etapa 1 (Kind migration) debe estar aplicada.** No se puede ejecutar antes: la
> Etapa 1 regenera `models_orm.go` renombrando campos (`ID` → `Id`), y este plan trabaja
> sobre esas mismas llamadas (`server/rbac.go`, `server/bootstrap.go`); además necesita
> tocar `models.go`, que es el archivo de la Etapa 1.
>
> **Antes de nada, el repo debe COMPILAR.** Hoy no lo hace: `form` está pineado en v0.2.12,
> que usa `model.Widget` (eliminado por el refactor Kind) — sube a **v0.2.13**.
>
> **COMPUERTA:** requiere `tinywasm/model` v0.0.11+, `tinywasm/router` (fase B) y
> `tinywasm/mcp` con `Access` (fase C), publicados. Sube `go.mod` primero. Si
> `mcp.AccessAuthenticated` o `model.Grant` no existen, PARA y reporta.
>
> **Break change: se publica v0.1.0 y queda obsoleta v0.0.32.**

# PLAN — `user`: la librería aporta el mecanismo; la política la declara el consumidor

**En una frase:** esta librería está decidiendo la política de seguridad de las apps que la
usan. Deja de hacerlo.

---

## El problema (léelo entero antes de tocar nada)

| | Qué es | De quién debe ser |
|---|---|---|
| **Mecanismo** | hashear, sesiones/JWT, cookies, `/login`, `/logout`, guardar y comprobar permisos, eventos de seguridad | **esta librería** |
| **Política** | qué roles existen y cómo se llaman, qué recursos hay, quién recibe qué, si hay comodín, cómo se llama el primer usuario | **el consumidor** |

Hoy la librería decide la política. Tres pruebas, en su propio código:

**1. Exporta el vocabulario de seguridad de la app** (`user.go`):

```go
RoleCodeAdmin = "admin"
ResourceAll   = "*"
ActionAll     = "*"
```

Una app cuyos roles sean `dueño`/`recepción`/`profesional` **hereda un vocabulario ajeno**.

**2. `Bootstrap` hornea una política completa que nadie puede cambiar ni rechazar**
(`server/bootstrap.go`): crea el usuario llamado `"Administrator"`, crea el rol `role_admin`,
crea el permiso `perm_all` con **comodín `*:*`** y se lo asigna.

Una app que deba operar bajo **mínimo privilegio** (sin rol total) **no tiene forma de decir
que no**. Eso no es "cerrado por defecto": el default **concede todo**. Es un fail-open, y es
el peor problema de este repo.

**3. El tool `me` se inventa un recurso** (`server/tools.go`): `Resource: "profile"`. La app
nunca declaró ese recurso, pero ahora su RBAC lo tiene.

**Además:** `*Module` **no satisface `router.APIModule`** (le falta `ModelName`), y los
consumidores lo descubren escribiendo un wrapper local — un fork de una responsabilidad de
esta librería. Ya ocurrió.

> **Aviso — no repitas este error.** En v0.0.32 se "arregló" añadiendo
> `ModelName() { return "user" }`. Eso mete un tercer string mágico y lo hace **divergir** del
> `"profile"` de sus propios tools: la UI filtra por identidad de módulo y el servidor exige
> por recurso, así que al usuario se le mostraría una sección y luego se le negarían sus
> datos, **sin un solo error**. v0.0.32 queda obsoleta. Este plan la reemplaza.

## Paso 1 — borrar la política del vocabulario

En `user.go`, **elimina**:

```go
RoleCodeAdmin = "admin"   // política: el nombre del rol lo pone la app
ResourceAll   = "*"       // duplica model.Wildcard
ActionAll     = "*"       // las acciones son un conjunto cerrado: "todas" es model.AllActions
```

El comodín de recurso es **mecanismo** y ya vive tipado en `model` (`model.Wildcard`). Para las
acciones no hay comodín: son un **conjunto cerrado CRUD** y "todas" es `model.AllActions`.
Úsalos al **comprobar** permisos; **jamás** al concederlos.

Tipa la API pública de RBAC con `model.Resource` / `model.Action` / `model.RoleCode`:
`CreateRole`, `CreatePermission`, `HasPermission`, `Can`. Reutiliza `model.Grant.Matches`
para interpretar el comodín en vez de compararlo a mano — **un solo sitio decide qué
significa `*`**.

### Cómo se GUARDA una acción (importante: no guardes el número)

`model.Action` es una máscara de bits porque **solo un tipo numérico cierra el conjunto de
verbos** — con un `type Action string`, `Requires("orders", "write")` seguiría compilando
(verbo inventado que nadie enforza). Pero un `6` en una columna es ilegible.

Por eso la columna `permissions.action` guarda **las letras de siempre**, no los bits:

```go
perm.Action = a.String()                 // Read|Update → "ru";  AllActions → "crud"
a, err := model.ParseAction(perm.Action)  // "ru" → Read|Update; letra desconocida → ERROR
```

La columna se declara en **`models.go`** (el `model.Definition` de `Permission`), no en el
`models_orm.go` generado: **edita la fuente y regenera con `ormc`**, nunca el archivo
generado. Ese `models.go` es el mismo que toca la Etapa 1 — por eso este plan va después.

Una letra desconocida **falla ruidosamente**: si `ParseAction` devolviera cero en silencio,
una fila corrupta (`"raed"`) se leería como "sin permisos" y podría re-guardarse así,
borrando el permiso real sin que nadie viera un error. **No te la tragues.**

La conversión vive **solo en la frontera de persistencia** (leer/escribir la fila). En
memoria y en las firmas, `model.Action` siempre.

`Can` es la costura del ecosistema; que el compilador garantice que encaja:

```go
var _ model.Authorizer = (*Module)(nil).Can
```

## Paso 2 — `Bootstrap` recibe la política, no la inventa

```go
// Seed es la política INICIAL de la app: la declara el consumidor, en su código.
// Esta librería solo la persiste, con hashing e invariantes.
type Seed struct {
	Email    string
	Password string
	Name     string         // "Administrator" era una decisión de la librería. Ya no.
	Role     model.RoleCode // el nombre del rol lo pone la app
	Grants   []model.Grant  // lo que ese rol puede hacer. Vacío = no concede nada.
}

// Bootstrap siembra al primer usuario. NO-OP si la tabla ya tiene usuarios (idempotente).
// No crea roles, permisos ni comodines por su cuenta: solo lo que Seed declara.
func (m *Module) Bootstrap(s Seed) error
```

Reglas duras:

- **Cero comodines implícitos.** Si `Seed.Grants` no trae `model.Wildcard`, el rol **no**
  tiene acceso total. La app que lo quiera lo escribe, y así queda `grep`-eable.
- **Cero nombres implícitos.** `Seed.Name` vacío → error; no lo rellenes con "Administrator".
- **Cero IDs adivinados.** `role_admin`/`perm_all` los inventaba la librería en la base del
  consumidor. Deriva los IDs de lo que declara el `Seed`.
- **Validación ruidosa:** `Seed` sin `Email`, `Password`, `Name` o `Role` → error explícito.
  Un `Seed` vacío no debe sembrar nada en silencio.

## Paso 3 — `me` deja de inventar un recurso

`me` devuelve **el perfil de quien llama**: la autenticación **ya es** la comprobación. `mcp`
(fase C) declara justo ese estado:

```go
{
	Name:        "me",
	Description: "Get the profile of the currently authenticated user.",
	Access:      mcp.AccessAuthenticated, // identidad sí; recurso ninguno
	Action:      model.Read,
	// SIN Resource: esta librería no ensucia el espacio de nombres RBAC de la app.
}
```

## Paso 4 — el módulo satisface el contrato que dice satisfacer

```go
// Sin esta aserción, la mitad que falta de la interfaz solo aparecía en el consumidor, que
// acababa escribiendo un wrapper para taparla.
var _ router.APIModule = (*Module)(nil)

// ModelName es la IDENTIDAD del módulo (model.ModuleNaming) — no un recurso RBAC de la app.
// Con `me` ya sin recurso (paso 3), no queda nada con lo que divergir.
func (m *Module) ModelName() string { return "user" }
```

## Paso 5 — los guards

1. **`Bootstrap` no concede nada que el `Seed` no declare.** Siembra con
   `Grants: []model.Grant{{Resource: "catalog", Actions: model.Read}}` y afirma que ese usuario
   **NO** puede `(invoices, model.Delete)`. *Con el código actual este test es ROJO: el rol nace
   con `*:*`.* **Es el test que define este plan — compruébalo en rojo antes de arreglar.**
2. **`Bootstrap` es idempotente**: con usuarios ya en la tabla, no toca nada.
3. **`Seed` incompleto → error**, no siembra en silencio.
4. **`me` no declara recurso** y es `mcp.AccessAuthenticated`.
5. **`*Module` es un `router.APIModule`** (aserción de compilación).
6. **`Can` encaja en `model.Authorizer`** (aserción de compilación).

---

## ⚠️ Anti-footguns

- **NO dejes `RoleCodeAdmin` "por compatibilidad".** Es la política que este plan extirpa.
- **NO hagas que un `Seed` sin `Grants` conceda todo** "por comodidad": es el fail-open que
  estamos matando.
- **NO reimplementes el matching del comodín**: úsalo desde `model.Grant.Matches`. Dos
  implementaciones que discrepen sobre qué significa `*` es un agujero que nadie vería.
- **NO añadas un `CreateUser` público** para que el consumidor siembre a mano: saltaría el
  hashing y las invariantes. `Bootstrap(Seed)` es el único camino.
- Nunca ejecutes `gopush` ni `codejob`.

## Criterios de aceptación

```bash
go build ./...
grep -rn "RoleCodeAdmin\|ResourceAll\|ActionAll" --include=*.go .                  # → vacío
grep -rn '"profile"\|"admin"\|"Administrator"' --include=*.go . | grep -v _test     # → vacío
gotest                                                                              # verde
```

## Al cerrar

Vuelca a `AGENTS.md` la regla permanente: *"esta librería aporta el mecanismo (hashear,
sesiones, rutas, comprobar permisos). La política —qué roles existen, quién recibe qué, si
hay comodín— la declara el consumidor vía `Bootstrap(Seed)`. Ninguna constante de rol o de
recurso vive aquí."*

**No borres ni renombres este archivo.** El ciclo de vida lo gestiona `codejob`.
