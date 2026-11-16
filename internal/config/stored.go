package config

import (
	"context"
	"encoding/json"
	"fmt"
)

const (
	SystemKey   = "system"
	BrandingKey = "branding"
)

type Server struct {
	ServerName               string
	IsStartupWizardCompleted bool
	CachePath                string
	MetadataPath             string
}

type Branding struct {
	CustomCss           string
	LoginDisclaimer     string
	SplashscreenEnabled bool
}

func DefaultServer() Server {
	return Server{ServerName: "gojellyfin", IsStartupWizardCompleted: true}
}

func DefaultBranding() Branding {
	return Branding{LoginDisclaimer: "This is a go server mimicing jellyfin dont be afraid."}
}

func (s *Service) Server(ctx context.Context) (Server, error) {
	server := DefaultServer()

	return server, s.read(ctx, SystemKey, &server)
}

func (s *Service) Branding(ctx context.Context) (Branding, error) {
	branding := DefaultBranding()

	return branding, s.read(ctx, BrandingKey, &branding)
}

func (s *Service) read(ctx context.Context, key string, into any) error {
	value, err := s.Configuration(ctx, key)
	if err != nil || value == nil {
		return err
	}
	if err := json.Unmarshal(value, into); err != nil {
		return fmt.Errorf("failed to read the %s configuration: %w", key, err)
	}

	return nil
}
