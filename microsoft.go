package user

import (
	"context"
	"encoding/json"
	"net/http"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/microsoft"
)

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

func (p *MicrosoftProvider) GetUserInfo(ctx context.Context, token *oauth2.Token) (OAuthUserInfo, error) {
	client := p.config.Client(ctx, token)
	resp, err := client.Get("https://graph.microsoft.com/v1.0/me")
	if err != nil {
		return OAuthUserInfo{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return OAuthUserInfo{}, ErrInvalidCredentials
	}

	var data struct {
		ID                string `json:"id"`
		Email             string `json:"mail"`
		UserPrincipalName string `json:"userPrincipalName"`
		Name              string `json:"displayName"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return OAuthUserInfo{}, err
	}

	email := data.Email
	if email == "" {
		email = data.UserPrincipalName
	}

	return OAuthUserInfo{
		ID:    data.ID,
		Email: email,
		Name:  data.Name,
	}, nil
}
