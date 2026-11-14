package link

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/FreekingDean/gojellyfin/internal/env"
	"github.com/FreekingDean/gojellyfin/internal/http/mux"
	"github.com/FreekingDean/gojellyfin/internal/quickconnect"
)

type waiting []*quickconnect.Request

func (w waiting) Waiting(context.Context) ([]*quickconnect.Request, error) {
	return w, nil
}

func handler(published string, requests waiting) (*Handler, *mux.Mux) {
	h := New(env.Config{PublishedServerURL: published}, nil)
	h.quickconnect = requests
	m := mux.New()
	h.Register(m)

	return h, m
}

func TestRedirect(t *testing.T) {
	_, m := handler("https://tv.example", nil)

	tests := []struct {
		name   string
		method string
		path   string
		want   string
	}{
		{"bare", http.MethodGet, "/link", "/web/#/quickconnect"},
		{"trailing slash", http.MethodGet, "/link/", "/web/#/quickconnect"},
		{"any case", http.MethodGet, "/LINK", "/web/#/quickconnect"},
		{"code in the path", http.MethodGet, "/link/123456", "/web/#/quickconnect?code=123456"},
		{"code in the query", http.MethodGet, "/link?code=654321", "/web/#/quickconnect?code=654321"},
		{"code typed with a space", http.MethodGet, "/link/123%20456", "/web/#/quickconnect?code=123456"},
		{"code typed with a dash", http.MethodGet, "/link/123-456", "/web/#/quickconnect?code=123456"},
		{"nothing numeric", http.MethodGet, "/link/hello", "/web/#/quickconnect"},
		{"markup", http.MethodGet, "/link?code=%22%3E%3Cscript%3E1", "/web/#/quickconnect?code=1"},
		{"head", http.MethodHead, "/link/123456", "/web/#/quickconnect?code=123456"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			m.ServeHTTP(recorder, httptest.NewRequest(tt.method, tt.path, nil))

			if recorder.Code != http.StatusFound {
				t.Fatalf("want 302, got %d", recorder.Code)
			}
			if got := recorder.Header().Get("Location"); got != tt.want {
				t.Errorf("want Location %q, got %q", tt.want, got)
			}

			cookies := recorder.Result().Cookies()
			if len(cookies) != 1 {
				t.Fatalf("want one cookie, got %d", len(cookies))
			}
			returnTo, err := url.QueryUnescape(cookies[0].Value)
			if err != nil {
				t.Fatalf("the return cookie does not unescape: %v", err)
			}
			if got := "/web/#" + returnTo; got != tt.want {
				t.Errorf("want the sign-in to return to %q, got %q", tt.want, got)
			}
			if cookie := cookies[0]; cookie.Name != "gojellyfin_sso_return" || cookie.Path != "/sso/oidc" || cookie.MaxAge != 600 || !cookie.Secure || !cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode {
				t.Errorf("want the return cookie the sign-in reads, got %+v", cookie)
			}
		})
	}
}

func TestRemember(t *testing.T) {
	tests := []struct {
		name       string
		published  string
		wantCookie bool
		wantSecure bool
	}{
		{"over https", "https://tv.example", true, true},
		{"over http", "http://tv.example", true, false},
		{"with no published address there is no QR to read it", "", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, _ := handler(tt.published, nil)
			recorder := httptest.NewRecorder()

			h.Remember(recorder, &quickconnect.Request{Secret: "the-secret", Code: "123456", DeviceID: "living-room"})

			cookies := recorder.Result().Cookies()
			if !tt.wantCookie {
				if len(cookies) != 0 {
					t.Fatalf("want no cookie, got %+v", cookies)
				}
				return
			}
			if len(cookies) != 1 {
				t.Fatalf("want one cookie, got %d", len(cookies))
			}
			cookie := cookies[0]
			if cookie.Name != "gojellyfin_quickconnect" || cookie.Value != DeviceToken("living-room") || cookie.Path != "/link/qr.svg" || cookie.MaxAge != 600 || !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode || cookie.Secure != tt.wantSecure {
				t.Errorf("want the device, and never the secret, scoped to the QR, got %+v", cookie)
			}
		})
	}
}

func TestTarget(t *testing.T) {
	h, _ := handler("https://tv.example/", nil)

	tests := []struct {
		name   string
		device string
		want   string
	}{
		{"a TV that started Quick Connect", DeviceToken("living-room"), "https://tv.example/link/tv/" + DeviceToken("living-room")},
		{"nobody in particular", "", "https://tv.example/link"},
		{"something that is not a device", "../../admin", "https://tv.example/link"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := h.Target(tt.device); got != tt.want {
				t.Errorf("want %q, got %q", tt.want, got)
			}
		})
	}
}

func TestQR(t *testing.T) {
	_, m := handler("https://tv.example", nil)

	plain := httptest.NewRecorder()
	m.ServeHTTP(plain, httptest.NewRequest(http.MethodGet, "/link/qr.svg", nil))

	bound := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/link/qr.svg", nil)
	request.AddCookie(&http.Cookie{Name: "gojellyfin_quickconnect", Value: DeviceToken("living-room")})
	m.ServeHTTP(bound, request)

	for _, recorder := range []*httptest.ResponseRecorder{plain, bound} {
		if recorder.Code != http.StatusOK {
			t.Fatalf("want 200, got %d", recorder.Code)
		}
		if got := recorder.Header().Get("Content-Type"); got != "image/svg+xml" {
			t.Errorf("want an svg, got %q", got)
		}
		if got := recorder.Header().Get("Cache-Control"); got != "no-store" {
			t.Errorf("want a QR nobody caches, got %q", got)
		}
		if !strings.HasPrefix(recorder.Body.String(), "<svg") {
			t.Errorf("want an svg body, got %q", recorder.Body.String())
		}
	}
	if plain.Body.String() == bound.Body.String() {
		t.Error("want the QR of a TV that started Quick Connect to differ from the bare one")
	}

	t.Run("is not served without a published address", func(t *testing.T) {
		_, m := handler("", nil)
		recorder := httptest.NewRecorder()
		m.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/link/qr.svg", nil))

		if recorder.Code != http.StatusNotFound {
			t.Errorf("want 404, got %d", recorder.Code)
		}
	})
}

func TestFromDevice(t *testing.T) {
	_, m := handler("https://tv.example", waiting{
		{Code: "222222", DeviceID: "living-room"},
		{Code: "111111", DeviceID: "living-room"},
		{Code: "333333", DeviceID: "bedroom"},
	})

	tests := []struct {
		name   string
		device string
		want   string
	}{
		{"the newest code that TV is waiting on", DeviceToken("living-room"), "/web/#/quickconnect?code=222222"},
		{"another TV's code", DeviceToken("bedroom"), "/web/#/quickconnect?code=333333"},
		{"a TV with nothing waiting", DeviceToken("kitchen"), "/web/#/quickconnect"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			m.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/link/tv/"+tt.device, nil))

			if recorder.Code != http.StatusFound {
				t.Fatalf("want 302, got %d", recorder.Code)
			}
			if got := recorder.Header().Get("Location"); got != tt.want {
				t.Errorf("want Location %q, got %q", tt.want, got)
			}
		})
	}
}

func TestSplashscreen(t *testing.T) {
	h, _ := handler("https://tv.example", nil)

	plain, err := h.Splashscreen("")
	if err != nil {
		t.Fatalf("Splashscreen returned %v", err)
	}
	bound, err := h.Splashscreen("living-room")
	if err != nil {
		t.Fatalf("Splashscreen returned %v", err)
	}

	picture, err := png.Decode(bytes.NewReader(plain))
	if err != nil {
		t.Fatalf("the splashscreen is not a png: %v", err)
	}
	if got := picture.Bounds().Size(); got != image.Pt(1920, 1080) {
		t.Errorf("want a 1920x1080 picture so a TV crops nothing, got %v", got)
	}
	if bytes.Equal(plain, bound) {
		t.Error("want the splashscreen of a known device to carry its own QR")
	}
}

func TestCSS(t *testing.T) {
	h, _ := handler("https://tv.example", nil)
	css := h.CSS()

	for _, want := range []string{`#quickConnectAlert`, `url("/link/qr.svg")`, `.backdropImage[data-url*="/Branding/Splashscreen"]{display:none}`, `"Or scan this with your phone, or open tv.example/link"`} {
		if !strings.Contains(css, want) {
			t.Errorf("want the stylesheet to carry %q, got %s", want, css)
		}
	}

	unpublished, _ := handler("", nil)
	if got := unpublished.CSS(); got != "" {
		t.Errorf("want no stylesheet without a published address, got %q", got)
	}

	if got := cssString(`a"b\c`); got != `"a\22 b\5c c"` {
		t.Errorf("want quotes and backslashes escaped, got %s", got)
	}
}
