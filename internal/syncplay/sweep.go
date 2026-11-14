package syncplay

import (
	"context"
	"fmt"
	"log"
	"time"

	entsql "entgo.io/ent/dialect/sql"

	"github.com/FreekingDean/gojellyfin/internal/store"
	groupmodal "github.com/FreekingDean/gojellyfin/internal/store/syncplaygroup"
)

const (
	sweepInterval   = time.Second
	sweepBatch      = 20
	collectInterval = time.Minute
	collectBatch    = 100
)

type Sweeper struct {
	service *Service
	cancel  context.CancelFunc
	done    chan struct{}
}

func NewSweeper(service *Service) *Sweeper {
	return &Sweeper{service: service, done: make(chan struct{})}
}

func (w *Sweeper) Start() error {
	ctx, cancel := context.WithCancel(context.Background())
	w.cancel = cancel

	go w.run(ctx)

	return nil
}

func (w *Sweeper) Stop() error {
	w.cancel()
	<-w.done

	return nil
}

func (w *Sweeper) run(ctx context.Context) {
	defer close(w.done)

	ticker := time.NewTicker(sweepInterval)
	defer ticker.Stop()

	collector := time.NewTicker(collectInterval)
	defer collector.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := w.service.ExpireWaits(ctx); err != nil && ctx.Err() == nil {
				log.Printf("syncplay: failed to expire waiting groups: %v", err)
			}
		case <-collector.C:
			if _, err := w.service.CollectEmpty(ctx); err != nil && ctx.Err() == nil {
				log.Printf("syncplay: failed to collect empty groups: %v", err)
			}
		}
	}
}

func (s *Service) CollectEmpty(ctx context.Context) (int, error) {
	var collected int

	err := s.store.WithTx(ctx, func(tx *store.Tx) error {
		ids, err := tx.SyncPlayGroup.Query().
			Where(groupmodal.Not(groupmodal.HasMembers())).
			Limit(collectBatch).
			ForUpdate(entsql.WithLockAction(entsql.SkipLocked)).
			IDs(ctx)
		if err != nil || len(ids) == 0 {
			return err
		}

		collected, err = tx.SyncPlayGroup.Delete().
			Where(groupmodal.IDIn(ids...), groupmodal.Not(groupmodal.HasMembers())).
			Exec(ctx)

		return err
	})
	if err != nil {
		return 0, fmt.Errorf("failed to collect empty syncplay groups: %w", err)
	}

	return collected, nil
}

func (s *Service) ExpireWaits(ctx context.Context) error {
	var messages []Outgoing

	err := s.store.WithTx(ctx, func(tx *store.Tx) error {
		now := s.now()

		ids, err := tx.SyncPlayGroup.Query().
			Where(groupmodal.WaitDeadlineLTE(now)).
			Limit(sweepBatch).
			ForUpdate(entsql.WithLockAction(entsql.SkipLocked)).
			IDs(ctx)
		if err != nil {
			return err
		}

		for _, id := range ids {
			room, err := lockRoom(ctx, tx, id)
			if err != nil {
				return err
			}

			room.catalogue = known{}
			room.now = now
			room.timeout()
			messages = append(messages, room.out...)

			if err := saveRoom(ctx, tx, room); err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return err
	}

	s.Publish(ctx, messages)

	return nil
}

func (s *Service) Publish(ctx context.Context, messages []Outgoing) {
	for _, message := range messages {
		if err := s.publisher.Publish(ctx, message.SessionIDs, message.Type, message.Data); err != nil {
			log.Printf("syncplay: failed to publish %s: %v", message.Type, err)
		}
	}
}
