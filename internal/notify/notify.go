package notify

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"slices"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/FreekingDean/gojellyfin/internal/env"
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
	pool *pgxpool.Pool

	mu       sync.RWMutex
	handlers []Handler

	cancel context.CancelFunc
	done   chan struct{}
}

func New(config env.Config) (*Service, error) {
	pool, err := pgxpool.New(context.Background(), config.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to open the notify pool: %w", err)
	}

	return &Service{pool: pool, done: make(chan struct{})}, nil
}

func (s *Service) Handle(handler Handler) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.handlers = append(s.handlers, handler)
}

func (s *Service) Publish(ctx context.Context, sessionIDs []uuid.UUID, messageType string, data any) error {
	if len(sessionIDs) == 0 {
		return nil
	}

	encoded, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to encode %s data: %w", messageType, err)
	}

	for start := 0; start < len(sessionIDs); {
		size := len(sessionIDs) - start

		var payload []byte
		for {
			payload, err = json.Marshal(Envelope{
				SessionIDs: sessionIDs[start : start+size],
				Type:       messageType,
				Data:       encoded,
			})
			if err != nil {
				return fmt.Errorf("failed to encode %s envelope: %w", messageType, err)
			}
			if len(payload) <= maxPayload || size == 1 {
				break
			}

			size /= 2
		}

		if len(payload) > maxPayload {
			return fmt.Errorf("a single %s recipient needs %d bytes, over the %d a notification allows", messageType, len(payload), maxPayload)
		}

		if _, err := s.pool.Exec(ctx, "select pg_notify($1, $2)", channel, string(payload)); err != nil {
			return fmt.Errorf("failed to publish %s: %w", messageType, err)
		}

		start += size
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
	s.pool.Close()

	return nil
}

func (s *Service) listen(ctx context.Context) (*pgxpool.Conn, error) {
	conn, err := s.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to acquire a listener: %w", err)
	}

	if _, err := conn.Exec(ctx, "listen "+channel); err != nil {
		conn.Release()

		return nil, fmt.Errorf("failed to listen on %s: %w", channel, err)
	}

	return conn, nil
}

func (s *Service) run(ctx context.Context, conn *pgxpool.Conn) {
	defer close(s.done)

	var lost time.Time

	for {
		if conn != nil {
			err := s.receive(ctx, conn)
			conn.Release()

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

func (s *Service) receive(ctx context.Context, conn *pgxpool.Conn) error {
	for {
		notification, err := conn.Conn().WaitForNotification(ctx)
		if err != nil {
			return fmt.Errorf("failed to read a notification: %w", err)
		}

		var envelope Envelope
		if err := json.Unmarshal([]byte(notification.Payload), &envelope); err != nil {
			log.Printf("notify listener: undecodable payload: %v", err)
			continue
		}

		s.dispatch(envelope)
	}
}

func (s *Service) dispatch(envelope Envelope) {
	s.mu.RLock()
	handlers := slices.Clone(s.handlers)
	s.mu.RUnlock()

	for _, handler := range handlers {
		handler(envelope)
	}
}
