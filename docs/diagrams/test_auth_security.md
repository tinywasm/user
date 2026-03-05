# Diagram: Core Authentication Security Flow

```mermaid
flowchart TD
    A["Start Login(email, password)"] --> B["getUserByEmail (cache/DB)"]
    B -- "Not Found" --> C["Return ErrInvalidCredentials"]
    B -- "Found" --> D["Check User Status"]
    D -- "Status == 'suspended'" --> E["Return ErrSuspended"]
    D -- "Status == 'active'" --> F["getLocalIdentity (provider='local')"]
    F -- "Not Found" --> C
    F -- "Found" --> G["bcrypt.CompareHashAndPassword"]
    G -- "Mismatch" --> C
    G -- "Match" --> H["Return User"]
    I["SetPassword(userID, password)"] --> J["len(password) < 8?"]
    J -- "Yes" --> K["Return ErrWeakPassword"]
    J -- "No" --> L["bcrypt.GenerateFromPassword"]
    L --> M["upsertIdentity (provider='local')"]
```

> **Security note:** `ErrSuspended` is distinct from `ErrInvalidCredentials`. Tests MUST assert the exact
> error type for each branch. The suspended check runs BEFORE bcrypt to avoid unnecessary CPU cost on
> locked accounts. The `SetPassword` weak-password guard is a separate flow also requiring test coverage.
