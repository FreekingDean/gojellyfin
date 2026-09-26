package notify

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/FreekingDean/gojellyfin/internal/env"
	"github.com/FreekingDean/gojellyfin/internal/store"
)

const (
	channel       = "gojellyfin_socket"
	retryInterval = time.Second
	maxPayload    = 7500
)

type Envelope struct {
	SessionIDs []uuid.UUID     `json:"SessionIds"`
	Type       string          `json:"Type"`
	Data       json.RawMessage `json:"Data"`
}

type Handler func(Envelope)

type Service struct {
	store       *store.Client
	databaseURL string
	handler     Handler

	cancel context.CancelFunc
	done   chan struct{}
}

func New(config env.Config, client *store.Client, handler Handler) *Service {
	return &Service{store: client, databaseURL: config.DatabaseURL, handler: handler, done: make(chan struct{})}
}

func (s *Service) Publish(ctx context.Context, sessionIDs []uuid.UUID, messageType string, data any) error {
	if len(sessionIDs) == 0 {
		return nil
	}

	encoded, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to encode %s data: %w", messageType, err)
	}

	payload, err := json.Marshal(Envelope{SessionIDs: sessionIDs, Type: messageType, Data: encoded})
	if err != nil {
		return fmt.Errorf("failed to encode %s envelope: %w", messageType, err)
	}
	if len(payload) > maxPayload {
		return fmt.Errorf("%s for %d sessions needs %d bytes, over the %d a notification allows", messageType, len(sessionIDs), len(payload), maxPayload)
	}

	if _, err := s.store.ExecContext(ctx, "select pg_notify($1, $2)", channel, string(payload)); err != nil {
		return fmt.Errorf("failed to publish %s: %w", messageType, err)
	}

	return nil
}

func (s *Service) Start() error {
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel

	conn, err := s.listen(ctx)
	if err != nil {
		cancel()
		close(s.done)

		return err
	}

	go s.run(ctx, conn)

	return nil
}

func (s *Service) Stop() error {
	s.cancel()
	<-s.done

	return nil
}

func (s *Service) listen(ctx context.Context) (*pgx.Conn, error) {
	conn, err := pgx.Connect(ctx, s.databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect a listener: %w", err)
	}

	if _, err := conn.Exec(ctx, "listen "+channel); err != nil {
		_ = conn.Close(context.Background())

		return nil, fmt.Errorf("failed to listen on %s: %w", channel, err)
	}

	return conn, nil
}

func (s *Service) run(ctx context.Context, conn *pgx.Conn) {
	defer close(s.done)

	var lost time.Time

	for {
		if conn != nil {
			err := s.receive(ctx, conn)
			_ = conn.Close(context.Background())

			if ctx.Err() != nil {
				return
			}

			lost = time.Now()
			log.Printf("notify listener lost, updates for sessions on this pod are dropped until it reconnects: %v", err)
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(retryInterval):
		}

		next, err := s.listen(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}

			conn = nil
			log.Printf("notify listener cannot reconnect: %v", err)

			continue
		}

		log.Printf("notify listener reconnected after %s, updates published in that window were missed", time.Since(lost).Round(time.Millisecond))

		conn = next
	}
}

func (s *Service) receive(ctx context.Context, conn *pgx.Conn) error {
	for {
		notification, err := conn.WaitForNotification(ctx)
		if err != nil {
			return fmt.Errorf("failed to read a notification: %w", err)
		}

		var envelope Envelope
		if err := json.Unmarshal([]byte(notification.Payload), &envelope); err != nil {
			log.Printf("notify listener: undecodable payload: %v", err)
			continue
		}

		s.handler(envelope)
	}
}
