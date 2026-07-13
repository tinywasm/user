← [Stage 3](PLAN_STAGE_3_EDGE_STDLIB.md) | Next → [Stage 5](PLAN_STAGE_5_OWASP.md)

# Stage 4 — isolate the test dependencies in `tests/go.mod`

`tests/` imports `github.com/tinywasm/sqlite` (`tests/suite_back_test.go`),
which drags a full CGO-free SQLite implementation into the **root** module's
dependency graph. A DB-agnostic, edge-targeted library must not carry a driver.

## Work

1. Create `tests/go.mod`:

   ```
   module github.com/tinywasm/user/tests

   go 1.25.2

   require (
       github.com/tinywasm/user v0.0.0
       github.com/tinywasm/sqlite v0.2.4
       // ... whatever `go mod tidy` resolves
   )

   replace github.com/tinywasm/user => ../
   ```

   The `replace` makes the suite always run against the working-tree library
   (correct for PR branches; no version pinning).

2. Run `go mod tidy` inside `tests/`.

3. Delete the blank import `_ "modernc.org/sqlite"` from
   `tests/user_back_test.go` — it is redundant: `tinywasm/sqlite` registers the
   driver internally (its `adapter.go` has the blank import).

4. Run `go mod tidy` at the **root**. `github.com/tinywasm/sqlite`,
   `modernc.org/sqlite` and their transitive deps (`modernc.org/libc`,
   `dustin/go-humanize`, `ncruces/go-strftime`, `remyoudompheng/bigfft`, …)
   disappear.

   Tidy will **also** drop `github.com/tinywasm/dom` and
   `github.com/tinywasm/html` (plus the indirects `tinywasm/css`,
   `tinywasm/sqlt`). That is **expected and correct**: they are dead requires
   left behind when the `ui/` package was deleted — nothing imports them any
   more. **Do not re-add them.**

5. **Test invocation changes** — `./...` does not traverse nested modules:

   ```
   gotest ./...              # repo root (library compiles; no test files — this is green)
   cd tests && gotest ./...  # the actual suite (native + wasm)
   ```

   If a CI workflow file exists, update it to run both. Do **not** merge the
   two modules back together to "simplify CI".

## Acceptance

- Root `go.mod` contains no `sqlite` (tinywasm or modernc) and no modernc
  transitive deps.
- Root `go.mod` gains **no** dependency; it only loses them.
- `go build ./...` and `GOOS=js GOARCH=wasm go build ./...` succeed at the root.
- `gotest ./...` green at the root and inside `tests/`.
