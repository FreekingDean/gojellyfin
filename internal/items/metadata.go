package items

import (
	"context"
	"fmt"
	"slices"
	"time"

	"entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/consts"
	"github.com/FreekingDean/gojellyfin/internal/store"
	itemmodal "github.com/FreekingDean/gojellyfin/internal/store/item"
)

const LockedName = "Name"

var titleColumns = []string{
	itemmodal.FieldName,
	itemmodal.FieldSortName,
	itemmodal.FieldProductionYear,
}

type Metadata struct {
	Name              *string
	SortName          *string
	Overview          *string
	OfficialRating    *consts.Rating
	CommunityRating   *float64
	ProductionYear    *int32
	PremiereDate      *time.Time
	EndDate           *time.Time
	IndexNumber       *int32
	ParentIndexNumber *int32
	Status            *string
	RunTimeTicks      *int64
	LockData          *bool
	Tags              *[]string
	Taglines          *[]string
	Genres            *[]string
	Studios           *[]string
	People            *[]Credit
	LockedFields      *[]string
	ProviderIds       *map[string]string
	Images            []RemoteImage
}

func (s *Service) UpdateMetadata(ctx context.Context, id uuid.UUID, metadata Metadata) (*Item, error) {
	update := s.store.Item.UpdateOneID(id).
		SetNillableName(metadata.Name).
		SetNillableSortName(metadata.SortName).
		SetNillableOverview(metadata.Overview).
		SetNillableOfficialRating((*string)(metadata.OfficialRating)).
		SetNillableCommunityRating(metadata.CommunityRating).
		SetNillableProductionYear(metadata.ProductionYear).
		SetNillablePremiereDate(metadata.PremiereDate).
		SetNillableEndDate(metadata.EndDate).
		SetNillableIndexNumber(metadata.IndexNumber).
		SetNillableParentIndexNumber(metadata.ParentIndexNumber).
		SetNillableStatus(metadata.Status).
		SetNillableRunTimeTicks(metadata.RunTimeTicks).
		SetNillableLockData(metadata.LockData)

	if metadata.Tags != nil {
		update.SetTags(*metadata.Tags)
	}
	if metadata.Taglines != nil {
		update.SetTaglines(*metadata.Taglines)
	}
	if metadata.LockedFields != nil {
		update.SetLockedFields(*metadata.LockedFields)
	}
	if metadata.ProviderIds != nil {
		update.SetProviderIds(*metadata.ProviderIds)
	}

	item, err := update.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to update item metadata: %w", err)
	}

	if err := s.replaceNamed(ctx, id, metadata); err != nil {
		return nil, err
	}

	return item, nil
}

type Credit struct {
	Name  string
	Kind  CreditKind
	Role  string
	Order int32
}

func (s *Service) replaceNamed(ctx context.Context, id uuid.UUID, metadata Metadata) error {
	if metadata.Genres == nil && metadata.Studios == nil && metadata.People == nil {
		return nil
	}

	return s.store.WithTx(ctx, func(tx *store.Tx) error {
		update := tx.Item.UpdateOneID(id)

		if metadata.Genres != nil {
			genres, err := genreIDs(ctx, tx, *metadata.Genres)
			if err != nil {
				return err
			}
			update = update.ClearGenres().AddGenreIDs(genres...)
		}
		if metadata.Studios != nil {
			studios, err := studioIDs(ctx, tx, *metadata.Studios)
			if err != nil {
				return err
			}
			update = update.ClearStudios().AddStudioIDs(studios...)
		}

		if err := update.Exec(ctx); err != nil {
			return err
		}

		if metadata.People != nil {
			return replaceCredits(ctx, tx, id, *metadata.People)
		}

		return nil
	})
}

func unclaimedTitle(upsert *store.ItemUpsert) {
	kept := upsert.Table()
	excluded := sql.Dialect(upsert.Dialect()).Table("excluded")
	unclaimed := fmt.Sprintf(
		"%s IS NULL AND NOT %s AND NOT COALESCE(%s @> '[%q]', false)",
		kept.C(itemmodal.FieldProviderIds),
		kept.C(itemmodal.FieldLockData),
		kept.C(itemmodal.FieldLockedFields),
		LockedName,
	)

	for _, column := range titleColumns {
		upsert.Set(column, sql.Expr(fmt.Sprintf(
			"CASE WHEN %s THEN %s ELSE %s END", unclaimed, excluded.C(column), kept.C(column),
		)))
	}
}

func (s *Service) EditMetadata(ctx context.Context, item *Item, metadata Metadata) (*Item, error) {
	if retitled(item, metadata) {
		locked := item.LockedFields
		if metadata.LockedFields != nil {
			locked = *metadata.LockedFields
		}
		if !slices.Contains(locked, LockedName) {
			locked = append(slices.Clone(locked), LockedName)
		}
		metadata.LockedFields = &locked
	}

	return s.UpdateMetadata(ctx, item.ID, metadata)
}

func retitled(item *Item, metadata Metadata) bool {
	if metadata.Name != nil && *metadata.Name != item.Name {
		return true
	}
	if metadata.SortName != nil && *metadata.SortName != item.SortName {
		return true
	}
	if metadata.ProductionYear == nil {
		return false
	}

	return item.ProductionYear == nil || *metadata.ProductionYear != *item.ProductionYear
}
