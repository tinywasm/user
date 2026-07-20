package google

import (
	"github.com/tinywasm/fetch"
	"github.com/tinywasm/json"
	"github.com/tinywasm/model"
	"github.com/tinywasm/user"
)

const (
	googleAuthURL  = "https://accounts.google.com/o/oauth2/auth"
	googleTokenURL = "https://oauth2.googleapis.com/token"
)

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
	return AuthCodeURLHelper(p.config(), state)
}

func (p *GoogleProvider) ExchangeCode(code string) (user.OAuthToken, error) {
	return ExchangeCodeHelper(p.config(), code)
}

type googleData struct {
	ID    string
	Email string
	Name  string
}

func (d *googleData) EncodeFields(w model.FieldWriter) {}
func (d *googleData) IsNil() bool                      { return false }
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

func AuthCodeURLHelper(cfg user.OAuthConfig, state string) string {
	res := cfg.AuthURL + "?response_type=code"
	res += "&client_id=" + QueryEscapeHelper(cfg.ClientID)
	res += "&redirect_uri=" + QueryEscapeHelper(cfg.RedirectURL)
	res += "&state=" + QueryEscapeHelper(state)
	if len(cfg.Scopes) > 0 {
		res += "&scope="
		for i, s := range cfg.Scopes {
			if i > 0 {
				res += "+"
			}
			res += QueryEscapeHelper(s)
		}
	}
	return res
}

func ExchangeCodeHelper(cfg user.OAuthConfig, code string) (user.OAuthToken, error) {
	body := "grant_type=authorization_code"
	body += "&code=" + QueryEscapeHelper(code)
	body += "&client_id=" + QueryEscapeHelper(cfg.ClientID)
	body += "&client_secret=" + QueryEscapeHelper(cfg.ClientSecret)
	body += "&redirect_uri=" + QueryEscapeHelper(cfg.RedirectURL)

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

func QueryEscapeHelper(s string) string {
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
