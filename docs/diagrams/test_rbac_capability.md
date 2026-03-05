# Diagram: RBAC Capability Verification

```mermaid
flowchart TD
    A["HasPermission(userID, resource, action)"] --> B["GetUser(userID)"]
    B -- "err == ErrNotFound or sql.ErrNoRows" --> C["Return false, nil"]
    B -- "other error" --> D["Return false, err"]
    B -- "Success (any status)" --> E["Iterate u.Permissions"]
    E -- "p.Resource==resource AND p.Action==string(action)" --> F["Return true, nil"]
    E -- "End of loop (no match)" --> G["Return false, nil"]
```

> **Suspended users:** `HasPermission` does NOT check `u.Status`. A suspended user who still has a valid
> context (e.g., JWT not yet expired) can pass `HasPermission` checks. Only `Login` blocks suspended
> users at authentication time. Tests MUST cover this case explicitly and document it as a known limitation.
> **Error mapping:** `ErrNotFound` and `sql.ErrNoRows` both map to `(false, nil)`. Other DB errors bubble up.
