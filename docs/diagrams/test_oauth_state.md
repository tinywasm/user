# Diagram: OAuth State Flow & Security

```mermaid
flowchart TD
    A["BeginOAuth(provider)"] --> B["Generate Unique State token"]
    B --> C["INSERT oauth_states(state, provider, expiresAt)"]
    D["CompleteOAuth(provider, r)"] --> E["consumeState(db, state, provider)"]
    E --> F["SELECT oauth_states WHERE state=?"]
    F -- "Not Found (len=0)<br/>Replay: already consumed" --> G["Return ErrInvalidOAuthState<br/>state gone — no delete needed"]
    F -- "Found" --> CP["stateObj.Provider == provider?"]
    CP -- "No (Cross-Provider Hijack)<br/>state NOT deleted — preserved for real provider" --> G
    CP -- "Yes" --> DEL["DELETE state from DB<br/>(single-use — delete before expiry check)"]
    DEL --> EXP["stateObj.ExpiresAt < now?"]
    EXP -- "Yes (Expired)<br/>state already deleted above" --> G
    EXP -- "No (valid)" --> OK["Return nil<br/>proceed to ExchangeCode + GetUserInfo"]
```

> **Deletion order matters:**
> - Cross-provider mismatch returns **before** delete → state preserved, legitimate provider A flow still works.
> - Expiry check happens **after** delete → expired state is always cleaned up even on failure.
> - "Not Found" path (len=0) means state was already consumed (true replay) — nothing to delete.
