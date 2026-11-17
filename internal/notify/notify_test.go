package notify

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/FreekingDean/gojellyfin/internal/storetest"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/env"
)

func newService(t *testing.T) (*Service, chan Envelope) {
	t.Helper()

	config, err := env.Load()
	if err != nil {
		t.Fatalf("failed to read the environment: %v", err)
	}

	connection := storetest.Open(t)

	received := make(chan Envelope, 64)
	service := New(config, connection.Client(), func(envelope Envelope) {
		select {
		case received <- envelope:
		default:
		}
	})
	if err := service.Start(); err != nil {
		t.Fatalf("failed to start the listener: %v", err)
	}

	t.Cleanup(func() {
	})

	return service, received
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
	t.Run("an envelope reaches a listener on another pod", anEnvelopeReachesAListenerOnAnotherPod())

	t.Run("the pod that published holds connections too, so it receives its own envelope", func(t *testing.T) {
		service, received := newService(t)

		session := uuid.New()
		if err := service.Publish(context.Background(), []uuid.UUID{session}, "SyncPlayGroupUpdate", "payload"); err != nil {
			t.Fatalf("failed to publish: %v", err)
		}

		if envelope := await(t, received); envelope.SessionIDs[0] != session {
			t.Fatalf("envelope addressed to %v, want %v", envelope.SessionIDs, session)
		}
	})

	t.Run("no recipients publishes nothing", func(t *testing.T) {
		service, received := newService(t)

		if err := service.Publish(context.Background(), nil, "SyncPlayGroupUpdate", "payload"); err != nil {
			t.Fatalf("failed to publish: %v", err)
		}

		select {
		case envelope := <-received:
			t.Fatalf("published %+v for nobody", envelope)
		case <-time.After(time.Second):
		}
	})

	t.Run("an envelope too large for one notification is refused rather than silently dropped", func(t *testing.T) {
		service, _ := newService(t)

		err := service.Publish(context.Background(), []uuid.UUID{uuid.New()}, "SyncPlayGroupUpdate", string(make([]byte, maxPayload)))
		if err == nil {
			t.Fatal("want an error for an oversized payload, got nil")
		}
	})
}

func anEnvelopeReachesAListenerOnAnotherPod() func(*testing.T) {
	return func(t *testing.T) {
		publisher, _ := newService(t)
		_, received := newService(t)

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
	}
}

func TestService_OnReconnect(t *testing.T) {
	service, received := newService(t)

	reconnected := make(chan struct{}, 1)
	service.OnReconnect(func(context.Context) {
		reconnected <- struct{}{}
	})

	if _, err := service.store.ExecContext(context.Background(), "select pg_terminate_backend($1)", int(service.backend.Load())); err != nil {
		t.Fatalf("failed to terminate the listener: %v", err)
	}

	select {
	case <-reconnected:
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for the listener to reconnect")
	}

	session := uuid.New()
	if err := service.Publish(context.Background(), []uuid.UUID{session}, "SyncPlayGroupUpdate", "payload"); err != nil {
		t.Fatalf("failed to publish: %v", err)
	}
	if envelope := await(t, received); envelope.SessionIDs[0] != session {
		t.Fatalf("envelope addressed to %v, want %v", envelope.SessionIDs, session)
	}
}
