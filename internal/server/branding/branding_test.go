package branding

import (
	"context"
	"strings"
	"testing"

	"github.com/FreekingDean/gojellyfin/internal/storetest"

	"github.com/FreekingDean/gojellyfin/internal/config"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	"github.com/FreekingDean/gojellyfin/internal/shelves"
)

type styling string

func (s styling) CSS() string { return string(s) }

func (s styling) Enabled() bool { return s != "" }

type signOn bool

func (s signOn) Enabled() bool { return bool(s) }

type titles []shelves.Title

func (t titles) Titles(context.Context) ([]shelves.Title, error) { return t, nil }

func TestServer_GetBrandingCss(t *testing.T) {
	connection := storetest.Open(t)

	id := shelves.ID("recently-aired")
	server := &Server{
		config:  config.New(connection.Client()),
		signOn:  signOn(false),
		links:   styling(""),
		shelves: titles{{ID: id, Name: `Recently "Aired"`}},
	}

	css, err := server.brandingCss(context.Background())
	if err != nil {
		t.Fatalf("brandingCss returned %v", err)
	}

	for _, want := range []string{
		`a.sectionTitleTextButton[href*="` + id.String() + `"]>h2.sectionTitle{text-indent:-9999px;line-height:0}`,
		`::after{content:"Recently \"Aired\"";`,
	} {
		if !strings.Contains(css, want) {
			t.Errorf("css = %q, want it to contain %q", css, want)
		}
	}

	server.links = styling("#quickConnectAlert{}")
	options, err := server.GetBrandingOptions(context.Background(), api.GetBrandingOptionsRequestObject{})
	if err != nil {
		t.Fatalf("GetBrandingOptions returned %v", err)
	}
	custom := apiutil.Deref(options.(api.GetBrandingOptions200JSONResponse).CustomCss)
	for _, want := range []string{"#quickConnectAlert{}", "sectionTitleTextButton"} {
		if !strings.Contains(custom, want) {
			t.Errorf("CustomCss = %q, want it to contain %q, since jellyfin-web 10.11 reads its stylesheet from there", custom, want)
		}
	}
	server.links = styling("")

	server.shelves = titles(nil)
	plain, err := server.brandingCss(context.Background())
	if err != nil {
		t.Fatalf("brandingCss returned %v", err)
	}
	if strings.Contains(plain, "sectionTitleTextButton") {
		t.Errorf("css = %q, want no shelf rule when no shelf is titled", plain)
	}
}

func TestServer_GetBrandingOptions(t *testing.T) {
	connection := storetest.Open(t)

	tests := []struct {
		name    string
		enabled bool
		want    bool
	}{
		{name: "offers the button when sign-on is configured", enabled: true, want: true},
		{name: "offers nothing when it is not", enabled: false, want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := &Server{config: config.New(connection.Client()), signOn: signOn(test.enabled), links: styling(""), shelves: titles(nil)}

			response, err := server.GetBrandingOptions(context.Background(), api.GetBrandingOptionsRequestObject{})
			if err != nil {
				t.Fatalf("GetBrandingOptions returned %v", err)
			}
			disclaimer := apiutil.Deref(response.(api.GetBrandingOptions200JSONResponse).LoginDisclaimer)

			if got := strings.Contains(disclaimer, `action="/sso/oidc/start"`); got != test.want {
				t.Errorf("button offered = %v, want %v in %q", got, test.want, disclaimer)
			}
		})
	}
}
