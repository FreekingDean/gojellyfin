package config

import (
	"context"
	"strings"

	"github.com/FreekingDean/gojellyfin/internal/server/api"
)

func (s *Server) GetBrandingOptions(ctx context.Context, request api.GetBrandingOptionsRequestObject) (api.GetBrandingOptionsResponseObject, error) {
	branding, err := s.brandingConfiguration(ctx)
	if err != nil {
		return nil, err
	}

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
	branding, err := s.brandingConfiguration(ctx)
	if err != nil {
		return "", err
	}

	return deref(branding.CustomCss), nil
}
