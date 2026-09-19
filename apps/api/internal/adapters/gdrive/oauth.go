package gdrive

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"

	"heatseeker/api/internal/domain"
)

const revokeURL = "https://oauth2.googleapis.com/revoke"

// OAuth implements domain.DriveAuthorizer with a Google "Web application"
// OAuth client. The publishing account grants full Drive access: files are
// created in an existing folder the app did not create, which drive.file
// does not allow.
type OAuth struct {
	cfg *oauth2.Config
}

// NewOAuth configures the flow; redirectURL must be registered in the client.
func NewOAuth(clientID, clientSecret, redirectURL string) *OAuth {
	return &OAuth{cfg: &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Endpoint:     google.Endpoint,
		Scopes:       []string{drive.DriveScope, "openid", "email"},
	}}
}

// AuthURL implements domain.DriveAuthorizer. Offline access with a forced
// consent screen makes Google return a refresh token every time.
func (o *OAuth) AuthURL(state string) string {
	return o.cfg.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.ApprovalForce)
}

// Exchange implements domain.DriveAuthorizer.
func (o *OAuth) Exchange(ctx context.Context, code string) (*domain.DriveGrant, error) {
	tok, err := o.cfg.Exchange(ctx, code)
	if err != nil {
		if terr := asTokenErr(err, true); terr != nil {
			return nil, terr
		}
		return nil, fmt.Errorf("google sign-in: %w", err)
	}
	scopes, _ := tok.Extra("scope").(string)
	// Google lets the user untick Drive access on the consent screen.
	if !hasScope(scopes, drive.DriveScope) {
		return nil, domain.WithCode(domain.CodeDriveScopeMissing,
			domain.Invalid("scope", "access to Google Drive was not granted: tick it on the consent screen"))
	}
	if tok.RefreshToken == "" {
		return nil, errors.New("google sign-in: no refresh token in the response")
	}
	idToken, _ := tok.Extra("id_token").(string)
	email, err := emailFromIDToken(idToken)
	if err != nil {
		return nil, err
	}
	return &domain.DriveGrant{RefreshToken: tok.RefreshToken, Email: email, Scopes: scopes}, nil
}

// Client implements domain.DriveAuthorizer.
func (o *OAuth) Client(ctx context.Context, refreshToken string) (domain.DriveClient, error) {
	ts := o.cfg.TokenSource(ctx, &oauth2.Token{RefreshToken: refreshToken})
	svc, err := drive.NewService(ctx, option.WithTokenSource(ts))
	if err != nil {
		return nil, fmt.Errorf("drive client: %w", err)
	}
	return &Client{svc: svc}, nil
}

// Revoke implements domain.DriveAuthorizer.
func (o *OAuth) Revoke(ctx context.Context, refreshToken string) error {
	body := url.Values{"token": {refreshToken}}.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, revokeURL, strings.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("revoke google token: %w", err)
	}
	defer func() { _ = res.Body.Close() }()
	// 400 invalid_token: already revoked, which is what we want.
	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusBadRequest {
		return fmt.Errorf("revoke google token: status %d", res.StatusCode)
	}
	return nil
}

func hasScope(scopes, want string) bool {
	for _, s := range strings.Fields(scopes) {
		if s == want {
			return true
		}
	}
	return false
}

// emailFromIDToken reads the e-mail claim. The token comes straight from
// Google's token endpoint over TLS, so its signature needs no check here
// (OpenID Connect Core §3.1.3.7).
func emailFromIDToken(idToken string) (string, error) {
	parts := strings.Split(idToken, ".")
	if len(parts) != 3 {
		return "", errors.New("google sign-in: no id_token (is the email scope granted?)")
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", fmt.Errorf("google sign-in: id_token: %w", err)
	}
	var claims struct {
		Email string `json:"email"`
	}
	if err := json.Unmarshal(raw, &claims); err != nil || claims.Email == "" {
		return "", errors.New("google sign-in: id_token has no e-mail")
	}
	return claims.Email, nil
}

// asTokenErr converts an OAuth token endpoint failure (an expired or reused
// code on sign-in, a revoked refresh token afterwards) and returns nil for any
// other error.
func asTokenErr(err error, signIn bool) error {
	var rerr *oauth2.RetrieveError
	if !errors.As(err, &rerr) {
		return nil
	}
	switch {
	case rerr.ErrorCode == "invalid_grant" && signIn:
		return domain.Invalid("code", "the Google sign-in has expired or was already used; start again")
	case rerr.ErrorCode == "invalid_grant":
		return domain.WithCode(domain.CodeDrivePublisherRevoked,
			fmt.Errorf("%w: Google rejected the publishing account's token (access revoked or password changed)", domain.ErrUnavailable))
	}
	return fmt.Errorf("google token: %s: %s", rerr.ErrorCode, rerr.ErrorDescription)
}

var _ domain.DriveAuthorizer = (*OAuth)(nil)
