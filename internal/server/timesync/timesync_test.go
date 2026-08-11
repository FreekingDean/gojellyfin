package timesync

import (
	"context"
	"testing"
	"time"

	"github.com/FreekingDean/gojellyfin/internal/server/api"
)

func TestGetUtcTime(t *testing.T) {
	before := time.Now().UTC()

	response, err := New().GetUtcTime(context.Background(), api.GetUtcTimeRequestObject{})
	if err != nil {
		t.Fatalf("failed to get the time: %v", err)
	}

	after := time.Now().UTC()

	result, ok := response.(api.GetUtcTime200JSONResponse)
	if !ok {
		t.Fatalf("response = %T, want api.GetUtcTime200JSONResponse", response)
	}

	received := *result.RequestReceptionTime
	transmitted := *result.ResponseTransmissionTime

	if received.Before(before) || transmitted.After(after) {
		t.Errorf("times = %v, %v, want within %v and %v", received, transmitted, before, after)
	}
	if transmitted.Before(received) {
		t.Errorf("transmission %v is before reception %v", transmitted, received)
	}
	if received.Location() != time.UTC || transmitted.Location() != time.UTC {
		t.Errorf("locations = %v, %v, want UTC", received.Location(), transmitted.Location())
	}
}
