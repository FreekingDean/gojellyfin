package dto

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	"github.com/FreekingDean/gojellyfin/internal/sessions"
)

func TestSessionDto_CarriesWhatTheKotlinSDKRequires(t *testing.T) {
	encoded, err := json.Marshal(SessionDto(&sessions.Session{LastActivityAt: apiutil.Ptr(time.Now())}))
	if err != nil {
		t.Fatalf("failed to encode the session: %v", err)
	}

	var fields map[string]any
	if err := json.Unmarshal(encoded, &fields); err != nil {
		t.Fatalf("failed to decode the session: %v", err)
	}

	for _, required := range []string{
		"PlayableMediaTypes",
		"LastActivityDate",
		"LastPlaybackCheckIn",
		"IsActive",
		"SupportsMediaControl",
		"SupportsRemoteControl",
		"HasCustomDeviceName",
		"SupportedCommands",
		"PlayState",
	} {
		if _, ok := fields[required]; !ok {
			t.Errorf("SessionInfoDto carries no %s, which the Kotlin SDK refuses to decode", required)
		}
	}
}

func TestSessionDto_SaysWhatIsPlaying(t *testing.T) {
	series := &items.Item{ID: uuid.New(), Kind: "Series", Name: "Fixture Show"}
	episode := &items.Item{ID: uuid.New(), Kind: "Episode", Name: "Pilot", Parent: &items.Item{Kind: "Season", Parent: series}}
	session := &sessions.Session{
		RemoteEndpoint: "203.0.113.7",
		NowPlaying:     episode,
		PlayingAt:      apiutil.Ptr(time.Now()),
		PositionTicks:  42,
		PlayMethod:     "DirectPlay",
	}

	dto := SessionDto(session)
	if apiutil.Deref(dto.RemoteEndPoint) != "203.0.113.7" {
		t.Errorf("RemoteEndPoint = %q, want 203.0.113.7", apiutil.Deref(dto.RemoteEndPoint))
	}
	if dto.NowPlayingItem == nil || apiutil.Deref(dto.NowPlayingItem.SeriesName) != "Fixture Show" || apiutil.Deref(dto.NowPlayingItem.Name) != "Pilot" {
		t.Fatalf("NowPlayingItem = %+v, want Pilot of Fixture Show", dto.NowPlayingItem)
	}
	if apiutil.Deref(dto.PlayState.PositionTicks) != 42 || apiutil.Deref(dto.PlayState.PlayMethod) != api.DirectPlay {
		t.Errorf("PlayState = %+v, want 42 ticks played directly", dto.PlayState)
	}

	session.PlayingAt = apiutil.Ptr(time.Now().Add(-sessions.PlayingTimeout))
	if stale := SessionDto(session); stale.NowPlayingItem != nil || stale.PlayState == nil {
		t.Errorf("a session that stopped reporting still says it is playing")
	}
}
