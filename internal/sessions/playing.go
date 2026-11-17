package sessions

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/store"
	"github.com/FreekingDean/gojellyfin/internal/store/entities"
	sessionmodel "github.com/FreekingDean/gojellyfin/internal/store/session"
)

const PlayingTimeout = 5 * time.Minute

type Playback struct {
	ItemID        uuid.UUID
	PositionTicks int64
	PlayMethod    string
	IsPaused      bool
}

func (s *Service) Playing(ctx context.Context, sessionID uuid.UUID, address string, playback Playback) error {
	err := s.store.Session.UpdateOneID(sessionID).
		SetNowPlayingItemID(playback.ItemID).
		SetPositionTicks(playback.PositionTicks).
		SetPlayMethod(playback.PlayMethod).
		SetIsPaused(playback.IsPaused).
		Modify(store.StampNow(sessionmodel.FieldPlayingAt, sessionmodel.FieldLastActivityAt)).
		SetRemoteEndpoint(host(address)).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to record what the session is playing: %w", err)
	}

	return nil
}

func (s *Service) Planned(ctx context.Context, sessionID, sourceID uuid.UUID, plan entities.PlaybackPlan) error {
	err := s.store.Session.UpdateOneID(sessionID).
		SetPlannedSourceID(sourceID).
		SetPlaybackPlan(&plan).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to record how the session is being served: %w", err)
	}

	return nil
}

func Planned(session *Session) (*MediaSource, *entities.PlaybackPlan) {
	source := session.PlannedSource
	if session.PlaybackPlan == nil || source == nil || session.NowPlaying == nil {
		return nil, nil
	}
	if source.ItemID != session.NowPlaying.ID && !StandsIn(session.PlaybackPlan, session.NowPlaying.ID) {
		return nil, nil
	}

	return source, session.PlaybackPlan
}

func StandsIn(plan *entities.PlaybackPlan, itemID uuid.UUID) bool {
	return plan != nil && plan.StandsInFor != nil && *plan.StandsInFor == itemID
}

func (s *Service) Stopped(ctx context.Context, sessionID uuid.UUID, address string) error {
	err := s.store.Session.UpdateOneID(sessionID).
		ClearNowPlayingItemID().
		SetPositionTicks(0).
		ClearPlayMethod().
		SetIsPaused(false).
		ClearPlayingAt().
		Modify(store.StampNow(sessionmodel.FieldLastActivityAt)).
		SetRemoteEndpoint(host(address)).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to record that the session stopped: %w", err)
	}

	return nil
}

func (s *Service) Seen(ctx context.Context, sessionID uuid.UUID, address string) error {
	if err := s.store.Session.UpdateOneID(sessionID).SetRemoteEndpoint(host(address)).Exec(ctx); err != nil {
		return fmt.Errorf("failed to record where the session came from: %w", err)
	}

	return nil
}

func NowPlaying(session *Session, now time.Time) bool {
	return session.NowPlaying != nil && session.PlayingAt != nil && now.Sub(*session.PlayingAt) < PlayingTimeout
}

func host(address string) string {
	if parsed, _, err := net.SplitHostPort(address); err == nil {
		return parsed
	}

	return address
}
