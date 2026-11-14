package dto

import (
	"encoding/json"
	"testing"
	"time"

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
	} {
		if _, ok := fields[required]; !ok {
			t.Errorf("SessionInfoDto carries no %s, which the Kotlin SDK refuses to decode", required)
		}
	}
}
