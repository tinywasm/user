# Auth Flow

> **Status:** Design — February 2026

`Login` validates credentials only. Session creation is a separate explicit step
(`CreateSession`). This follows Single Responsibility — authentication is stateless.

```mermaid
sequenceDiagram
    participant App
    participant user
    participant DB

    App->>user: Login(email, password)
    user->>DB: GetUserByEmail(email)

    alt user not found
        DB-->>user: ErrNotFound
        user-->>App: ErrInvalidCredentials
        Note over App: Same error as wrong password
        Note over App: Prevents email enumeration
    else user found
        DB-->>user: User{ID, status}

        alt status == "suspended"
            user-->>App: ErrSuspended
        else status == "active"
            user->>DB: SELECT user_identities WHERE user_id=ID AND provider='local'

            alt no local identity (OAuth/LAN-only user)
                DB-->>user: no rows
                user-->>App: ErrInvalidCredentials
                Note over App: Same error — no info leak
            else local identity found
                DB-->>user: Identity{ProviderID: bcryptHash}
                user->>user: bcrypt.CompareHashAndPassword(hash, password)

                alt hash mismatch
                    user-->>App: ErrInvalidCredentials
                else hash match
                    user-->>App: User{ID, Email, Name, Phone, Status}
                end
            end
        end
    end

    Note over App: If Login succeeds, App calls:
    App->>user: CreateSession(user.ID, ip, userAgent)
    user-->>App: Session{ID, UserID, ExpiresAt}
    Note over App: LoginModule.Create handles this automatically
```

## Security notes

- `ErrInvalidCredentials` returned for "user not found", "no local identity", and "wrong
  password" — no information leaked about which condition occurred.
- bcrypt cost: 12 (fixed). Not configurable — safe default for production use.
- OAuth-only and LAN-only users have no `local` identity, so `Login(email, "")` always
  fails with `ErrInvalidCredentials`.
- `Login` is pure CPU work — no session state written until `CreateSession` is called explicitly.

## Tests

| Test | Branch |
|------|--------|
| `TestLogin_ValidCredentials` | local identity found + hash match + active → User |
| `TestLogin_InvalidPassword` | hash mismatch → ErrInvalidCredentials |
| `TestLogin_UserNotFound` | DB miss → ErrInvalidCredentials (same error, no enum) |
| `TestLogin_NoLocalIdentity` | OAuth-only user → ErrInvalidCredentials |
| `TestLogin_SuspendedUser` | status=="suspended" → ErrSuspended |
