package user

import (
	"context"
	"encoding/json"
	"net/http"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

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

func (p *GoogleProvider) GetUserInfo(ctx context.Context, token *oauth2.Token) (OAuthUserInfo, error) {
	client := p.config.Client(ctx, token)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		return OAuthUserInfo{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return OAuthUserInfo{}, ErrInvalidCredentials
	}

	var data struct {
		ID    string `json:"id"`
		Email string `json:"email"`
		Name  string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return OAuthUserInfo{}, err
	}

	return OAuthUserInfo{
		ID:    data.ID,
		Email: data.Email,
		Name:  data.Name,
	}, nil
}
