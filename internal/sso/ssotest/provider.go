package ssotest

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	jose "github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
)

const (
	ClientID     = "gojellyfin"
	ClientSecret = "not-a-real-secret"
	keyID        = "ssotest"
	keyBits      = 2048
)

type Claims map[string]any

type grant struct {
	nonce       string
	challenge   string
	redirectURI string
	claims      Claims
}

type Provider struct {
	URL string

	server *httptest.Server
	key    *rsa.PrivateKey

	mu       sync.Mutex
	next     Claims
	nonce    string
	userinfo Claims
	grants   map[string]grant
	tokens   map[string]Claims
	redirect string
}

func New(t *testing.T) *Provider {
	t.Helper()

	p, err := Start()
	if err != nil {
		t.Fatalf("failed to start the provider: %v", err)
	}
	t.Cleanup(p.Close)

	return p
}

func Start() (*Provider, error) {
	key, err := rsa.GenerateKey(rand.Reader, keyBits)
	if err != nil {
		return nil, err
	}

	p := &Provider{key: key, grants: map[string]grant{}, tokens: map[string]Claims{}}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /.well-known/openid-configuration", p.discovery)
	mux.HandleFunc("GET /keys", p.keys)
	mux.HandleFunc("GET /authorize", p.authorize)
	mux.HandleFunc("POST /token", p.token)
	mux.HandleFunc("GET /userinfo", p.info)
	p.server = httptest.NewServer(mux)
	p.URL = p.server.URL

	return p, nil
}

func (p *Provider) Close() {
	p.server.Close()
}

func (p *Provider) Register(redirectURI string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.redirect = redirectURI
}

func (p *Provider) SignIn(claims Claims) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.next = claims
}

func (p *Provider) UserInfo(claims Claims) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.userinfo = claims
}

func (p *Provider) ForgeNonce(nonce string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.nonce = nonce
}

func (p *Provider) discovery(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, map[string]any{
		"issuer":                                p.URL,
		"authorization_endpoint":                p.URL + "/authorize",
		"token_endpoint":                        p.URL + "/token",
		"userinfo_endpoint":                     p.URL + "/userinfo",
		"jwks_uri":                              p.URL + "/keys",
		"response_types_supported":              []string{"code"},
		"subject_types_supported":               []string{"public"},
		"id_token_signing_alg_values_supported": []string{"RS256"},
		"code_challenge_methods_supported":      []string{"S256"},
	})
}

func (p *Provider) keys(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{
		Key:       &p.key.PublicKey,
		KeyID:     keyID,
		Algorithm: string(jose.RS256),
		Use:       "sig",
	}}})
}

func (p *Provider) authorize(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	p.mu.Lock()
	registered := p.redirect
	p.mu.Unlock()
	if registered == "" || query.Get("redirect_uri") != registered {
		http.Error(w, "redirect_uri is not registered", http.StatusBadRequest)
		return
	}
	redirect, err := url.Parse(registered)
	if err != nil || query.Get("client_id") != ClientID || query.Get("response_type") != "code" || query.Get("code_challenge_method") != "S256" {
		http.Error(w, "bad authorization request", http.StatusBadRequest)
		return
	}

	p.mu.Lock()
	claims := p.next
	nonce := query.Get("nonce")
	if p.nonce != "" {
		nonce = p.nonce
	}
	code := randomHex()
	p.grants[code] = grant{nonce: nonce, challenge: query.Get("code_challenge"), redirectURI: redirect.String(), claims: claims}
	p.mu.Unlock()

	answer := redirect.Query()
	if claims == nil {
		answer.Set("error", "access_denied")
		answer.Set("error_description", "the user declined")
	} else {
		answer.Set("code", code)
	}
	answer.Set("state", query.Get("state"))
	redirect.RawQuery = answer.Encode()
	http.Redirect(w, r, redirect.String(), http.StatusFound)
}

func (p *Provider) token(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	id, secret, ok := r.BasicAuth()
	if !ok {
		id, secret = r.PostForm.Get("client_id"), r.PostForm.Get("client_secret")
	}
	if id != ClientID || secret != ClientSecret {
		http.Error(w, `{"error":"invalid_client"}`, http.StatusUnauthorized)
		return
	}

	p.mu.Lock()
	code := r.PostForm.Get("code")
	granted, ok := p.grants[code]
	delete(p.grants, code)
	p.mu.Unlock()

	sum := sha256.Sum256([]byte(r.PostForm.Get("code_verifier")))
	if !ok || granted.redirectURI != r.PostForm.Get("redirect_uri") || base64.RawURLEncoding.EncodeToString(sum[:]) != granted.challenge {
		http.Error(w, `{"error":"invalid_grant"}`, http.StatusBadRequest)
		return
	}

	idToken, err := p.sign(granted)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	access := randomHex()
	p.mu.Lock()
	p.tokens[access] = granted.claims
	p.mu.Unlock()

	writeJSON(w, map[string]any{
		"access_token": access,
		"token_type":   "Bearer",
		"expires_in":   int(time.Hour.Seconds()),
		"id_token":     idToken,
	})
}

func (p *Provider) info(w http.ResponseWriter, r *http.Request) {
	p.mu.Lock()
	claims, ok := p.tokens[bearer(r)]
	extra := p.userinfo
	p.mu.Unlock()
	if !ok {
		http.Error(w, "unknown token", http.StatusUnauthorized)
		return
	}

	answer := map[string]any{"sub": claims["sub"]}
	for key, value := range extra {
		answer[key] = value
	}
	writeJSON(w, answer)
}

func (p *Provider) sign(granted grant) (string, error) {
	signer, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.RS256, Key: p.key},
		(&jose.SignerOptions{}).WithType("JWT").WithHeader("kid", keyID),
	)
	if err != nil {
		return "", err
	}

	now := time.Now()
	standard := jwt.Claims{
		Issuer:   p.URL,
		Audience: jwt.Audience{ClientID},
		IssuedAt: jwt.NewNumericDate(now),
		Expiry:   jwt.NewNumericDate(now.Add(time.Hour)),
	}

	return jwt.Signed(signer).Claims(standard).Claims(map[string]any(granted.claims)).Claims(map[string]any{"nonce": granted.nonce}).Serialize()
}

func bearer(r *http.Request) string {
	const prefix = "Bearer "
	header := r.Header.Get("Authorization")
	if len(header) <= len(prefix) {
		return ""
	}

	return header[len(prefix):]
}

func randomHex() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)

	return hex.EncodeToString(b)
}

func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(value)
}
