# OAuth Flow

> **Status:** Design — February 2026

Full sequence for OAuth 2.0 Authorization Code flow. Covers both `BeginOAuth` (redirect)
and `CompleteOAuth` (callback) with all decision branches.

```mermaid
sequenceDiagram
    participant Browser
    participant App as site (auth handlers)
    participant user
    participant DB
    participant Provider as OAuth Provider

    Note over Browser,Provider: Step 1 — Begin (app route: GET /auth/google/begin)
    Browser->>App: GET /auth/google/begin
    App->>user: BeginOAuth("google", w, r)
    user->>user: generate state = random 32 bytes hex
    user->>DB: INSERT oauth_states(state, "google", now)
    user->>user: build AuthURL(state, redirectURL)
    user->>Browser: 302 Redirect → Provider /authorize?state=...&client_id=...

    Note over Browser,Provider: Step 2 — User authenticates with provider
    Browser->>Provider: GET /authorize?state=...
    Provider->>Browser: show login / consent screen
    Browser->>Provider: user grants consent
    Provider->>Browser: 302 Redirect → /auth/google/callback?code=...&state=...

    Note over Browser,Provider: Step 3 — Callback (app route: GET /auth/google/callback)
    Browser->>App: GET /auth/google/callback?code=...&state=...
    App->>user: CompleteOAuth("google", r, ip, ua)

    user->>DB: SELECT oauth_states WHERE state=? AND provider="google"
    alt state not found
        DB-->>user: no rows
        user-->>App: ErrInvalidOAuthState
        App-->>Browser: 400 Bad Request
    else state found — check TTL
        DB-->>user: {state, created_at}
        alt now - created_at > 600s (10 min)
            user-->>App: ErrInvalidOAuthState
            App-->>Browser: 400 Bad Request
        else valid state
            user->>DB: DELETE oauth_states WHERE state=? (single-use)
            user->>Provider: ExchangeCode(code)
            alt exchange fails
                Provider-->>user: error
                user-->>App: error
                App-->>Browser: 500 Internal Server Error
            else exchange ok
                Provider-->>user: OAuthToken
                user->>Provider: GetUserInfo(token)
                Provider-->>user: OAuthUserInfo{ID, Email, Name}

                user->>DB: SELECT user_identities WHERE provider=? AND provider_id=?
                alt identity found (returning OAuth user)
                    DB-->>user: Identity{UserID}
                    user->>DB: SELECT users WHERE id=UserID
                    DB-->>user: User
                    user-->>App: (User, isNewUser=false, nil)
                else identity not found
                    user->>DB: SELECT users WHERE email=Email
                    alt email found (existing local account — auto-link)
                        DB-->>user: User
                        user->>DB: INSERT user_identities(id, user.ID, "google", ID, Email)
                        user-->>App: (User, isNewUser=false, nil)
                    else email not found (brand new user — auto-register, no local identity)
                        user->>DB: INSERT users(id, Email, name=Name, status="active")
                        user->>DB: INSERT user_identities(id, newUser.ID, "google", ID, Email)
                        user-->>App: (User, isNewUser=true, nil)
                    end
                end
                Note over App: Caller creates session + sets cookie (same as Login)
                alt isNewUser == true
                    App->>App: site.AssignRole(user.ID, 'v')
                end
                App-->>Browser: 302 Redirect → /home (or returnURL)
            end
        end
    end
```

## Security properties

| Property | Mechanism |
|----------|-----------|
| CSRF protection | `state` validated in DB, deleted on use (replay-proof) |
| State expiry | TTL 10 min — not reusable after window |
| Session cookie | `HttpOnly; Secure; SameSite=Strict` — XSS + CSRF resistant |
| Email enumeration | Not applicable (OAuth email is from trusted provider) |
| Account takeover | Auto-link requires matching email from verified provider |

## Configuration

```go
// In main.go, configure via Config struct:
site.SetUserConfig(user.Config{
    OAuthProviders: []user.OAuthProvider{
        &user.GoogleProvider{
            ClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
            ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
            RedirectURL:  "https://example.com/oauth/callback",
        },
        &user.MicrosoftProvider{
            ClientID:     os.Getenv("AZURE_CLIENT_ID"),
            ClientSecret: os.Getenv("AZURE_CLIENT_SECRET"),
            RedirectURL:  "https://example.com/oauth/callback",
            TenantID:     "common",
        },
    },
})

// OAuthCallback module handles the routes automatically:
// GET /auth/<provider>/begin     → user.BeginOAuth(provider, w, r)
// GET /oauth/callback            → u, isNew, err := user.CompleteOAuth(provider, r, ip, ua)
//                                  if isNew { site.AssignRole(u.ID, 'v') }
```

## Tests

| Test | Branch covered |
|------|---------------|
| `TestOAuth_NewUser_AutoRegister` | email not found → create user, isNewUser=true |
| `TestOAuth_ExistingOAuthUser` | known (provider, id) → user, isNewUser=false |
| `TestOAuth_LinkToLocalAccount` | OAuth email matches local user → link identity, isNewUser=false |
| `TestOAuth_InvalidState` | state not in DB → ErrInvalidOAuthState |
| `TestOAuth_ExpiredState` | state older than 10 min → ErrInvalidOAuthState |
| `TestOAuth_StateConsumedOnce` | second request with same state → error (replay-proof) |
| `TestUnlinkIdentity_LastIdentity` | only identity remaining → ErrCannotUnlink |
| `TestUnlinkIdentity_MultipleIdentities` | two identities → unlink succeeds |
