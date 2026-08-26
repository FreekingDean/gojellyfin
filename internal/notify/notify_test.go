package notify

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/env"
)

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

	received := make(chan Envelope, 64)
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

func TestService_Publish(t *testing.T) {
	t.Run("an envelope reaches a listener on another pod", func(t *testing.T) {
		publisher := newService(t)
		subscriber := newService(t)
		received := receive(t, subscriber)

		session := uuid.New()
		if err := publisher.Publish(context.Background(), []uuid.UUID{session}, "SyncPlayGroupUpdate", map[string]string{"Type": "UserJoined"}); err != nil {
			t.Fatalf("failed to publish: %v", err)
		}

		envelope := await(t, received)
		if len(envelope.SessionIDs) != 1 || envelope.SessionIDs[0] != session {
			t.Fatalf("envelope addressed to %v, want %v", envelope.SessionIDs, session)
		}
		if envelope.Type != "SyncPlayGroupUpdate" {
			t.Fatalf("envelope type = %q, want SyncPlayGroupUpdate", envelope.Type)
		}

		var data map[string]string
		if err := json.Unmarshal(envelope.Data, &data); err != nil {
			t.Fatalf("failed to decode the data: %v", err)
		}
		if data["Type"] != "UserJoined" {
			t.Fatalf("data = %v, want UserJoined", data)
		}
	})

	t.Run("the pod that published holds connections too, so it receives its own envelope", func(t *testing.T) {
		service := newService(t)
		received := receive(t, service)

		session := uuid.New()
		if err := service.Publish(context.Background(), []uuid.UUID{session}, "SyncPlayGroupUpdate", "payload"); err != nil {
			t.Fatalf("failed to publish: %v", err)
		}

		if envelope := await(t, received); envelope.SessionIDs[0] != session {
			t.Fatalf("envelope addressed to %v, want %v", envelope.SessionIDs, session)
		}
	})

	t.Run("no recipients publishes nothing", func(t *testing.T) {
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
	})

	t.Run("a group too large for one notification is split across several", func(t *testing.T) {
		service := newService(t)
		received := receive(t, service)

		recipients := make([]uuid.UUID, 400)
		for i := range recipients {
			recipients[i] = uuid.New()
		}

		if err := service.Publish(context.Background(), recipients, "SyncPlayGroupUpdate", "payload"); err != nil {
			t.Fatalf("failed to publish to a large group: %v", err)
		}

		var delivered []uuid.UUID
		for len(delivered) < len(recipients) {
			delivered = append(delivered, await(t, received).SessionIDs...)
		}

		if len(delivered) != len(recipients) {
			t.Fatalf("delivered %d recipients, want %d", len(delivered), len(recipients))
		}
		for i, id := range recipients {
			if delivered[i] != id {
				t.Fatalf("recipient %d = %v, want %v", i, delivered[i], id)
			}
		}
	})

	t.Run("data too large for a single recipient is refused rather than silently dropped", func(t *testing.T) {
		service := newService(t)

		err := service.Publish(context.Background(), []uuid.UUID{uuid.New()}, "SyncPlayGroupUpdate", string(make([]byte, maxPayload)))
		if err == nil {
			t.Fatal("want an error for an oversized payload, got nil")
		}
	})
}
