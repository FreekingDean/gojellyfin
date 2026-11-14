package branding

import (
	"context"
	"fmt"
	"strings"

	"github.com/FreekingDean/gojellyfin/internal/config"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	"github.com/FreekingDean/gojellyfin/internal/server/configuration"
	"github.com/FreekingDean/gojellyfin/internal/server/link"
	"github.com/FreekingDean/gojellyfin/internal/shelves"
	"github.com/FreekingDean/gojellyfin/internal/sso"
)

const signOnButton = `<form action="/sso/oidc/start" method="get" style="margin-top:1em"><button is="emby-button" type="submit" class="raised block emby-button button-submit">Sign in with SSO</button></form>`

const shelfTitle = `
a.sectionTitleTextButton[href*="%[1]s"]>h2.sectionTitle{text-indent:-9999px;line-height:0}
a.sectionTitleTextButton[href*="%[1]s"]>h2.sectionTitle::after{content:"%[2]s";display:block;text-indent:0;line-height:normal}`

var cssString = strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", " ")

type SignOn interface {
	Enabled() bool
}

type Styling interface {
	Enabled() bool
	CSS() string
}

type Shelves interface {
	Titles() []shelves.Title
}

type Server struct {
	config  *config.Service
	signOn  SignOn
	links   Styling
	shelves Shelves
}

func New(config *config.Service, signOn *sso.Service, links *link.Handler, shelves *shelves.Service) *Server {
	return &Server{config: config, signOn: signOn, links: links, shelves: shelves}
}

func (s *Server) GetBrandingOptions(ctx context.Context, request api.GetBrandingOptionsRequestObject) (api.GetBrandingOptionsResponseObject, error) {
	branding, err := configuration.BrandingConfiguration(ctx, s.config)
	if err != nil {
		return nil, err
	}

	if s.signOn.Enabled() {
		branding.LoginDisclaimer = apiutil.Ptr(strings.TrimSpace(apiutil.Deref(branding.LoginDisclaimer) + "\n\n" + signOnButton))
	}

	if s.links.Enabled() {
		branding.SplashscreenEnabled = apiutil.Ptr(true)
	}

	css, err := s.brandingCss(ctx)
	if err != nil {
		return nil, err
	}
	branding.CustomCss = apiutil.Ptr(css)

	return api.GetBrandingOptions200JSONResponse(branding), nil
}

func (s *Server) GetBrandingCss(ctx context.Context, request api.GetBrandingCssRequestObject) (api.GetBrandingCssResponseObject, error) {
	css, err := s.brandingCss(ctx)
	if err != nil {
		return nil, err
	}

	return api.GetBrandingCss200TextcssResponse{
		Body:          strings.NewReader(css),
		ContentLength: int64(len(css)),
	}, nil
}

func (s *Server) GetBrandingCss2(ctx context.Context, request api.GetBrandingCss2RequestObject) (api.GetBrandingCss2ResponseObject, error) {
	css, err := s.brandingCss(ctx)
	if err != nil {
		return nil, err
	}

	return api.GetBrandingCss2200TextcssResponse{
		Body:          strings.NewReader(css),
		ContentLength: int64(len(css)),
	}, nil
}

func (s *Server) brandingCss(ctx context.Context) (string, error) {
	branding, err := configuration.BrandingConfiguration(ctx, s.config)
	if err != nil {
		return "", err
	}

	css := strings.Builder{}
	css.WriteString(apiutil.Deref(branding.CustomCss))
	css.WriteString(s.links.CSS())
	for _, title := range s.shelves.Titles() {
		fmt.Fprintf(&css, shelfTitle, title.ID, cssString.Replace(title.Name))
	}

	return strings.TrimSpace(css.String()), nil
}
