---
PLAN: "feat: exportar las rutas de OAuth2 para que el consumidor no las duplique"
EXECUTOR: jules
REVIEWER: none
STATUS: review
SESSION: 612294461273881843
PR: https://github.com/tinywasm/user/pull/23
---

> Este plan se despacha con el flujo CodeJob. Ver skill: agents-workflow.

# Plan — `tinywasm/user`: las rutas de OAuth2 dejan de ser literales

## El problema, medido en producción

`oauth2.Authenticator.Mount` registra dos rutas construyendo cadenas a mano:

```go
r.Get("/oauth/"+providerName, ...)
r.Get("/oauth/callback/"+providerName, ...)
```

Esos dos literales son la **única** definición de las rutas, y viven dentro del
cuerpo de `Mount`. Un consumidor que necesita enlazar el botón de "Iniciar sesión
con Google", o registrar la URI de redirección en la consola de Google, no tiene
ningún símbolo que referenciar: **está obligado a volver a escribir la cadena**.

Lo que pasa cuando alguien la escribe distinto se midió el 2026-08-23 en
`veltylabs/misitio`, un consumidor real: declaraba
`PathLoginGoogle = "/api/login/google"` mientras esta librería montaba
`/oauth/google`. El botón apuntaba a una ruta inexistente, el servidor devolvía
el shell estático con `200`, y **el login estuvo caído sin que ningún test, ni
aquí ni allá, se pusiera rojo**. El compilador no podía verlo: son dos cadenas
que nadie obliga a coincidir.

Este paquete ya exporta `PathLogin`, `PathLogout` y `PathAfterLogin`. Las de
OAuth2 —justo las que un consumidor necesita para pintar el botón— faltan. La
superficie está incompleta, y la incompletitud **fuerza** la duplicación aguas
abajo.

## La decisión

Las rutas se exportan como funciones puras del nombre del proveedor, y `Mount`
pasa a construirlas con ellas. Una sola definición, referenciable desde fuera.

El prefijo `/oauth/` queda **fijo, no configurable**. Un `WithPathPrefix` haría
que `PathOAuthStart` dejara de ser función pura del proveedor y volvería a abrir
el hueco: el consumidor tendría que saber qué prefijo se configuró en el arranque
para poder enlazar el botón. Una sola forma, verificable en compilación.

## Reglas de código — obligatorias

### Nada de literales repetidos

```
REGLA: toda cadena repetida (ruta, prefijo, nombre de proveedor) es una constante
o función con nombre. Prohibido el literal suelto en la lógica.
```

Tras este plan, la cadena `"/oauth/"` debe aparecer **exactamente una vez** en
todo el repositorio: en la declaración de `PathOAuthPrefix`.

### Sin biblioteca estándar

Este paquete compila también a WASM. Usa `github.com/tinywasm/fmt` para todo lo
que en stdlib sería `strings`/`strconv`/`errors`. La concatenación con `+` de
cadenas es Go puro y está permitida: **no** importes nada nuevo para esto.

### Superficie mínima

Exporta exactamente lo que un consumidor necesita: el prefijo y las dos
funciones. Nada más.

---

## Etapa 1 — Exportar las rutas

**Archivo:** `user.go`, junto al bloque que ya declara `PathLogin`.

```go
const (
	PathLogin      = "/login"
	PathLogout     = "/logout"
	PathAfterLogin = "/"

	// PathOAuthPrefix es la raiz bajo la que oauth2.Authenticator monta sus
	// rutas. Es la unica definicion de esa cadena en el repositorio.
	PathOAuthPrefix = "/oauth/"
)

// PathOAuthStart devuelve la ruta que inicia el intercambio OAuth2 con el
// proveedor indicado. Un consumidor enlaza aqui su boton de "iniciar sesion".
func PathOAuthStart(provider string) string {
	return PathOAuthPrefix + provider
}

// PathOAuthCallback devuelve la ruta a la que el proveedor redirige de vuelta.
// Es el valor que se registra como URI de redireccion en la consola del
// proveedor, precedido del dominio publico de la aplicacion.
func PathOAuthCallback(provider string) string {
	return PathOAuthPrefix + "callback/" + provider
}
```

**Criterio:** las dos funciones existen y son puras — no leen estado, no dependen
de que `Mount` se haya llamado.

---

## Etapa 2 — `Mount` consume sus propias rutas

**Archivo:** `oauth2/oauth.go`.

Sustituye los dos literales dentro de `Mount`:

```go
r.Get(user.PathOAuthStart(providerName), func(ctx router.Context) {
	...
}).Public()

r.Get(user.PathOAuthCallback(providerName), func(ctx router.Context) {
	...
}).Public()
```

No cambies nada más de esos manejadores: ni el cuerpo, ni el orden, ni el
`.Public()`.

**Criterio de aceptación (verificable con grep):**

```
grep -rn '"/oauth/' .   → una sola linea: la declaracion de PathOAuthPrefix en user.go
```

---

## Etapa 3 — El nombre del proveedor también deja de ser literal

**Archivos:** `oauth2/provider/google/google.go`, `oauth2/provider/microsoft/microsoft.go`.

Hoy `Name()` devuelve un literal. Un consumidor que escribe
`user.PathOAuthStart("google")` sigue teniendo una cadena que nadie verifica.

En cada paquete de proveedor:

```go
// ProviderName identifica a este proveedor en las rutas y en el almacen de
// estado. Un consumidor lo usa con user.PathOAuthStart / PathOAuthCallback.
const ProviderName = "google"      // "microsoft" en el otro paquete

func (p *GoogleProvider) Name() string { return ProviderName }
```

**Criterios:**

```
grep -rn 'return "google"'    . → vacio
grep -rn 'return "microsoft"' . → vacio
```

---

## Etapa 4 — El test que habría detectado la caída

**Archivo nuevo:** `tests/oauth_routes_test.go` (paquete `user_test`, en el módulo
`tests/` que ya existe).

Este es el test que faltaba: **con forma de consumidor**, montando el
autenticador real sobre el `mock.Router` de `tinywasm/router` y comprobando que
las rutas registradas son exactamente las que las funciones exportadas prometen.

```go
func TestOAuthRoutesMatchExportedPaths(t *testing.T) {
	// Monta el modulo real con el proveedor real de Google (sin credenciales:
	// no se ejecuta ningun intercambio, solo se registran rutas).
	// ... construccion igual que en tests/production_wiring_test.go: authority.New
	// + m.Enable(oauth2.New(m, m, m, []user.OAuthProvider{&google.GoogleProvider{}}))

	r := &mock.Router{}
	m.MountAPI(r)

	want := map[string]bool{
		user.PathOAuthStart(google.ProviderName):    false,
		user.PathOAuthCallback(google.ProviderName): false,
	}
	for _, info := range r.Routes() {
		if _, ok := want[info.Path]; ok {
			want[info.Path] = true
		}
	}
	for path, found := range want {
		if !found {
			t.Errorf("la ruta exportada %q no fue registrada por Mount", path)
		}
	}
}
```

Dos comprobaciones más en el mismo archivo:

1. **Las dos rutas son públicas.** Un `.Public()` perdido deja el login detrás de
   la sesión que el login mismo crea. Recorre `r.Routes()` y falla si alguna de
   las dos no lo es — mismo patrón que `testMountAPI` en
   `tests/production_wiring_test.go`.
2. **Los valores literales quedan clavados**, para que un cambio de ruta sea una
   decisión consciente y no un accidente de refactor:

   ```go
   if got := user.PathOAuthStart("google"); got != "/oauth/google" {
       t.Errorf("PathOAuthStart cambio: %q", got)
   }
   if got := user.PathOAuthCallback("google"); got != "/oauth/callback/google" {
       t.Errorf("PathOAuthCallback cambio: %q", got)
   }
   ```

   Si una versión futura mueve las rutas, este test se pone rojo **aquí**, en la
   librería, en vez de en producción de un consumidor.

**Criterio:** `cd tests && go test ./...` verde.

---

## Etapa 5 — Documentación

- **`docs/SKILL.md`** línea 33: el comentario que lista las rutas
  (`// POST /login, POST /logout, GET /oauth/:provider, GET /oauth/callback/:provider`)
  pasa a nombrar los símbolos, y se agrega una fila a la tabla de "cómo hago X":

  | Quiero… | Uso |
  |---|---|
  | enlazar el botón de login social | `user.PathOAuthStart(google.ProviderName)` |
  | registrar la URI de redirección en el proveedor | dominio público + `user.PathOAuthCallback(google.ProviderName)` |

- **`README.md`** línea 165: el ejemplo con
  `RedirectURL: "https://miapp.cl/oauth/callback/google"` pasa a construirse con
  la función, para que el ejemplo enseñe el símbolo y no la cadena:

  ```go
  RedirectURL: "https://miapp.cl" + user.PathOAuthCallback(google.ProviderName),
  ```

- **`README.md`** línea 237: donde dice `/oauth/:provider`, nombrar también las
  dos funciones exportadas.

- **`docs/ARCHITECTURE.md`**: si describe el montaje de rutas, misma corrección.
  Si no las menciona, no lo toques.

**Criterio:** `grep -rn '/oauth/callback/google' docs README.md` → vacío.

---

## Tabla de etapas

| # | Qué | Archivos | Cierra cuando |
|---|---|---|---|
| 1 | Exportar `PathOAuthPrefix`, `PathOAuthStart`, `PathOAuthCallback` | `user.go` | compila |
| 2 | `Mount` las consume | `oauth2/oauth.go` | `grep -rn '"/oauth/' .` da una sola línea |
| 3 | `ProviderName` por proveedor | `oauth2/provider/google/`, `oauth2/provider/microsoft/` | los dos greps de la etapa 3 vacíos |
| 4 | Test con forma de consumidor | `tests/oauth_routes_test.go` (nuevo) | `cd tests && go test ./...` verde |
| 5 | Documentación | `docs/SKILL.md`, `README.md`, `docs/ARCHITECTURE.md` | `grep -rn '/oauth/callback/google' docs README.md` vacío |

Secuenciales: la 2 necesita la 1, la 4 necesita la 3.

## Verificación final

```bash
go vet ./...
cd tests && go test ./...
grep -rn '"/oauth/' ..     # una sola linea, en user.go
```

**Compatibilidad:** este plan **no cambia ninguna ruta**. `/oauth/google` y
`/oauth/callback/google` siguen siendo exactamente las mismas cadenas; lo único
que cambia es que ahora tienen nombre. Ningún consumidor se rompe al actualizar.
