# User CRUD Flow

> **Status:** Design — February 2026

User creation, update, and suspension. Reads are cache-free (DB query on each call) —
users are infrequent reads compared to sessions.

```mermaid
sequenceDiagram
    participant App
    participant user
    participant DB

    Note over App,DB: CreateUser (no password — use SetPassword separately)
    App->>user: CreateUser(email, name, phone)
    user->>user: unixid.NewUnixID() → id
    user->>DB: INSERT users (id, email, name, phone, created_at)

    alt UNIQUE constraint on email
        DB-->>user: constraint error
        user-->>App: ErrEmailTaken
    else success
        DB-->>user: ok
        user-->>App: User{ID, Email, Name, Phone, Status:"active", CreatedAt}
    end

    Note over App,DB: SetPassword (creates local identity)
    App->>user: SetPassword(userID, password)
    user->>user: validate password length >= 8
    alt too short
        user-->>App: ErrWeakPassword
    else valid
        user->>user: bcrypt.GenerateFromPassword(password, cost=12)
        user->>DB: UPSERT user_identities (provider='local', provider_id=hash)
        DB-->>user: ok
        user-->>App: nil
    end

    Note over App,DB: GetUser / GetUserByEmail
    App->>user: GetUserByEmail(email)
    user->>DB: SELECT WHERE email=?
    alt not found
        DB-->>user: no rows
        user-->>App: ErrNotFound
    else found
        DB-->>user: User row
        user-->>App: User
    end

    Note over App,DB: UpdateUser
    App->>user: UpdateUser(id, name, phone)
    user->>DB: UPDATE users SET name=?, phone=? WHERE id=?
    alt id not found
        DB-->>user: 0 rows affected
        user-->>App: ErrNotFound
    else success
        DB-->>user: 1 row affected
        user-->>App: nil
    end

    Note over App,DB: SuspendUser
    App->>user: SuspendUser(id)
    user->>DB: UPDATE users SET status='suspended' WHERE id=?
    alt id not found
        DB-->>user: 0 rows affected
        user-->>App: ErrNotFound
    else success
        DB-->>user: 1 row affected
        user-->>App: nil
        Note over user: Existing sessions remain valid until expiry
        Note over user: Next Login attempt returns ErrSuspended
    end

    Note over App,DB: ReactivateUser
    App->>user: ReactivateUser(id)
    user->>DB: UPDATE users SET status='active' WHERE id=?
    alt id not found
        DB-->>user: 0 rows affected
        user-->>App: ErrNotFound
    else success
        DB-->>user: 1 row affected
        user-->>App: nil
    end
```

## Notes

- No `password_hash` column in `users` — passwords live in `user_identities` (`provider='local'`).
- `CreateUser` does NOT create a local identity. Call `SetPassword` explicitly to enable login.
- OAuth and LAN users are created without calling `SetPassword` — they can't password-login.
- `SuspendUser` does NOT delete existing sessions. They expire naturally.
  If immediate revocation is needed, call `DeleteSession` for each active session.
- `UpdateUser` updates `name` and `phone` only. Email changes are
  not yet supported (out of scope for v0.1).
- Password changes use `SetPassword` (in `auth.go`, not `crud.go`).

## Tests

| Test | Branch |
|------|--------|
| `TestCreateUser_Success` | insert OK → User with status "active" (no local identity) |
| `TestCreateUser_DuplicateEmail` | UNIQUE violation → ErrEmailTaken |
| `TestGetUserByEmail_NotFound` | no rows → ErrNotFound |
| `TestGetUser_Success` | found → User |
| `TestUpdateUser_Success` | 1 row affected → nil |
| `TestUpdateUser_NotFound` | 0 rows affected → ErrNotFound |
| `TestSuspendUser_Success` | status updated → nil |
| `TestReactivateUser_Success` | status → "active" |
| `TestSuspendedUser_BlocksLogin` | suspended → Login returns ErrSuspended |
| `TestCreateUser_ThenSetPassword` | CreateUser + SetPassword → Login works |
| `TestSetPassword_WeakPassword` | < 8 chars → ErrWeakPassword |
