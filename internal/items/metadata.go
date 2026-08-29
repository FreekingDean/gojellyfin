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
	Name                         *string
	OriginalTitle                *string
	SortName                     *string
	Overview                     *string
	OfficialRating               *consts.Rating
	CustomRating                 *consts.Rating
	CommunityRating              *float64
	CriticRating                 *float64
	ProductionYear               *int32
	PremiereDate                 *time.Time
	EndDate                      *time.Time
	IndexNumber                  *int32
	IndexNumberEnd               *int32
	ParentIndexNumber            *int32
	AirsBeforeSeasonNumber       *int32
	AirsAfterSeasonNumber        *int32
	AirsBeforeEpisodeNumber      *int32
	Status                       *string
	AirTime                      *string
	DisplayOrder                 *string
	LockData                     *bool
	PreferredMetadataLanguage    *string
	PreferredMetadataCountryCode *string
	AirDays                      *[]string
	Tags                         *[]string
	Taglines                     *[]string
	ProductionLocations          *[]string
	LockedFields                 *[]string
	ProviderIds                  *map[string]string
	Images                       []RemoteImage
}

func (s *Service) UpdateMetadata(ctx context.Context, id uuid.UUID, metadata Metadata) (*Item, error) {
	update := s.store.Item.UpdateOneID(id).
		SetNillableName(metadata.Name).
		SetNillableOriginalTitle(metadata.OriginalTitle).
		SetNillableSortName(metadata.SortName).
		SetNillableOverview(metadata.Overview).
		SetNillableOfficialRating((*string)(metadata.OfficialRating)).
		SetNillableCustomRating((*string)(metadata.CustomRating)).
		SetNillableCommunityRating(metadata.CommunityRating).
		SetNillableCriticRating(metadata.CriticRating).
		SetNillableProductionYear(metadata.ProductionYear).
		SetNillablePremiereDate(metadata.PremiereDate).
		SetNillableEndDate(metadata.EndDate).
		SetNillableIndexNumber(metadata.IndexNumber).
		SetNillableIndexNumberEnd(metadata.IndexNumberEnd).
		SetNillableParentIndexNumber(metadata.ParentIndexNumber).
		SetNillableAirsBeforeSeasonNumber(metadata.AirsBeforeSeasonNumber).
		SetNillableAirsAfterSeasonNumber(metadata.AirsAfterSeasonNumber).
		SetNillableAirsBeforeEpisodeNumber(metadata.AirsBeforeEpisodeNumber).
		SetNillableStatus(metadata.Status).
		SetNillableAirTime(metadata.AirTime).
		SetNillableDisplayOrder(metadata.DisplayOrder).
		SetNillableLockData(metadata.LockData).
		SetNillablePreferredMetadataLanguage(metadata.PreferredMetadataLanguage).
		SetNillablePreferredMetadataCountryCode(metadata.PreferredMetadataCountryCode)

	if metadata.AirDays != nil {
		update.SetAirDays(*metadata.AirDays)
	}
	if metadata.Tags != nil {
		update.SetTags(*metadata.Tags)
	}
	if metadata.Taglines != nil {
		update.SetTaglines(*metadata.Taglines)
	}
	if metadata.ProductionLocations != nil {
		update.SetProductionLocations(*metadata.ProductionLocations)
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

	return item, nil
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
