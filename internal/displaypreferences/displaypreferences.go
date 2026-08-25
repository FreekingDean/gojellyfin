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

func (s *Service) Get(ctx context.Context, userID uuid.UUID, refID uuid.UUID, client string) (*DisplayPreferences, error) {
	prefs, err := s.store.DisplayPreferences.Query().Where(
		displaypreferencesmodal.Client(client),
		displaypreferencesmodal.HasUserWith(usermodal.ID(userID)),
		displaypreferencesmodal.HasItemWith(itemmodal.ID(refID)),
	).First(ctx)
	if store.IsNotFound(err) {
		return &DisplayPreferences{
			Edges: store.DisplayPreferencesEdges{
				User: &store.User{ID: userID},
				Item: &store.Item{ID: refID},
			},
			Client: client,
		}, nil
	} else if err != nil {
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

func (s *Service) Update(
	ctx context.Context,
	userID uuid.UUID,
	refID uuid.UUID,
	client string,
	settings Settings,
) error {
	upsert := s.store.DisplayPreferences.Create().
		SetClient(client).
		SetUserID(userID).
		SetItemID(refID).
		OnConflictColumns(
			displaypreferencesmodal.FieldClient,
			displaypreferencesmodal.EdgeUser,
			displaypreferencesmodal.EdgeItem,
		)

	if settings.ViewType != nil {
		upsert.SetViewType(*settings.ViewType)
	}

	if settings.SortBy != nil {
		upsert.SetSortBy(*settings.SortBy)
	}

	if settings.IndexBy != nil {
		upsert.SetIndexBy(*settings.IndexBy)
	}

	if settings.SortOrder != nil {
		upsert.SetSortOrder(*settings.SortOrder)
	}

	if settings.ScrollDirection != nil {
		upsert.SetScrollDirection(*settings.ScrollDirection)
	}

	if settings.RememberIndexing != nil {
		upsert.SetRememberIndexing(*settings.RememberIndexing)
	}

	if settings.RememberSorting != nil {
		upsert.SetRememberSorting(*settings.RememberSorting)
	}

	if settings.ShowBackdrop != nil {
		upsert.SetShowBackdrop(*settings.ShowBackdrop)
	}

	if settings.ShowSidebar != nil {
		upsert.SetShowSidebar(*settings.ShowSidebar)
	}

	if settings.PrimaryImageHeight != nil {
		upsert.SetPrimaryImageHeight(*settings.PrimaryImageHeight)
	}

	if settings.PrimaryImageWidth != nil {
		upsert.SetPrimaryImageWidth(*settings.PrimaryImageWidth)
	}

	if settings.CustomPrefs != nil {
		upsert.SetCustomPrefs(*settings.CustomPrefs)
	}

	return upsert.Exec(ctx)
}
