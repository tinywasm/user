package userserver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"time"

	"github.com/tinywasm/orm"
	"github.com/tinywasm/router"
	"github.com/tinywasm/unixid"
	"github.com/tinywasm/user"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"golang.org/x/oauth2/microsoft"
)

func (m *Module) BeginOAuth(providerName string) (string, error) {
	p := m.getProvider(providerName)
	if p == nil {
		return "", user.ErrProviderNotFound
	}

	u, err := unixid.NewUnixID()
	if err != nil {
		return "", err
	}
	state := u.GetNewID()

	now := time.Now().Unix()
	expiresAt := now + 600 // 10 minutes

	stateObj := &user.OAuthState{
		State:     state,
		Provider:  providerName,
		ExpiresAt: expiresAt,
		CreatedAt: now,
	}

	if err := m.db.Create(stateObj); err != nil {
		return "", err
	}

	return p.AuthCodeURL(state), nil
}

func (m *Module) CompleteOAuth(providerName string, ctx router.Context, ip, ua string) (user.User, bool, error) {
	var state, code string
	if q, ok := ctx.Value("query").(url.Values); ok {
		state = q.Get("state")
		code = q.Get("code")
	} else {
		// Fallback: parse from Path if it's a full URL or contains query
		if u, err := url.Parse(ctx.Path()); err == nil {
			state = u.Query().Get("state")
			code = u.Query().Get("code")
		}
	}

	if err := consumeState(m.db, state, providerName); err != nil {
		return user.User{}, false, user.ErrInvalidOAuthState
	}

	p := m.getProvider(providerName)
	if p == nil {
		return user.User{}, false, user.ErrProviderNotFound
	}

	// Use background context as router.Context doesn't provide one
	bgCtx := context.Background()

	token, err := p.ExchangeCode(bgCtx, code)
	if err != nil {
		return user.User{}, false, err
	}

	info, err := p.GetUserInfo(bgCtx, token)
	if err != nil {
		return user.User{}, false, err
	}

	identity, err := getIdentityByProvider(m.db, providerName, info.ID)
	if err == nil {
		u, err := getUser(m.db, m.ucache, identity.UserID)
		return u, false, err
	}

	u, err := getUserByEmail(m.db, m.ucache, info.Email)
	if err == nil {
		_ = createIdentity(m.db, u.ID, providerName, info.ID, info.Email)
		return u, false, nil
	}

	u, err = createUser(m.db, info.Email, info.Name, "")
	if err != nil {
		return user.User{}, false, err
	}
	_ = createIdentity(m.db, u.ID, providerName, info.ID, info.Email)
	return u, true, nil
}

func consumeState(db *orm.DB, state, provider string) error {
	qb := db.Query(&user.OAuthState{}).Where(user.OAuthState_.State).Eq(state)
	results, err := user.ReadAllOAuthState(qb)
	if err != nil {
		return err
	}
	if len(results) == 0 {
		return user.ErrInvalidOAuthState
	}
	stateObj := results[0]

	if stateObj.Provider != provider {
		return user.ErrInvalidOAuthState
	}

	// Delete state (single use) - done regardless of expiration to prevent reuse
	if err := db.Delete(stateObj, orm.Eq(user.OAuthState_.State, stateObj.State)); err != nil {
		return err
	}

	if stateObj.ExpiresAt < time.Now().Unix() {
		return user.ErrInvalidOAuthState
	}

	return nil
}

func (m *Module) PurgeExpiredOAuthStates() error {
	qb := m.db.Query(&user.OAuthState{}).Where(user.OAuthState_.ExpiresAt).Lt(time.Now().Unix())
	states, _ := user.ReadAllOAuthState(qb)
	for _, s := range states {
		m.db.Delete(s, orm.Eq(user.OAuthState_.State, s.State))
	}
	return nil
}

type GoogleProvider struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
	config       *oauth2.Config
}

func (p *GoogleProvider) Name() string {
	return "google"
}

func (p *GoogleProvider) ensureConfig() {
	if p.config == nil {
		p.config = &oauth2.Config{
			ClientID:     p.ClientID,
			ClientSecret: p.ClientSecret,
			RedirectURL:  p.RedirectURL,
			Scopes:       []string{"https://www.googleapis.com/auth/userinfo.email", "https://www.googleapis.com/auth/userinfo.profile"},
			Endpoint:     google.Endpoint,
		}
	}
}

func (p *GoogleProvider) AuthCodeURL(state string) string {
	p.ensureConfig()
	return p.config.AuthCodeURL(state)
}

func (p *GoogleProvider) ExchangeCode(ctx context.Context, code string) (*oauth2.Token, error) {
	p.ensureConfig()
	return p.config.Exchange(ctx, code)
}

func (p *GoogleProvider) GetUserInfo(ctx context.Context, token *oauth2.Token) (user.OAuthUserInfo, error) {
	client := p.config.Client(ctx, token)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		return user.OAuthUserInfo{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return user.OAuthUserInfo{}, user.ErrInvalidCredentials
	}

	var data struct {
		ID    string `json:"id"`
		Email string `json:"email"`
		Name  string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return user.OAuthUserInfo{}, err
	}

	return user.OAuthUserInfo{
		ID:    data.ID,
		Email: data.Email,
		Name:  data.Name,
	}, nil
}

type MicrosoftProvider struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
	config       *oauth2.Config
}

func (p *MicrosoftProvider) Name() string {
	return "microsoft"
}

func (p *MicrosoftProvider) ensureConfig() {
	if p.config == nil {
		p.config = &oauth2.Config{
			ClientID:     p.ClientID,
			ClientSecret: p.ClientSecret,
			RedirectURL:  p.RedirectURL,
			Scopes:       []string{"User.Read"},
			Endpoint:     microsoft.AzureADEndpoint("common"),
		}
	}
}

func (p *MicrosoftProvider) AuthCodeURL(state string) string {
	p.ensureConfig()
	return p.config.AuthCodeURL(state)
}

func (p *MicrosoftProvider) ExchangeCode(ctx context.Context, code string) (*oauth2.Token, error) {
	p.ensureConfig()
	return p.config.Exchange(ctx, code)
}

func (p *MicrosoftProvider) GetUserInfo(ctx context.Context, token *oauth2.Token) (user.OAuthUserInfo, error) {
	client := p.config.Client(ctx, token)
	resp, err := client.Get("https://graph.microsoft.com/v1.0/me")
	if err != nil {
		return user.OAuthUserInfo{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return user.OAuthUserInfo{}, user.ErrInvalidCredentials
	}

	var data struct {
		ID                string `json:"id"`
		Email             string `json:"mail"`
		UserPrincipalName string `json:"userPrincipalName"`
		Name              string `json:"displayName"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return user.OAuthUserInfo{}, err
	}

	email := data.Email
	if email == "" {
		email = data.UserPrincipalName
	}

	return user.OAuthUserInfo{
		ID:    data.ID,
		Email: email,
		Name:  data.Name,
	}, nil
}
