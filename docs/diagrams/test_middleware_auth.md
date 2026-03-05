# Diagram: Middleware Authentication Context

```mermaid
flowchart TD
    A["Middleware(next) - HTTP Request"] --> B["r.Cookie(CookieName)"]
    B -- "Missing Cookie" --> C["http.Error 401 Unauthorized"]
    B -- "Cookie Present" --> D["validateSession(r)"]
    D -- "AuthModeJWT" --> E["ValidateJWT(secret, cookie.Value)"]
    D -- "AuthModeCookie" --> F["GetSession(cookie.Value)"]
    E -- "Fail (tampered/expired)" --> C
    E -- "Success → userID" --> G["GetUser(userID)"]
    F -- "Fail (not found/expired)" --> C
    F -- "Success → sess.UserID" --> G
    G -- "Fail" --> C
    G -- "Success" --> H["context.WithValue(ctx, userKey, user)"]
    H --> I["next.ServeHTTP(w, r.WithContext(ctx))"]
    J["AccessCheck(resource, action, data...)"] --> K["Extract *http.Request from data"]
    K -- "No request found" --> L["Return false"]
    K -- "Found" --> M["validateSession(r)"]
    M -- "Fail" --> L
    M -- "Success" --> N["HasPermission(userID, resource, action)"]
    N --> O["Return bool"]
    P["mcpContextFunc() - MCP SSE/HTTP"] --> Q["validateSession(r)"]
    Q -- "Fail" --> R["Return ctx unchanged<br/>(unauthenticated)"]
    Q -- "Success" --> S["context.WithValue(ctx, userKey, user)"]
```

> **Security note (JWT mode):** `validateSession` with JWT does NOT check `u.Status`. A suspended user
> retains middleware access until JWT expiry. Test MUST document this as a known limitation — suspension
> only prevents new sessions/logins, but active JWTs remain valid.
> **`FromContext` type safety:** if the context value is not a `*User`, it returns `(nil, false)` safely.
