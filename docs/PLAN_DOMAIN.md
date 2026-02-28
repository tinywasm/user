# Domain & State Plan: Hydrated User Models & Cache (`user`)

## Goal
Extend the base `user` structs to directly hold Role and Permission slices, eliminating map-based storage. Implement an isolated, bounded in-memory cache for users with strict limits.

## Execution Steps

### Phase 1: Domain Extension in `user` (Structs)
**Target Files:**
- `user.go`
- Create new: `user_roles.go`
- Create new: `user_permissions.go`

**Instructions (WASM & TinyGo Compatibility):**
1. Modify the `User` struct in `user.go` by adding two explicit slices to hold roles (`[]Role`) and permissions (`[]Permission`). **CRITICAL:** Do NOT use Go Maps (`map[string]bool`) to store roles/permissions to prevent WASM binary bloat. Always use Structs or Slices for small collections as dictated by `DEFAULT_LLM_SKILL.md`.
2. Move the base RBAC structures by defining them in the `user` package:
   - Create the `Role` struct in `user_roles.go`.
   - Create the `Permission` struct in `user_permissions.go`.
   *(See `PLAN_RBAC.md` for legacy reference on what properties they need).*
3. Every file must use the custom `tinywasm` standard library replacements for frontend compatibility. Instead of standard libraries, rely on:
   - `github.com/tinywasm/fmt` instead of `fmt`/`strings`/`strconv`/`errors`
   - `github.com/tinywasm/time` instead of `time`
   - `github.com/tinywasm/json` instead of `encoding/json`

### Phase 2: Implementation of User Cache with Limits
**Target Files:**
- `cache.go` (Keep existing session cache intact here if they coexist, or create a specific one for users)
- Create new: `cache_users.go`
- Modify: `sql.go` (extend `Store` struct)

**Instructions:**
1. Create a new cache structure for users `userCache` (currently `cache.go` is focused on sessions, do not break it, keep them separated).
2. The cache must have a hard limit of **1000 users**. Implement a basic LRU or FIFO cleanup logic when the map exceeds 1000 entries to prevent OOM (Out Of Memory).
3. **Central Invalidation Logic:** Add methods so that if a base "Role" changes (e.g., a new permission is assigned to it), the cached users possessing that `RoleID` update their permissions list or their entries are evicted from the cache (e.g., `InvalidateByRole(roleID)` and `InvalidateByPermission(permID)`).
