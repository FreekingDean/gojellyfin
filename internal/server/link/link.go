package link

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"rsc.io/qr"

	"github.com/FreekingDean/gojellyfin/internal/env"
	"github.com/FreekingDean/gojellyfin/internal/http/mux"
	"github.com/FreekingDean/gojellyfin/internal/quickconnect"
)

const (
	clientRoot       = "/web/#"
	quickConnectPage = "/quickconnect"
	ssoReturnCookie  = "gojellyfin_sso_return"
	ssoReturnPath    = "/sso/oidc"
	ssoReturnMaxAge  = 600
	pendingCookie    = "gojellyfin_quickconnect"
	qrPath           = "/link/qr.svg"
	devicePath       = "/link/tv"
	deviceLength     = 16
	pendingMaxAge    = 600
	quietZone        = 4
)

var redirects = []string{
	"GET /link",
	"HEAD /link",
	"GET /link/{code}",
	"HEAD /link/{code}",
}

var (
	nonDigit    = regexp.MustCompile(`\D`)
	knownDevice = regexp.MustCompile(`^[0-9a-f]{16}$`)
)

type WaitingService interface {
	Waiting(ctx context.Context) ([]*quickconnect.Request, error)
	Nearby(ctx context.Context, address string) ([]*quickconnect.Request, error)
}

type Handler struct {
	published    string
	secure       bool
	quickconnect WaitingService
}

func New(config env.Config, quickconnect *quickconnect.Service) *Handler {
	return &Handler{
		published:    strings.TrimSuffix(config.PublishedServerURL, "/"),
		secure:       strings.HasPrefix(config.PublishedServerURL, "https://"),
		quickconnect: quickconnect,
	}
}

func (h *Handler) Register(m *mux.Mux) {
	m.HandleFunc("GET "+qrPath, h.QR)
	m.HandleFunc("GET "+devicePath+"/{device}", h.FromDevice)
	for _, pattern := range redirects {
		m.HandleFunc(pattern, h.Redirect)
	}
}

func (h *Handler) Redirect(w http.ResponseWriter, r *http.Request) {
	code := codeFrom(r)
	if code == "" {
		code = h.onlyNearby(r)
	}
	route := Route(code)

	http.SetCookie(w, &http.Cookie{
		Name:     ssoReturnCookie,
		Value:    url.QueryEscape(route),
		Path:     ssoReturnPath,
		MaxAge:   ssoReturnMaxAge,
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, clientRoot+route, http.StatusFound)
}

func (h *Handler) onlyNearby(r *http.Request) string {
	near, err := h.quickconnect.Nearby(r.Context(), r.RemoteAddr)
	if err != nil || len(near) == 0 {
		return ""
	}

	for _, request := range near[1:] {
		if request.DeviceName != near[0].DeviceName || request.AppName != near[0].AppName {
			return ""
		}
	}

	return near[0].Code
}

func Route(code string) string {
	if code == "" {
		return quickConnectPage
	}

	return quickConnectPage + "?" + url.Values{"code": {code}}.Encode()
}

func codeFrom(r *http.Request) string {
	code := r.PathValue("code")
	if code == "" {
		code = r.URL.Query().Get("code")
	}

	return nonDigit.ReplaceAllString(code, "")
}

func (h *Handler) Remember(w http.ResponseWriter, request *quickconnect.Request) {
	if h.published == "" {
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     pendingCookie,
		Value:    DeviceToken(request.DeviceID),
		Path:     qrPath,
		MaxAge:   pendingMaxAge,
		Secure:   h.secure,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
}

func DeviceToken(deviceID string) string {
	sum := sha256.Sum256([]byte(deviceID))

	return hex.EncodeToString(sum[:])[:deviceLength]
}

func (h *Handler) Enabled() bool {
	return h.published != ""
}

func (h *Handler) Target(device string) string {
	if !knownDevice.MatchString(device) {
		return h.published + "/link"
	}

	return h.published + devicePath + "/" + device
}

func (h *Handler) QR(w http.ResponseWriter, r *http.Request) {
	if h.published == "" {
		http.NotFound(w, r)
		return
	}

	device := ""
	if cookie, err := r.Cookie(pendingCookie); err == nil {
		device = cookie.Value
	}

	code, err := qr.Encode(h.Target(device), qr.M)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "image/svg+xml")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(svg(code))
}

func (h *Handler) FromDevice(w http.ResponseWriter, r *http.Request) {
	waiting, err := h.quickconnect.Waiting(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	for _, request := range waiting {
		if DeviceToken(request.DeviceID) == r.PathValue("device") {
			r.SetPathValue("code", request.Code)
			break
		}
	}

	h.Redirect(w, r)
}

func svg(code *qr.Code) []byte {
	side := code.Size + 2*quietZone

	var modules strings.Builder
	for y := range code.Size {
		for x := range code.Size {
			if code.Black(x, y) {
				fmt.Fprintf(&modules, "M%d %dh1v1h-1z", x+quietZone, y+quietZone)
			}
		}
	}

	return fmt.Appendf(nil,
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d" shape-rendering="crispEdges"><rect width="100%%" height="100%%" fill="#fff"/><path fill="#000" d="%s"/></svg>`,
		side, side, modules.String())
}

func (h *Handler) CSS() string {
	if h.published == "" {
		return ""
	}

	caption := cssString("Or scan this with your phone, or open " + h.address() + "/link")

	return `
#quickConnectAlert .dialogContentInner .text::after{content:` + caption + `;display:block;margin-top:1em;opacity:.8}
#quickConnectAlert .dialogContentInner::after{content:"";display:block;width:14em;height:14em;max-width:100%;margin:1.5em auto 0;background:url("` + qrPath + `") center/contain no-repeat}
.backdropImage[data-url*="/Branding/Splashscreen"]{display:none}
`
}

func cssString(text string) string {
	var quoted strings.Builder
	quoted.WriteByte('"')
	for _, r := range text {
		if r == '"' || r == '\\' || r < ' ' {
			fmt.Fprintf(&quoted, "\\%x ", r)
			continue
		}
		quoted.WriteRune(r)
	}
	quoted.WriteByte('"')

	return quoted.String()
}
