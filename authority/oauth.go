package authority

import (
	"github.com/tinywasm/fetch"
	"github.com/tinywasm/fmt"
	"github.com/tinywasm/json"
	"github.com/tinywasm/model"
	"github.com/tinywasm/orm"
	"github.com/tinywasm/router"
	"github.com/tinywasm/time"
	"github.com/tinywasm/unixid"
	"github.com/tinywasm/user"
)

const (
	googleAuthURL  = "https://accounts.google.com/o/oauth2/auth"
	googleTokenURL = "https://oauth2.googleapis.com/token"

	msAuthURL  = "https://login.microsoftonline.com/common/oauth2/v2.0/authorize"
	msTokenURL = "https://login.microsoftonline.com/common/oauth2/v2.0/token"
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

	now := time.Now() / 1e9
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
	path := ctx.Path()
	if fmt.Contains(path, "?") {
		query := fmt.Split(path, "?")[1]
		parts := fmt.Split(query, "&")
		for _, part := range parts {
			kv := fmt.Split(part, "=")
			if len(kv) == 2 {
				if kv[0] == "state" {
					state = kv[1]
				} else if kv[0] == "code" {
					code = kv[1]
				}
			}
		}
	}

	if err := consumeState(m.db, state, providerName); err != nil {
		return user.User{}, false, user.ErrInvalidOAuthState
	}

	p := m.getProvider(providerName)
	if p == nil {
		return user.User{}, false, user.ErrProviderNotFound
	}

	token, err := p.ExchangeCode(code)
	if err != nil {
		return user.User{}, false, err
	}

	info, err := p.GetUserInfo(token)
	if err != nil {
		return user.User{}, false, err
	}

	identity, err := getIdentityByProvider(m.db, providerName, info.ID)
	if err == nil {
		u, err := getUser(m.db, m.ucache, identity.UserId)
		return u, false, err
	}

	u, err := getUserByEmail(m.db, m.ucache, info.Email)
	if err == nil {
		_ = createIdentity(m.db, u.Id, providerName, info.ID, info.Email)
		return u, false, nil
	}

	u, err = createUser(m.db, info.Email, info.Name, "")
	if err != nil {
		return user.User{}, false, err
	}
	_ = createIdentity(m.db, u.Id, providerName, info.ID, info.Email)
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

	if stateObj.ExpiresAt < time.Now()/1e9 {
		return user.ErrInvalidOAuthState
	}

	return nil
}

func (m *Module) PurgeExpiredOAuthStates() error {
	qb := m.db.Query(&user.OAuthState{}).Where(user.OAuthState_.ExpiresAt).Lt(time.Now() / 1e9)
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
}

func (p *GoogleProvider) Name() string {
	return "google"
}

func (p *GoogleProvider) config() user.OAuthConfig {
	return user.OAuthConfig{
		ClientID:     p.ClientID,
		ClientSecret: p.ClientSecret,
		RedirectURL:  p.RedirectURL,
		Scopes:       []string{"https://www.googleapis.com/auth/userinfo.email", "https://www.googleapis.com/auth/userinfo.profile"},
		AuthURL:      googleAuthURL,
		TokenURL:     googleTokenURL,
	}
}

func (p *GoogleProvider) AuthCodeURL(state string) string {
	return authCodeURL(p.config(), state)
}

func (p *GoogleProvider) ExchangeCode(code string) (user.OAuthToken, error) {
	return exchangeCode(p.config(), code)
}

type googleData struct {
	ID    string
	Email string
	Name  string
}

func (d *googleData) EncodeFields(w model.FieldWriter) {}
func (d *googleData) IsNil() bool                     { return false }
func (d *googleData) DecodeFields(r model.FieldReader) {
	d.ID, _ = r.String("id")
	d.Email, _ = r.String("email")
	d.Name, _ = r.String("name")
}

func (p *GoogleProvider) GetUserInfo(token user.OAuthToken) (user.OAuthUserInfo, error) {
	var res user.OAuthUserInfo
	var errOut error
	done := make(chan bool)

	fetch.Get("https://www.googleapis.com/oauth2/v2/userinfo").
		Header("Authorization", "Bearer "+token.AccessToken).
		Send(func(resp *fetch.Response, err error) {
			defer func() { done <- true }()
			if err != nil {
				errOut = err
				return
			}
			if resp.Status != 200 {
				errOut = user.ErrInvalidCredentials
				return
			}
			var data googleData
			if err := json.Decode(resp.Text(), &data); err != nil {
				errOut = err
				return
			}
			res = user.OAuthUserInfo{
				ID:    data.ID,
				Email: data.Email,
				Name:  data.Name,
			}
		})

	<-done
	return res, errOut
}

type MicrosoftProvider struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
}

func (p *MicrosoftProvider) Name() string {
	return "microsoft"
}

func (p *MicrosoftProvider) config() user.OAuthConfig {
	return user.OAuthConfig{
		ClientID:     p.ClientID,
		ClientSecret: p.ClientSecret,
		RedirectURL:  p.RedirectURL,
		Scopes:       []string{"User.Read"},
		AuthURL:      msAuthURL,
		TokenURL:     msTokenURL,
	}
}

func (p *MicrosoftProvider) AuthCodeURL(state string) string {
	return authCodeURL(p.config(), state)
}

func (p *MicrosoftProvider) ExchangeCode(code string) (user.OAuthToken, error) {
	return exchangeCode(p.config(), code)
}

type msData struct {
	ID                string
	Email             string
	UserPrincipalName string
	Name              string
}

func (d *msData) EncodeFields(w model.FieldWriter) {}
func (d *msData) IsNil() bool                     { return false }
func (d *msData) DecodeFields(r model.FieldReader) {
	d.ID, _ = r.String("id")
	d.Email, _ = r.String("mail")
	d.UserPrincipalName, _ = r.String("userPrincipalName")
	d.Name, _ = r.String("displayName")
}

func (p *MicrosoftProvider) GetUserInfo(token user.OAuthToken) (user.OAuthUserInfo, error) {
	var res user.OAuthUserInfo
	var errOut error
	done := make(chan bool)

	fetch.Get("https://graph.microsoft.com/v1.0/me").
		Header("Authorization", "Bearer "+token.AccessToken).
		Send(func(resp *fetch.Response, err error) {
			defer func() { done <- true }()
			if err != nil {
				errOut = err
				return
			}
			if resp.Status != 200 {
				errOut = user.ErrInvalidCredentials
				return
			}
			var data msData
			if err := json.Decode(resp.Text(), &data); err != nil {
				errOut = err
				return
			}
			email := data.Email
			if email == "" {
				email = data.UserPrincipalName
			}
			res = user.OAuthUserInfo{
				ID:    data.ID,
				Email: email,
				Name:  data.Name,
			}
		})

	<-done
	return res, errOut
}

func authCodeURL(cfg user.OAuthConfig, state string) string {
	res := cfg.AuthURL + "?response_type=code"
	res += "&client_id=" + queryEscape(cfg.ClientID)
	res += "&redirect_uri=" + queryEscape(cfg.RedirectURL)
	res += "&state=" + queryEscape(state)
	if len(cfg.Scopes) > 0 {
		res += "&scope="
		for i, s := range cfg.Scopes {
			if i > 0 {
				res += "+"
			}
			res += queryEscape(s)
		}
	}
	return res
}

func exchangeCode(cfg user.OAuthConfig, code string) (user.OAuthToken, error) {
	body := "grant_type=authorization_code"
	body += "&code=" + queryEscape(code)
	body += "&client_id=" + queryEscape(cfg.ClientID)
	body += "&client_secret=" + queryEscape(cfg.ClientSecret)
	body += "&redirect_uri=" + queryEscape(cfg.RedirectURL)

	var res user.OAuthToken
	var errOut error
	done := make(chan bool)

	fetch.Post(cfg.TokenURL).
		Header("Content-Type", "application/x-www-form-urlencoded").
		Body([]byte(body)).
		Send(func(resp *fetch.Response, err error) {
			defer func() { done <- true }()
			if err != nil {
				errOut = err
				return
			}
			if resp.Status != 200 {
				errOut = user.ErrInvalidCredentials
				return
			}
			if err := json.Decode(resp.Text(), &res); err != nil {
				errOut = err
				return
			}
		})

	<-done
	return res, errOut
}

func queryEscape(s string) string {
	res := ""
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_' || c == '.' || c == '~' {
			res += string(c)
		} else if c == ' ' {
			res += "+"
		} else {
			res += "%" + hexDigit(c>>4) + hexDigit(c&0x0F)
		}
	}
	return res
}

func hexDigit(c byte) string {
	if c < 10 {
		return string('0' + c)
	}
	return string('A' + (c - 10))
}
