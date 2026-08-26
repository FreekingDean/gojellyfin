package displaypreferences

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/store"
	displaypreferencesmodal "github.com/FreekingDean/gojellyfin/internal/store/displaypreferences"
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

func (s *Service) Get(ctx context.Context, userID uuid.UUID, referenceID, client string) (*DisplayPreferences, error) {
	prefs, err := s.store.DisplayPreferences.Query().Where(
		displaypreferencesmodal.UserID(userID),
		displaypreferencesmodal.ReferenceID(referenceID),
		displaypreferencesmodal.Client(client),
	).First(ctx)
	if store.IsNotFound(err) {
		return defaults(userID, referenceID, client), nil
	} else if err != nil {
		return nil, fmt.Errorf("failed to query display preferences: %w", err)
	}

	return prefs, nil
}

func defaults(userID uuid.UUID, referenceID, client string) *DisplayPreferences {
	return &DisplayPreferences{
		UserID:             userID,
		ReferenceID:        referenceID,
		Client:             client,
		SortBy:             displaypreferencesmodal.DefaultSortBy,
		SortOrder:          displaypreferencesmodal.DefaultSortOrder,
		ScrollDirection:    displaypreferencesmodal.DefaultScrollDirection,
		RememberIndexing:   displaypreferencesmodal.DefaultRememberIndexing,
		RememberSorting:    displaypreferencesmodal.DefaultRememberSorting,
		ShowBackdrop:       displaypreferencesmodal.DefaultShowBackdrop,
		ShowSidebar:        displaypreferencesmodal.DefaultShowSidebar,
		PrimaryImageHeight: displaypreferencesmodal.DefaultPrimaryImageHeight,
		PrimaryImageWidth:  displaypreferencesmodal.DefaultPrimaryImageWidth,
	}
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
	referenceID string,
	client string,
	settings Settings,
) error {
	upsert := s.store.DisplayPreferences.Create().
		SetUserID(userID).
		SetReferenceID(referenceID).
		SetClient(client).
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
		upsert.SetCustomPrefs(*settings.CustomPrefs)
	}

	return upsert.
		OnConflictColumns(
			displaypreferencesmodal.FieldUserID,
			displaypreferencesmodal.FieldReferenceID,
			displaypreferencesmodal.FieldClient,
		).
		UpdateNewValues().
		Exec(ctx)
}
