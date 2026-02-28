# Testing Plan: Validating Hydration and Caching (`user`)

## Goal
Ensure the refactored code (Domain, ORM queries, and Cache invalidation) is entirely covered through real in-memory SQLite integration tests.

## Execution Steps

### Phase 5: Strict Testing and Migration of Legacy Tests
**Target Files:**
- `tests/rbac_*.go` (Legacy tests already ported to the `user/tests/` folder)
- `setup_test.go` (Update test database initialization)

**Instructions:**
1. The old RBAC tests have been copied into `user/tests/` and prefixed with `rbac_` (e.g. `rbac_roles_test.go`). You must **refactor** these tests so they stop importing the old `rbac` package and instead test the new unified logic inside the `user` package.
2. Update all function calls in these tests to use the new `user_rbac_mutations.go` functions.
3. **In-Memory Database for Tests:** Instead of writing raw mocks for the `Executor` or dealing with `database/sql`, leverage the actual un-mocked `tinywasm/orm` combined with `tinywasm/sqlite` in in-memory mode (`:memory:`). This prevents repetitive mock code and tests query semantics accurately.
   - **Speed:** In-memory operations are very fast, making test suites quick.
   - **Realism:** Real SQLite parsing validates translations and avoids false-positive mocks.
   - **Integration:** Initialize directly via `sqlite.Open(":memory:")` which returns an `*orm.DB`.
4. **Main Test Case (Invalidation):** Write or modify a test to explicitly verify Cache Invalidation. Create a User, assign them a Role (e.g., "Editor"), and load the user to force them into the new `userCache`. Then, assign a new `Permission` to the "Editor" Role. Load the user again and perform a strict assertion verifying that the user in memory NOW has that new permission listed in their `[]Permission` slice.
5. Test by running native `gotest` in the root of the project.

---

## 📟 Annex B: Testing Strategy (Integration Only)

Given the new `tinywasm/orm` architecture, unit testing SQL strings via a custom QueryBuilder interface is completely redundant and prone to false positives since the ORM handles AST compilation. Rely entirely on **realistic Integration Tests**.

### Integration Tests (In-Memory Database)
- Use `github.com/tinywasm/sqlite` with `:memory:` database instead of manual mocks.
- Test full flows: User creation → RBAC assignment → Cache invalidation.
- Verify that cache is correctly populated after queries executed via `tinywasm/orm`.
- Fast (in-memory) but highly realistic as it validates your queries against a true SQL parser logic.

**Example dependencies in go.mod:**
```
require (
    github.com/tinywasm/orm v0.X.X
    github.com/tinywasm/sqlite v0.X.X
)
```

**Test Setup (setup_test.go):**
```go
import (
    "testing"
    "github.com/tinywasm/orm"
    "github.com/tinywasm/sqlite"
)

func setupTestDB(t *testing.T) *orm.DB {
    db, err := sqlite.Open(":memory:")
    if err != nil {
        t.Fatalf("failed to open in-memory DB: %v", err)
    }
    
    // Run schema migrations using the ORM Executor
    if err := createSchema(db); err != nil {
        t.Fatalf("failed to create schema: %v", err)
    }
    return db
}

func createSchema(db *orm.DB) error {
    // Execute CREATE TABLE statements for users, roles, permissions, assignments
    // Example: db.Executor().Exec("CREATE TABLE users ...")
    return nil // implement migrations
}
```
