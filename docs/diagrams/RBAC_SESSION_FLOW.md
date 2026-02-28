# Session Flow

> **Status:** Design — February 2026

Sessions are stored in `user_sessions` and cached in memory. The hot path (per-request
`GetSession`) hits the cache only — zero DB I/O when the session is valid.

```mermaid
sequenceDiagram
    participant Handler as LoginModule handler
    participant user
    participant Cache
    participant DB

    Note over Handler,DB: Session creation (on Login via LoginModule)
    Handler->>Handler: LoginModule.Create(LoginData)
    Handler->>user: Login(email, password)
    user-->>Handler: User
    Handler->>user: CreateSession(user.ID, ip, userAgent)
    user->>user: generate sessionID (unixid)
    user->>user: expiresAt = now + TTL (default 24h)
    user->>DB: INSERT user_sessions
    user->>Cache: store(sessionID → Session)
    user-->>Handler: Session{ID, UserID, ExpiresAt}
    Note over Handler: Set-Cookie session=ID (HttpOnly, Secure, SameSite=Strict, Max-Age=TTL)

    Note over Handler,DB: Per-request session validation (extractUserID)
    Handler->>Handler: extractUserID(*http.Request)
    Handler->>Handler: r.Cookie(sessionCookieName)
    alt no cookie
        Handler-->>Handler: "" (anonymous)
    else cookie present
        Handler->>user: GetSession(sessionID)
        user->>Cache: lookup(sessionID)
        alt cache hit
            Cache-->>user: Session
            alt Session.ExpiresAt < now
                user-->>Handler: ErrSessionExpired
            else valid
                user-->>Handler: Session.UserID
            end
        else cache miss
            user->>DB: SELECT WHERE id=?
            alt not found
                DB-->>user: ErrNotFound
                user-->>Handler: error
            else found
                DB-->>user: Session row
                alt expired
                    user-->>Handler: ErrSessionExpired
                else valid
                    user-->>Handler: Session.UserID
                end
            end
        end
    end

    Note over Handler,DB: Logout
    Handler->>Handler: r.Cookie(sessionCookieName)
    Handler->>user: DeleteSession(sessionID)
    user->>DB: DELETE WHERE id=?
    user->>Cache: delete(sessionID)
    Note over Handler: Set-Cookie session= (MaxAge=-1, clear cookie)
```

## Tests

| Test | Branch |
|------|--------|
| `TestSession_CreateAndGet` | create → cache hit → valid Session |
| `TestSession_CacheMiss` | create → evict from cache → DB hit → valid |
| `TestSession_Expired` | ExpiresAt in past → ErrSessionExpired |
| `TestSession_Delete` | delete → subsequent get → error |
| `TestSession_NoCookie` | request without cookie → empty userID |
