package displaypreferences

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/store"
	displaypreferencesmodal "github.com/FreekingDean/gojellyfin/internal/store/displaypreferences"
	itemmodal "github.com/FreekingDean/gojellyfin/internal/store/item"
	usermodal "github.com/FreekingDean/gojellyfin/internal/store/user"
)

type (
	DisplayPreferences = store.DisplayPreferences
	SortOrder          = displaypreferencesmodal.SortOrder
	ScrollDirection    = displaypreferencesmodal.ScrollDirection
)

type Service struct {
	store *store.Client
}

func New(client *store.Client) *Service {
	return &Service{store: client}
}

func (s *Service) Preferences(ctx context.Context, userID uuid.UUID, client, id string) (*DisplayPreferences, error) {
	itemID, err := s.itemID(ctx, id)
	if err != nil {
		return nil, err
	}

	prefs, err := s.existing(ctx, userID, client, itemID)
	if err != nil || prefs != nil {
		return prefs, err
	}

	prefs, err = s.store.DisplayPreferences.Create().
		SetUserID(userID).
		SetNillableItemID(itemID).
		SetClient(client).
		Save(ctx)
	if store.IsConstraintError(err) {
		if raced, requery := s.existing(ctx, userID, client, itemID); requery == nil && raced != nil {
			return raced, nil
		}
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create display preferences: %w", err)
	}

	return prefs, nil
}

func (s *Service) existing(ctx context.Context, userID uuid.UUID, client string, itemID *uuid.UUID) (*DisplayPreferences, error) {
	query := s.store.DisplayPreferences.Query().
		Where(
			displaypreferencesmodal.Client(client),
			displaypreferencesmodal.HasUserWith(usermodal.ID(userID)),
		)
	if itemID == nil {
		query = query.Where(displaypreferencesmodal.Not(displaypreferencesmodal.HasItem()))
	} else {
		query = query.Where(displaypreferencesmodal.HasItemWith(itemmodal.ID(*itemID)))
	}

	prefs, err := query.First(ctx)
	if store.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query display preferences: %w", err)
	}

	return prefs, nil
}

type Settings struct {
	ViewType           *string
	SortBy             *string
	IndexBy            *string
	SortOrder          *SortOrder
	ScrollDirection    *ScrollDirection
	RememberIndexing   *bool
	RememberSorting    *bool
	ShowBackdrop       *bool
	ShowSidebar        *bool
	PrimaryImageHeight *int32
	PrimaryImageWidth  *int32
	CustomPrefs        *map[string]string
}

func (s *Service) UpdateSettings(ctx context.Context, id uuid.UUID, settings Settings) (*DisplayPreferences, error) {
	update := s.store.DisplayPreferences.UpdateOneID(id).
		SetNillableViewType(settings.ViewType).
		SetNillableSortBy(settings.SortBy).
		SetNillableIndexBy(settings.IndexBy).
		SetNillableSortOrder(settings.SortOrder).
		SetNillableScrollDirection(settings.ScrollDirection).
		SetNillableRememberIndexing(settings.RememberIndexing).
		SetNillableRememberSorting(settings.RememberSorting).
		SetNillableShowBackdrop(settings.ShowBackdrop).
		SetNillableShowSidebar(settings.ShowSidebar).
		SetNillablePrimaryImageHeight(settings.PrimaryImageHeight).
		SetNillablePrimaryImageWidth(settings.PrimaryImageWidth)

	if settings.CustomPrefs != nil {
		update.SetCustomPrefs(*settings.CustomPrefs)
	}

	prefs, err := update.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to update display preferences: %w", err)
	}

	return prefs, nil
}

func (s *Service) itemID(ctx context.Context, id string) (*uuid.UUID, error) {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return nil, nil
	}

	exists, err := s.store.Item.Query().Where(itemmodal.ID(parsed)).Exist(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to look up the display preferences item: %w", err)
	}
	if !exists {
		return nil, nil
	}

	return &parsed, nil
}
