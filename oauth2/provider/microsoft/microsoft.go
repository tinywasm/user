package microsoft

import (
	"github.com/tinywasm/fetch"
	"github.com/tinywasm/json"
	"github.com/tinywasm/model"
	"github.com/tinywasm/user"
	"github.com/tinywasm/user/oauth2/provider/google"
)

const (
	msAuthURL  = "https://login.microsoftonline.com/common/oauth2/v2.0/authorize"
	msTokenURL = "https://login.microsoftonline.com/common/oauth2/v2.0/token"
)

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
	return google.AuthCodeURLHelper(p.config(), state)
}

func (p *MicrosoftProvider) ExchangeCode(code string) (user.OAuthToken, error) {
	return google.ExchangeCodeHelper(p.config(), code)
}

type msData struct {
	ID                string
	Email             string
	UserPrincipalName string
	Name              string
}

func (d *msData) EncodeFields(w model.FieldWriter) {}
func (d *msData) IsNil() bool                      { return false }
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
