package notify

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/env"
)

// Two services are two pools and two backends, which is what a second replica
// is as far as LISTEN/NOTIFY is concerned.
func newService(t *testing.T) *Service {
	t.Helper()

	config, err := env.Load()
	if err != nil {
		t.Fatalf("failed to read the environment: %v", err)
	}

	service, err := New(config)
	if err != nil {
		t.Fatalf("failed to open the notify pool: %v", err)
	}
	if err := service.Start(); err != nil {
		t.Fatalf("failed to reach the database, set DATABASE_URL: %v", err)
	}

	t.Cleanup(func() {
		if err := service.Stop(); err != nil {
			t.Errorf("failed to stop the notifier: %v", err)
		}
	})

	return service
}

func receive(t *testing.T, service *Service) chan Envelope {
	t.Helper()

	received := make(chan Envelope, 4)
	service.Handle(func(envelope Envelope) {
		select {
		case received <- envelope:
		default:
		}
	})

	return received
}

func await(t *testing.T, received chan Envelope) Envelope {
	t.Helper()

	select {
	case envelope := <-received:
		return envelope
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for a notification")
	}

	return Envelope{}
}

func TestPublishReachesAnotherReplica(t *testing.T) {
	publisher := newService(t)
	subscriber := newService(t)
	received := receive(t, subscriber)

	session := uuid.New()
	if err := publisher.Publish(context.Background(), []uuid.UUID{session}, "SyncPlayGroupUpdate", map[string]string{"Type": "UserJoined"}); err != nil {
		t.Fatalf("failed to publish: %v", err)
	}

	envelope := await(t, received)
	if len(envelope.SessionIDs) != 1 || envelope.SessionIDs[0] != session {
		t.Fatalf("envelope addressed to %v", envelope.SessionIDs)
	}
	if envelope.Type != "SyncPlayGroupUpdate" {
		t.Fatalf("envelope type is %q", envelope.Type)
	}

	var data map[string]string
	if err := json.Unmarshal(envelope.Data, &data); err != nil {
		t.Fatalf("failed to decode the data: %v", err)
	}
	if data["Type"] != "UserJoined" {
		t.Fatalf("data is %v", data)
	}
}

// The pod that publishes also holds connections, so it has to receive its own
// envelope rather than assume it already delivered it.
func TestPublishReachesTheSendingReplica(t *testing.T) {
	service := newService(t)
	received := receive(t, service)

	session := uuid.New()
	if err := service.Publish(context.Background(), []uuid.UUID{session}, "SyncPlayGroupUpdate", "payload"); err != nil {
		t.Fatalf("failed to publish: %v", err)
	}

	if envelope := await(t, received); envelope.SessionIDs[0] != session {
		t.Fatalf("envelope addressed to %v", envelope.SessionIDs)
	}
}

func TestPublishWithoutSessionsIsANoop(t *testing.T) {
	service := newService(t)
	received := receive(t, service)

	if err := service.Publish(context.Background(), nil, "SyncPlayGroupUpdate", "payload"); err != nil {
		t.Fatalf("failed to publish: %v", err)
	}

	select {
	case envelope := <-received:
		t.Fatalf("published %+v for nobody", envelope)
	case <-time.After(time.Second):
	}
}
