package sso

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/activity"
	"github.com/FreekingDean/gojellyfin/internal/config"
	"github.com/FreekingDean/gojellyfin/internal/env"
	"github.com/FreekingDean/gojellyfin/internal/sessions"
	domain "github.com/FreekingDean/gojellyfin/internal/sso"
	"github.com/FreekingDean/gojellyfin/internal/sso/ssotest"
	"github.com/FreekingDean/gojellyfin/internal/store"
	entrymodel "github.com/FreekingDean/gojellyfin/internal/store/activitylogentry"
	devicemodel "github.com/FreekingDean/gojellyfin/internal/store/device"
	sessionmodel "github.com/FreekingDean/gojellyfin/internal/store/session"
	usermodel "github.com/FreekingDean/gojellyfin/internal/store/user"
	"github.com/FreekingDean/gojellyfin/internal/users"
)

type fixture struct {
	provider *ssotest.Provider
	server   *httptest.Server
	client   *store.Client
	sessions *sessions.Service
	users    *users.Service
}

func newFixture(t *testing.T, configure func(*env.OIDC)) *fixture {
	t.Helper()

	base, err := env.Load()
	if err != nil {
		t.Fatalf("failed to read the environment: %v", err)
	}
	connection, err := store.NewStore(base)
	if err != nil {
		t.Fatalf("failed to open the database: %v", err)
	}
	if err := connection.Start(); err != nil {
		t.Fatalf("failed to reach the database, set DATABASE_URL: %v", err)
	}
	t.Cleanup(func() {
		if err := connection.Stop(); err != nil {
			t.Errorf("failed to close the database: %v", err)
		}
	})

	f := &fixture{provider: ssotest.New(t), client: connection.Client()}
	f.users = users.New(f.client)
	f.sessions = sessions.New(f.client, activity.New(f.client))

	var handler http.Handler
	f.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { handler.ServeHTTP(w, r) }))
	t.Cleanup(f.server.Close)

	cfg := base
	cfg.PublishedServerURL = f.server.URL
	f.provider.Register(f.server.URL + domain.CallbackPath)
	cfg.OIDC = env.OIDC{IssuerURL: f.provider.URL, ClientID: ssotest.ClientID, ClientSecret: ssotest.ClientSecret}
	if configure != nil {
		configure(&cfg.OIDC)
	}
	handler = http.HandlerFunc(New(cfg, domain.New(cfg, f.users), f.sessions, f.users).Serve)

	return f
}

func (f *fixture) username(t *testing.T) string {
	t.Helper()

	name := "sso-" + uuid.NewString()
	t.Cleanup(func() { f.forget(t, name) })

	return name
}

func (f *fixture) forget(t *testing.T, username string) {
	t.Helper()
	ctx := context.Background()

	user, err := f.client.User.Query().Where(usermodel.Username(username)).Only(ctx)
	if store.IsNotFound(err) {
		return
	}
	if err != nil {
		t.Errorf("failed to find the user: %v", err)
		return
	}
	devices, err := f.client.Session.Query().Where(sessionmodel.HasUserWith(usermodel.ID(user.ID))).QueryDevice().IDs(ctx)
	if err != nil {
		t.Errorf("failed to find the devices: %v", err)
	}
	if _, err := f.client.ActivityLogEntry.Delete().Where(entrymodel.HasUserWith(usermodel.ID(user.ID))).Exec(ctx); err != nil {
		t.Errorf("failed to delete the entries: %v", err)
	}
	if err := f.client.User.DeleteOne(user).Exec(ctx); err != nil {
		t.Errorf("failed to delete the user: %v", err)
	}
	if _, err := f.client.Device.Delete().Where(devicemodel.IDIn(devices...)).Exec(ctx); err != nil {
		t.Errorf("failed to delete the devices: %v", err)
	}
}

func (f *fixture) browser(t *testing.T) *http.Client {
	t.Helper()

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("failed to make a cookie jar: %v", err)
	}

	return &http.Client{Jar: jar}
}

func (f *fixture) get(t *testing.T, browser *http.Client, path string) (int, string) {
	t.Helper()

	response, err := browser.Get(f.server.URL + path)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	defer func() { _ = response.Body.Close() }()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("failed to read the body: %v", err)
	}

	return response.StatusCode, string(body)
}

func signedIn(t *testing.T, page string) signIn {
	t.Helper()

	const marker = "var signin = "
	start := strings.Index(page, marker)
	if start < 0 {
		t.Fatalf("page carries no sign-in:\n%s", page)
	}
	rest := page[start+len(marker):]
	var answer signIn
	if err := json.NewDecoder(strings.NewReader(rest)).Decode(&answer); err != nil {
		t.Fatalf("failed to read the sign-in out of the page: %v\n%s", err, rest)
	}

	return answer
}

func TestHandler_SignsInAndHandsJellyfinWebAToken(t *testing.T) {
	f := newFixture(t, nil)
	username := f.username(t)
	f.provider.SignIn(ssotest.Claims{"sub": "subject-" + username, "preferred_username": username})

	status, page := f.get(t, f.browser(t), StartPath)
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200\n%s", status, page)
	}

	answer := signedIn(t, page)
	if answer.Key != "jellyfin_credentials" || answer.ServerId != config.ServerID || answer.Target != "/web/" {
		t.Errorf("sign-in = %+v, want the jellyfin-web credentials key, this server and the web root", answer)
	}

	session, err := f.sessions.ByToken(context.Background(), answer.AccessToken)
	if err != nil {
		t.Fatalf("the handed token does not resolve: %v", err)
	}
	if session.User == nil || session.User.Username != username || session.UserID.String() != answer.UserId {
		t.Errorf("session user = %+v, want %s", session.User, username)
	}
	if session.Device == nil || !strings.HasPrefix(session.Device.ClientID, "sso-") {
		t.Errorf("device = %+v, want an sso device", session.Device)
	}

	user, err := f.users.UserByUsername(context.Background(), username)
	if err != nil {
		t.Fatalf("the user was not provisioned: %v", err)
	}
	if user.PasswordHash != "" || user.Policy == nil || user.Policy.IsAdministrator {
		t.Errorf("provisioned user = %+v, want no password and no administrator", user)
	}
}

func TestHandler_SecondSignInIsTheSameAccount(t *testing.T) {
	f := newFixture(t, nil)
	username := f.username(t)
	f.provider.SignIn(ssotest.Claims{"sub": "subject-" + username, "preferred_username": username})

	_, first := f.get(t, f.browser(t), StartPath)
	f.provider.SignIn(ssotest.Claims{"sub": "subject-" + username, "preferred_username": username + "-renamed"})
	_, second := f.get(t, f.browser(t), StartPath)

	if signedIn(t, first).UserId != signedIn(t, second).UserId {
		t.Error("the same subject signed in as two different users")
	}
}

func TestHandler_ReturnsToTheRouteTheLinkPageLeft(t *testing.T) {
	tests := []struct {
		name   string
		cookie string
		query  string
		want   string
	}{
		{name: "cookie", cookie: url.QueryEscape("/quickconnect?code=123456"), want: "/web/#/quickconnect?code=123456"},
		{name: "query", query: "?redirect=" + url.QueryEscape("/quickconnect"), want: "/web/#/quickconnect"},
		{name: "admin", query: "?redirect=" + url.QueryEscape("/admin"), want: "/admin"},
		{name: "admin route", query: "?redirect=" + url.QueryEscape("/admin/users"), want: "/admin/users"},
		{name: "not admin", query: "?redirect=" + url.QueryEscape("/administrators"), want: "/web/#/administrators"},
		{name: "request page", query: "?redirect=" + url.QueryEscape("/request"), want: "/request"},
		{name: "request page with a search", query: "?redirect=" + url.QueryEscape("/request?q=dune"), want: "/request?q=dune"},
		{name: "not the request page", query: "?redirect=" + url.QueryEscape("/requests"), want: "/web/#/requests"},
		{name: "protocol relative", cookie: url.QueryEscape("//evil.example"), want: "/web/"},
		{name: "backslash", cookie: url.QueryEscape("/\\evil.example"), want: "/web/"},
		{name: "absolute", query: "?redirect=" + url.QueryEscape("https://evil.example"), want: "/web/"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f := newFixture(t, nil)
			username := f.username(t)
			f.provider.SignIn(ssotest.Claims{"sub": "subject-" + username, "preferred_username": username})

			browser := f.browser(t)
			if test.cookie != "" {
				where, _ := url.Parse(f.server.URL + "/sso/oidc")
				browser.Jar.SetCookies(where, []*http.Cookie{{Name: returnCookie, Value: test.cookie, Path: "/sso/oidc"}})
			}

			status, page := f.get(t, browser, StartPath+test.query)
			if status != http.StatusOK {
				t.Fatalf("status = %d\n%s", status, page)
			}
			if got := signedIn(t, page).Target; got != test.want {
				t.Errorf("target = %q, want %q", got, test.want)
			}
		})
	}
}

func TestHandler_Refuses(t *testing.T) {
	tests := []struct {
		name      string
		configure func(*env.OIDC)
		claims    func(username string) ssotest.Claims
		forge     string
		existing  bool
		want      int
		says      string
	}{
		{
			name:   "a provider that declines",
			claims: func(string) ssotest.Claims { return nil },
			want:   http.StatusForbidden,
			says:   "access_denied",
		},
		{
			name:      "an account outside the allowed group",
			configure: func(c *env.OIDC) { c.AllowedGroup = "jellyfin" },
			claims: func(username string) ssotest.Claims {
				return ssotest.Claims{"sub": username, "preferred_username": username, "groups": []string{"other"}}
			},
			want: http.StatusForbidden,
			says: "not in the group",
		},
		{
			name: "an id_token for another sign-in",
			claims: func(username string) ssotest.Claims {
				return ssotest.Claims{"sub": username, "preferred_username": username}
			},
			forge: "replayed",
			want:  http.StatusBadRequest,
			says:  "expired",
		},
		{
			name: "a username a local account holds",
			claims: func(username string) ssotest.Claims {
				return ssotest.Claims{"sub": username, "preferred_username": username}
			},
			existing: true,
			want:     http.StatusForbidden,
			says:     "local account",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f := newFixture(t, test.configure)
			username := f.username(t)
			if test.existing {
				if _, err := f.users.CreateUser(context.Background(), username, "hash", false); err != nil {
					t.Fatalf("failed to create the local user: %v", err)
				}
			}
			f.provider.SignIn(test.claims(username))
			if test.forge != "" {
				f.provider.ForgeNonce(test.forge)
			}

			status, page := f.get(t, f.browser(t), StartPath)
			if status != test.want || !strings.Contains(page, test.says) {
				t.Errorf("status = %d, want %d saying %q\n%s", status, test.want, test.says, page)
			}
			if strings.Contains(page, "var signin") {
				t.Error("a refused sign-in handed out a token")
			}
		})
	}
}

func TestHandler_CallbackWithoutAStartIsRefused(t *testing.T) {
	f := newFixture(t, nil)

	status, page := f.get(t, f.browser(t), domain.CallbackPath+"?code=anything&state=anything")
	if status != http.StatusBadRequest || strings.Contains(page, "var signin") {
		t.Errorf("status = %d, want 400 with no token\n%s", status, page)
	}
}

func TestHandler_AdminGroupDecidesAdministrator(t *testing.T) {
	f := newFixture(t, func(c *env.OIDC) { c.AdminGroup = "jellyfin-admins" })
	username := f.username(t)

	f.provider.SignIn(ssotest.Claims{"sub": username, "preferred_username": username, "groups": []string{"jellyfin-admins"}})
	f.get(t, f.browser(t), StartPath)
	user, err := f.users.UserByUsername(context.Background(), username)
	if err != nil || !user.Policy.IsAdministrator {
		t.Fatalf("a member of the admin group is not an administrator: %v", err)
	}
	if user.Policy.EnableContentDeletion || user.Policy.EnableCollectionManagement || user.Policy.EnableRemoteControlOfOtherUsers {
		t.Errorf("policy = %+v, want only IsAdministrator to follow the group", user.Policy)
	}

	f.provider.SignIn(ssotest.Claims{"sub": username, "preferred_username": username, "groups": []string{}})
	f.get(t, f.browser(t), StartPath)
	if user, err := f.users.UserByUsername(context.Background(), username); err != nil || user.Policy.IsAdministrator {
		t.Fatalf("leaving the admin group left the account an administrator: %v", err)
	}
}

func TestHandler_LinksAnExistingAccountWhenAllowed(t *testing.T) {
	f := newFixture(t, func(c *env.OIDC) { c.LinkExistingUsers = true })
	username := f.username(t)
	local, err := f.users.CreateUser(context.Background(), username, "hash", false)
	if err != nil {
		t.Fatalf("failed to create the local user: %v", err)
	}

	f.provider.SignIn(ssotest.Claims{"sub": "subject-" + username, "preferred_username": username})
	_, page := f.get(t, f.browser(t), StartPath)
	if got := signedIn(t, page).UserId; got != local.ID.String() {
		t.Errorf("signed in as %s, want the existing %s", got, local.ID)
	}

	f.provider.SignIn(ssotest.Claims{"sub": "someone-else", "preferred_username": username})
	if status, _ := f.get(t, f.browser(t), StartPath); status != http.StatusForbidden {
		t.Errorf("a second subject took over a linked account: status %d", status)
	}
}

func TestHandler_ReadsUserinfoForClaimsTheIDTokenLeftOut(t *testing.T) {
	f := newFixture(t, func(c *env.OIDC) { c.AllowedGroup = "jellyfin" })
	username := f.username(t)
	f.provider.SignIn(ssotest.Claims{"sub": "subject-" + username})
	f.provider.UserInfo(ssotest.Claims{"preferred_username": username, "groups": []string{"jellyfin"}})

	status, page := f.get(t, f.browser(t), StartPath)
	if status != http.StatusOK {
		t.Fatalf("status = %d\n%s", status, page)
	}
	if _, err := f.users.UserByUsername(context.Background(), username); err != nil {
		t.Errorf("the userinfo username was not used: %v", err)
	}
}

func TestHandler_NotFoundWhenOff(t *testing.T) {
	handler := New(env.Config{}, domain.New(env.Config{}, nil), nil, nil)
	for _, path := range []string{StartPath, domain.CallbackPath} {
		recorder := httptest.NewRecorder()
		handler.Serve(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		if recorder.Code != http.StatusNotFound {
			t.Errorf("%s = %d, want 404", path, recorder.Code)
		}
	}
}
