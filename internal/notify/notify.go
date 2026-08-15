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
)

// Every pod receives every envelope, so the sessions it is meant for travel in
// the payload: a pod drops the ones it does not hold without asking the
// database who was in the group.
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

// LISTEN holds a connection for as long as it is listening, so this owns a pool
// of its own rather than borrowing the one every query shares.
func New(config env.Config) (*Service, error) {
	pool, err := pgxpool.New(context.Background(), config.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to open the notify pool: %w", err)
	}

	return &Service{pool: pool, done: make(chan struct{})}, nil
}

// Handlers run on the listener, in order, because a group's updates only make
// sense in the order they happened. One that blocks therefore stalls delivery
// for every other handler, so a handler must return without waiting.
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

	payload, err := json.Marshal(Envelope{SessionIDs: sessionIDs, Type: messageType, Data: encoded})
	if err != nil {
		return fmt.Errorf("failed to encode %s envelope: %w", messageType, err)
	}

	if _, err := s.pool.Exec(ctx, "select pg_notify($1, $2)", channel, string(payload)); err != nil {
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
	s.pool.Close()

	return nil
}

// LISTEN is issued before Start returns, so an envelope published immediately
// after startup is not lost to a listener that has not registered yet.
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

// Losing the connection is how a dropped listener is noticed, so releasing it
// and acquiring another is the reconnect.
func (s *Service) run(ctx context.Context, conn *pgxpool.Conn) {
	defer close(s.done)

	for {
		if conn != nil {
			if err := s.receive(ctx, conn); err != nil && ctx.Err() == nil {
				log.Printf("notify listener: %v", err)
			}
			conn.Release()
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(retryInterval):
		}

		next, err := s.listen(ctx)
		if err != nil {
			if ctx.Err() == nil {
				log.Printf("notify listener: %v", err)
			}
			conn = nil

			continue
		}

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
