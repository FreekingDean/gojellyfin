package sessions

import (
	"context"
	"fmt"
)

type Capabilities struct {
	PlayableMediaTypes           []string
	SupportedCommands            []string
	SupportsMediaControl         bool
	SupportsPersistentIdentifier bool
	Profile                      map[string]any
}

func (s *Service) ReportCapabilities(ctx context.Context, session *Session, capabilities Capabilities) error {
	if session.Device == nil {
		return nil
	}

	update := s.store.Device.UpdateOneID(session.Device.ID).
		SetPlayableMediaTypes(capabilities.PlayableMediaTypes).
		SetSupportedCommands(capabilities.SupportedCommands).
		SetSupportsMediaControl(capabilities.SupportsMediaControl).
		SetSupportsPersistentIdentifier(capabilities.SupportsPersistentIdentifier)
	if len(capabilities.Profile) > 0 {
		update.SetProfile(capabilities.Profile)
	} else {
		update.ClearProfile()
	}

	if err := update.Exec(ctx); err != nil {
		return fmt.Errorf("failed to record the device's capabilities: %w", err)
	}

	return nil
}

func Profile(session *Session) map[string]any {
	if session == nil || session.Device == nil {
		return nil
	}

	return session.Device.Profile
}
