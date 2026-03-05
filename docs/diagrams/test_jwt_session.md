# Diagram: JWT & Session Lifecycle

```mermaid
flowchart TD
    A["GenerateJWT(secret, userID, ttl)"] --> B["Build jwtHeader + jwtPayload"]
    B --> C["jwtSign(secret, header.payload)"]
    C --> D["Return 3-segment token"]
    E["ValidateJWT(secret, token)"] --> F["SplitN on '.' - 3 parts?"]
    F -- "No (0-2 parts)" --> G["Return ErrInvalidToken"]
    F -- "Yes" --> H["Recompute HMAC-SHA256 signature"]
    H -- "Mismatch (tampered header/payload)" --> G
    H -- "Match" --> I["base64.RawURLEncoding.Decode payload"]
    I -- "Invalid base64" --> G
    I -- "Valid" --> J["json.Unmarshal into jwtPayload"]
    J -- "JSON error" --> G
    J -- "OK" --> K["time.Now().Unix() > p.Exp?"]
    K -- "Yes" --> L["Return ErrSessionExpired"]
    K -- "No" --> M["Return p.Sub (userID)"]
    N["PurgeExpiredSessions()"] --> O["Lock sessionCache.mu"]
    O --> P["Iterate cache items:<br/>ExpiresAt < now?"]
    P -- "Yes" --> Q["delete(cache.items, key)"]
    P -- "No" --> R["Keep"]
    Q & R --> S["Unlock mu"]
    S --> T["Query DB: ExpiresAt < now"]
    T --> U["db.Delete each expired session"]
```

> **Alg-none attack:** not applicable. `ValidateJWT` ignores the `alg` header field and always applies
> HMAC-SHA256. An injected `"alg":"none"` cannot bypass signature verification.
> **iat field:** a future `iat` value is accepted (no issuance clock-skew check). Document as known trade-off.
