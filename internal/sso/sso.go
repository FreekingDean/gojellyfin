package sso

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	gooidc "github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"github.com/FreekingDean/gojellyfin/internal/auth"
	"github.com/FreekingDean/gojellyfin/internal/env"
	"github.com/FreekingDean/gojellyfin/internal/observability/tracing"
	"github.com/FreekingDean/gojellyfin/internal/users"
)

const (
	CallbackPath = "/sso/oidc/callback"

	defaultScopes        = "openid profile email"
	defaultUsernameClaim = "preferred_username"
	defaultGroupsClaim   = "groups"

	providerTimeout = 15 * time.Second
)

var (
	ErrDisabled     = errors.New("single sign-on is not configured")
	ErrStateInvalid = errors.New("the sign-in did not start here or has expired")
	ErrNotAllowed   = errors.New("the identity provider vouched for this account but it is not in the group allowed to sign in")
	ErrNoUsername   = errors.New("the identity provider answered no username for this account")
)

type Flow struct {
	State    string `json:"state"`
	Nonce    string `json:"nonce"`
	Verifier string `json:"verifier"`
}

type Service struct {
	config      env.OIDC
	redirectURL string
	users       *users.Service

	client *http.Client

	mu       sync.Mutex
	provider *gooidc.Provider
}

func New(config env.Config, users *users.Service) *Service {
	return &Service{
		config:      config.OIDC,
		redirectURL: strings.TrimSuffix(config.PublishedServerURL, "/") + CallbackPath,
		users:       users,
		client:      &http.Client{Timeout: providerTimeout, Transport: tracing.Transport(nil)},
	}
}

func (s *Service) bounded(ctx context.Context) context.Context {
	return gooidc.ClientContext(ctx, s.client)
}

func (s *Service) Enabled() bool {
	return s.config.IssuerURL != ""
}

func (s *Service) Issuer() string {
	return s.config.IssuerURL
}

func (s *Service) Discover(ctx context.Context) error {
	_, err := s.discover(s.bounded(ctx))

	return err
}

func (s *Service) Begin(ctx context.Context) (string, Flow, error) {
	ctx = s.bounded(ctx)
	oauth, _, err := s.oauth(ctx)
	if err != nil {
		return "", Flow{}, err
	}

	state, err := auth.NewToken()
	if err != nil {
		return "", Flow{}, err
	}
	nonce, err := auth.NewToken()
	if err != nil {
		return "", Flow{}, err
	}
	flow := Flow{State: state, Nonce: nonce, Verifier: oauth2.GenerateVerifier()}

	return oauth.AuthCodeURL(flow.State, gooidc.Nonce(flow.Nonce), oauth2.S256ChallengeOption(flow.Verifier)), flow, nil
}

func (s *Service) Finish(ctx context.Context, flow Flow, state, code string) (*users.User, error) {
	ctx = s.bounded(ctx)
	if flow.State == "" || state != flow.State {
		return nil, ErrStateInvalid
	}

	oauth, provider, err := s.oauth(ctx)
	if err != nil {
		return nil, err
	}

	token, err := oauth.Exchange(ctx, code, oauth2.VerifierOption(flow.Verifier))
	if err != nil {
		return nil, fmt.Errorf("the identity provider refused the code: %w", err)
	}
	raw, ok := token.Extra("id_token").(string)
	if !ok {
		return nil, errors.New("the identity provider answered no id_token")
	}
	idToken, err := provider.Verifier(&gooidc.Config{ClientID: s.config.ClientID}).Verify(ctx, raw)
	if err != nil {
		return nil, fmt.Errorf("the id_token did not verify: %w", err)
	}
	if idToken.Nonce != flow.Nonce {
		return nil, ErrStateInvalid
	}

	claims, err := s.claims(ctx, provider, oauth.TokenSource(ctx, token), idToken)
	if err != nil {
		return nil, err
	}

	identity, err := s.identity(idToken, claims)
	if err != nil {
		return nil, err
	}

	return s.users.SignInIdentity(ctx, identity, s.config.LinkExistingUsers)
}

func (s *Service) claims(ctx context.Context, provider *gooidc.Provider, source oauth2.TokenSource, idToken *gooidc.IDToken) (map[string]any, error) {
	claims := map[string]any{}
	if err := idToken.Claims(&claims); err != nil {
		return nil, fmt.Errorf("the id_token claims could not be read: %w", err)
	}

	_, hasUsername := claims[s.usernameClaim()]
	_, hasGroups := claims[s.groupsClaim()]
	if hasUsername && (hasGroups || !s.readsGroups()) {
		return claims, nil
	}

	info, err := provider.UserInfo(ctx, source)
	if err != nil {
		return claims, nil
	}
	if info.Subject != idToken.Subject {
		return nil, errors.New("the userinfo endpoint answered for a different subject than the id_token")
	}
	extra := map[string]any{}
	if err := info.Claims(&extra); err != nil {
		return claims, nil
	}
	for key, value := range extra {
		if _, present := claims[key]; !present {
			claims[key] = value
		}
	}

	return claims, nil
}

func (s *Service) identity(idToken *gooidc.IDToken, claims map[string]any) (users.Identity, error) {
	username, _ := claims[s.usernameClaim()].(string)
	username = strings.TrimSpace(username)
	if username == "" {
		return users.Identity{}, ErrNoUsername
	}

	groups := stringsClaim(claims[s.groupsClaim()])
	if s.config.AllowedGroup != "" && !slices.Contains(groups, s.config.AllowedGroup) {
		return users.Identity{}, ErrNotAllowed
	}

	identity := users.Identity{
		Issuer:   idToken.Issuer,
		Subject:  idToken.Subject,
		Username: username,
	}
	if s.config.AdminGroup != "" {
		administrator := slices.Contains(groups, s.config.AdminGroup)
		identity.Administrator = &administrator
	}

	return identity, nil
}

func (s *Service) oauth(ctx context.Context) (*oauth2.Config, *gooidc.Provider, error) {
	provider, err := s.discover(ctx)
	if err != nil {
		return nil, nil, err
	}

	return &oauth2.Config{
		ClientID:     s.config.ClientID,
		ClientSecret: s.config.ClientSecret,
		Endpoint:     provider.Endpoint(),
		RedirectURL:  s.redirectURL,
		Scopes:       strings.Fields(orDefault(s.config.Scopes, defaultScopes)),
	}, provider, nil
}

func (s *Service) discover(ctx context.Context) (*gooidc.Provider, error) {
	if !s.Enabled() {
		return nil, ErrDisabled
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.provider != nil {
		return s.provider, nil
	}

	provider, err := gooidc.NewProvider(ctx, s.config.IssuerURL)
	if err != nil {
		return nil, fmt.Errorf("the identity provider could not be discovered: %w", err)
	}
	s.provider = provider

	return provider, nil
}

func (s *Service) readsGroups() bool {
	return s.config.AllowedGroup != "" || s.config.AdminGroup != ""
}

func (s *Service) usernameClaim() string {
	return orDefault(s.config.UsernameClaim, defaultUsernameClaim)
}

func (s *Service) groupsClaim() string {
	return orDefault(s.config.GroupsClaim, defaultGroupsClaim)
}

func stringsClaim(value any) []string {
	switch typed := value.(type) {
	case string:
		return []string{typed}
	case []any:
		values := make([]string, 0, len(typed))
		for _, entry := range typed {
			if text, ok := entry.(string); ok {
				values = append(values, text)
			}
		}

		return values
	default:
		return nil
	}
}

func orDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}

	return value
}
