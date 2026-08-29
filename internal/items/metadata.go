package items

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/consts"
)

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
