package sso

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/auth"
	"github.com/FreekingDean/gojellyfin/internal/config"
	"github.com/FreekingDean/gojellyfin/internal/env"
	"github.com/FreekingDean/gojellyfin/internal/sessions"
	domain "github.com/FreekingDean/gojellyfin/internal/sso"
	"github.com/FreekingDean/gojellyfin/internal/users"
)

const (
	StartPath = "/sso/oidc/start"

	cookiePath   = "/sso/oidc"
	flowCookie   = "gojellyfin_oidc"
	returnCookie = "gojellyfin_sso_return"
	flowLifetime = 10 * time.Minute
	maxReturnLen = 512

	credentialsKey = "jellyfin_credentials"
	webRoot        = "/web/"
	loginRoute     = "/web/#/login.html"
	deviceName     = "Single sign-on"
	appName        = "Jellyfin Web"
)

var pageRoots = []string{"/admin", "/request"}

var Routes = []string{
	"GET " + StartPath,
	"GET " + domain.CallbackPath,
}

var signedInPage = template.Must(template.New("signedin").Parse(`<!doctype html>
<html lang="en">
<head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>Signing in</title></head>
<body style="background:#101010;color:#fff;font-family:sans-serif;text-align:center;padding-top:20vh">
<p>Signing you in…</p>
<script>
(function () {
  var signin = {{.}};
  var credentials = {};
  try { credentials = JSON.parse(localStorage.getItem(signin.Key)) || {}; } catch (e) {}
  var servers = credentials.Servers || [];
  var server = servers.filter(function (entry) { return entry.Id === signin.ServerId; })[0];
  if (!server) {
    server = { Id: signin.ServerId, ManualAddress: location.origin, LastConnectionMode: 2 };
    servers.push(server);
  }
  server.AccessToken = signin.AccessToken;
  server.UserId = signin.UserId;
  server.DateLastAccessed = Date.now();
  credentials.Servers = servers;
  localStorage.setItem(signin.Key, JSON.stringify(credentials));
  localStorage.setItem('enableAutoLogin', 'true');
  location.replace(signin.Target);
})();
</script>
</body>
</html>
`))

var failedPage = template.Must(template.New("failed").Parse(`<!doctype html>
<html lang="en">
<head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>Sign-in failed</title></head>
<body style="background:#101010;color:#fff;font-family:sans-serif;text-align:center;padding-top:20vh">
<h1>Sign-in failed</h1>
<p>{{.Reason}}</p>
<p><a style="color:#00a4dc" href="{{.Login}}">Back to sign in</a></p>
</body>
</html>
`))

type signIn struct {
	Key         string
	ServerId    string
	AccessToken string
	UserId      string
	Target      string
}

type failure struct {
	Reason string
	Login  string
}

type pending struct {
	Flow   domain.Flow `json:"flow"`
	Return string      `json:"return,omitempty"`
}

type Handler struct {
	sso      *domain.Service
	sessions *sessions.Service
	users    *users.Service
	secure   bool
}

func New(config env.Config, sso *domain.Service, sessions *sessions.Service, users *users.Service) *Handler {
	return &Handler{
		sso:      sso,
		sessions: sessions,
		users:    users,
		secure:   strings.HasPrefix(config.PublishedServerURL, "https://"),
	}
}

func (h *Handler) Serve(w http.ResponseWriter, r *http.Request) {
	if !h.sso.Enabled() {
		http.NotFound(w, r)
		return
	}

	if strings.EqualFold(strings.TrimSuffix(r.URL.Path, "/"), domain.CallbackPath) {
		h.callback(w, r)
		return
	}
	h.start(w, r)
}

func (h *Handler) start(w http.ResponseWriter, r *http.Request) {
	target, flow, err := h.sso.Begin(r.Context())
	if err != nil {
		log.Printf("sso: %v", err)
		h.fail(w, http.StatusBadGateway, "The identity provider could not be reached. Try again in a moment.")
		return
	}

	value, err := json.Marshal(pending{Flow: flow, Return: h.returnTo(r)})
	if err != nil {
		h.fail(w, http.StatusInternalServerError, "The sign-in could not be started.")
		return
	}
	h.setCookie(w, flowCookie, base64.RawURLEncoding.EncodeToString(value), flowLifetime)

	http.Redirect(w, r, target, http.StatusFound)
}

func (h *Handler) callback(w http.ResponseWriter, r *http.Request) {
	state, ok := h.pending(r)
	h.clearCookie(w, flowCookie)
	h.clearCookie(w, returnCookie)
	if !ok {
		h.fail(w, http.StatusBadRequest, "This sign-in expired or did not start here. Start it again from the sign-in page.")
		return
	}

	query := r.URL.Query()
	if reason := query.Get("error"); reason != "" {
		h.fail(w, http.StatusForbidden, "The identity provider refused the sign-in: "+strings.TrimSpace(reason+" "+query.Get("error_description")))
		return
	}

	user, err := h.sso.Finish(r.Context(), state.Flow, query.Get("state"), query.Get("code"))
	if err != nil {
		h.refuse(w, err)
		return
	}
	if user.Policy != nil && user.Policy.IsDisabled {
		h.fail(w, http.StatusForbidden, "This account is disabled.")
		return
	}

	token, err := h.mint(r.Context(), user, r.UserAgent())
	if errors.Is(err, sessions.ErrOutsideSchedule) {
		h.fail(w, http.StatusForbidden, "This account cannot sign in at this time.")
		return
	}
	if err != nil {
		log.Printf("sso: %v", err)
		h.fail(w, http.StatusInternalServerError, "The session could not be created.")
		return
	}

	target := webRoot
	switch {
	case ownPage(state.Return):
		target = state.Return
	case state.Return != "":
		target = webRoot + "#" + state.Return
	}

	h.page(w, http.StatusOK, signedInPage, signIn{
		Key:         credentialsKey,
		ServerId:    config.ServerID,
		AccessToken: token,
		UserId:      user.ID.String(),
		Target:      target,
	})
}

func (h *Handler) mint(ctx context.Context, user *users.User, userAgent string) (string, error) {
	token, err := auth.NewToken()
	if err != nil {
		return "", err
	}

	if _, err := h.sessions.Create(ctx, user.ID, token, device(user.ID, userAgent)); err != nil {
		return "", err
	}
	if err := h.users.TouchLogin(ctx, user.ID); err != nil {
		return "", err
	}

	return token, nil
}

func device(userID uuid.UUID, userAgent string) sessions.Device {
	sum := sha256.Sum256([]byte(userID.String() + "\x00" + userAgent))

	return sessions.Device{
		ClientID: "sso-" + hex.EncodeToString(sum[:16]),
		Name:     deviceName,
		AppName:  appName,
	}
}

func (h *Handler) refuse(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrStateInvalid):
		h.fail(w, http.StatusBadRequest, "This sign-in expired or did not start here. Start it again from the sign-in page.")
	case errors.Is(err, domain.ErrNotAllowed):
		h.fail(w, http.StatusForbidden, "Your account is not in the group allowed to use this server.")
	case errors.Is(err, domain.ErrNoUsername):
		h.fail(w, http.StatusForbidden, "The identity provider did not say what your username is.")
	case errors.Is(err, users.ErrUsernameClaimed):
		h.fail(w, http.StatusForbidden, "A local account already uses your username. Ask an administrator to link it.")
	default:
		log.Printf("sso: %v", err)
		h.fail(w, http.StatusBadGateway, "The identity provider's answer could not be verified.")
	}
}

func (h *Handler) pending(r *http.Request) (pending, bool) {
	cookie, err := r.Cookie(flowCookie)
	if err != nil {
		return pending{}, false
	}
	raw, err := base64.RawURLEncoding.DecodeString(cookie.Value)
	if err != nil {
		return pending{}, false
	}
	var state pending
	if err := json.Unmarshal(raw, &state); err != nil || state.Flow.State == "" {
		return pending{}, false
	}
	if !validReturn(state.Return) {
		state.Return = ""
	}

	return state, true
}

func (h *Handler) returnTo(r *http.Request) string {
	if route := r.URL.Query().Get("redirect"); validReturn(route) {
		return route
	}
	cookie, err := r.Cookie(returnCookie)
	if err != nil {
		return ""
	}
	route, err := url.QueryUnescape(cookie.Value)
	if err != nil || !validReturn(route) {
		return ""
	}

	return route
}

func ownPage(route string) bool {
	return slices.ContainsFunc(pageRoots, func(root string) bool {
		return route == root || strings.HasPrefix(route, root+"/") || strings.HasPrefix(route, root+"?")
	})
}

func validReturn(route string) bool {
	if route == "" || len(route) > maxReturnLen || !strings.HasPrefix(route, "/") {
		return false
	}
	if strings.HasPrefix(route, "//") || strings.HasPrefix(route, "/\\") {
		return false
	}

	return !strings.ContainsFunc(route, func(r rune) bool { return r < ' ' || r == 0x7f })
}

func (h *Handler) setCookie(w http.ResponseWriter, name, value string, lifetime time.Duration) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     cookiePath,
		MaxAge:   int(lifetime.Seconds()),
		HttpOnly: true,
		Secure:   h.secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func (h *Handler) clearCookie(w http.ResponseWriter, name string) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Path:     cookiePath,
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func (h *Handler) fail(w http.ResponseWriter, status int, reason string) {
	h.page(w, status, failedPage, failure{Reason: reason, Login: loginRoute})
}

func (h *Handler) page(w http.ResponseWriter, status int, page *template.Template, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.WriteHeader(status)
	if err := page.Execute(w, data); err != nil {
		log.Printf("sso: rendering a page: %v", err)
	}
}
