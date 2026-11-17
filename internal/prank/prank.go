package prank

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/store"
	itemmodel "github.com/FreekingDean/gojellyfin/internal/store/item"
)

const key = "prank"

var ErrInvalid = errors.New("invalid playback override")

var disguisable = []items.Kind{itemmodel.KindMovie, itemmodel.KindEpisode}

type Override struct {
	Enabled    bool        `json:"enabled"`
	ItemID     *uuid.UUID  `json:"item_id,omitempty"`
	From       *time.Time  `json:"from,omitempty"`
	Until      *time.Time  `json:"until,omitempty"`
	UserIDs    []uuid.UUID `json:"user_ids,omitempty"`
	StartTicks int64       `json:"start_ticks,omitempty"`
}

type Catalogue interface {
	ItemByID(ctx context.Context, viewer items.Viewer, id uuid.UUID) (*items.Item, error)
}

type Settings interface {
	Configuration(ctx context.Context, key string) (json.RawMessage, error)
	SetConfiguration(ctx context.Context, key string, value json.RawMessage) error
}

type Service struct {
	catalogue Catalogue
	settings  Settings
	now       func() time.Time
}

func newService(catalogue Catalogue, settings Settings, now func() time.Time) *Service {
	return &Service{catalogue: catalogue, settings: settings, now: now}
}

func AprilFirst(now time.Time) (time.Time, time.Time) {
	year := now.Year()
	if now.Month() > time.April || (now.Month() == time.April && now.Day() > 1) {
		year++
	}
	from := time.Date(year, time.April, 1, 0, 0, 0, 0, now.Location())

	return from, from.AddDate(0, 0, 1)
}

func (s *Service) Override(ctx context.Context) (Override, error) {
	stored, err := s.settings.Configuration(ctx, key)
	if err != nil {
		return Override{}, fmt.Errorf("failed to read the playback override: %w", err)
	}
	if stored == nil {
		from, until := AprilFirst(s.now())

		return Override{From: &from, Until: &until}, nil
	}

	var override Override
	if err := json.Unmarshal(stored, &override); err != nil {
		return Override{}, fmt.Errorf("failed to decode the playback override: %w", err)
	}

	return override, nil
}

func (s *Service) Save(ctx context.Context, override Override) error {
	if override.Enabled && override.ItemID == nil {
		return fmt.Errorf("%w: pick the title to play before turning it on", ErrInvalid)
	}
	if override.From != nil && override.Until != nil && !override.Until.After(*override.From) {
		return fmt.Errorf("%w: the window ends before it starts", ErrInvalid)
	}
	if override.StartTicks < 0 {
		return fmt.Errorf("%w: the start time is before the beginning", ErrInvalid)
	}
	if override.ItemID != nil {
		item, err := s.catalogue.ItemByID(ctx, items.Everyone, *override.ItemID)
		if store.IsNotFound(err) {
			return fmt.Errorf("%w: no title has that id", ErrInvalid)
		}
		if err != nil {
			return err
		}
		if !slices.Contains(disguisable, item.Kind) {
			return fmt.Errorf("%w: only a movie or an episode can stand in", ErrInvalid)
		}
		if item.RunTimeTicks != nil && override.StartTicks >= *item.RunTimeTicks {
			return fmt.Errorf("%w: the start time is after the title ends", ErrInvalid)
		}
	}

	encoded, err := json.Marshal(override)
	if err != nil {
		return err
	}

	return s.settings.SetConfiguration(ctx, key, encoded)
}

func (o Override) live(now time.Time) bool {
	if !o.Enabled || o.ItemID == nil {
		return false
	}
	if o.From != nil && now.Before(*o.From) {
		return false
	}

	return o.Until == nil || now.Before(*o.Until)
}

func (s *Service) Swap(ctx context.Context, userID uuid.UUID, item *items.Item) (*items.Item, bool, error) {
	if !slices.Contains(disguisable, item.Kind) {
		return item, false, nil
	}

	target, ok, err := s.target(ctx, userID, item.ID)
	if err != nil || !ok {
		return item, false, err
	}

	standIn, err := s.catalogue.ItemByID(ctx, items.Everyone, target)
	if store.IsNotFound(err) {
		return item, false, nil
	}
	if err != nil {
		return item, false, err
	}

	return standIn, true, nil
}

func (s *Service) Start(ctx context.Context) (int64, error) {
	override, err := s.Override(ctx)
	if err != nil {
		return 0, err
	}

	return override.StartTicks, nil
}

func (s *Service) Disguises(ctx context.Context, userID, itemID uuid.UUID) (bool, error) {
	_, ok, err := s.target(ctx, userID, itemID)
	if err != nil || !ok {
		return false, err
	}

	item, err := s.catalogue.ItemByID(ctx, items.Everyone, itemID)
	if store.IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	return slices.Contains(disguisable, item.Kind), nil
}

func (s *Service) target(ctx context.Context, userID, itemID uuid.UUID) (uuid.UUID, bool, error) {
	override, err := s.Override(ctx)
	if err != nil {
		return uuid.Nil, false, err
	}
	if !override.live(s.now()) || *override.ItemID == itemID {
		return uuid.Nil, false, nil
	}
	if len(override.UserIDs) > 0 && !slices.Contains(override.UserIDs, userID) {
		return uuid.Nil, false, nil
	}

	return *override.ItemID, true, nil
}

func (s *Service) Name(ctx context.Context, override Override) (string, error) {
	if override.ItemID == nil {
		return "", nil
	}

	item, err := s.catalogue.ItemByID(ctx, items.Everyone, *override.ItemID)
	if store.IsNotFound(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}

	return item.Name, nil
}
