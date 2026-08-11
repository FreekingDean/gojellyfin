package itemupdate

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
)

// The scanner owns date_modified and the probe owns container, run_time_ticks,
// probed_at and the media sources, so the editor never writes them.
func (s *Server) saveMetadata(ctx context.Context, id uuid.UUID, req *api.BaseItemDto) error {
	update := s.items.UpdateMetadata(id).
		SetNillableName(req.Name).
		SetNillableOriginalTitle(req.OriginalTitle).
		SetNillableSortName(req.SortName).
		SetNillableOverview(req.Overview).
		SetNillableOfficialRating(req.OfficialRating).
		SetNillableCustomRating(req.CustomRating).
		SetNillableCommunityRating(rating(req.CommunityRating)).
		SetNillableCriticRating(rating(req.CriticRating)).
		SetNillableProductionYear(req.ProductionYear).
		SetNillablePremiereDate(req.PremiereDate).
		SetNillableEndDate(req.EndDate).
		SetNillableIndexNumber(req.IndexNumber).
		SetNillableIndexNumberEnd(req.IndexNumberEnd).
		SetNillableParentIndexNumber(req.ParentIndexNumber).
		SetNillableAirsBeforeSeasonNumber(req.AirsBeforeSeasonNumber).
		SetNillableAirsAfterSeasonNumber(req.AirsAfterSeasonNumber).
		SetNillableAirsBeforeEpisodeNumber(req.AirsBeforeEpisodeNumber).
		SetNillableStatus(req.Status).
		SetNillableAirTime(req.AirTime).
		SetNillableDisplayOrder(req.DisplayOrder).
		SetNillableLockData(req.LockData).
		SetNillablePreferredMetadataLanguage(req.PreferredMetadataLanguage).
		SetNillablePreferredMetadataCountryCode(req.PreferredMetadataCountryCode)

	if req.Tags != nil {
		update.SetTags(*req.Tags)
	}
	if req.Taglines != nil {
		update.SetTaglines(*req.Taglines)
	}
	if req.ProductionLocations != nil {
		update.SetProductionLocations(*req.ProductionLocations)
	}
	if req.AirDays != nil {
		days := make([]string, 0, len(*req.AirDays))
		for _, day := range *req.AirDays {
			days = append(days, string(day))
		}
		update.SetAirDays(days)
	}
	if req.LockedFields != nil {
		fields := make([]string, 0, len(*req.LockedFields))
		for _, field := range *req.LockedFields {
			fields = append(fields, string(field))
		}
		update.SetLockedFields(fields)
	}
	if req.ProviderIds != nil {
		providers := make(map[string]string, len(*req.ProviderIds))
		for name, value := range *req.ProviderIds {
			if value != nil {
				providers[name] = *value
			}
		}
		update.SetProviderIds(providers)
	}

	return update.Exec(ctx)
}

func rating(value *float32) *float64 {
	if value == nil {
		return nil
	}

	return apiutil.Ptr(float64(*value))
}

// Content types live in the server configuration keyed by path, the same place
// Jellyfin keeps them.
func configuredContentType(server api.ServerConfiguration, path string) string {
	for _, pair := range apiutil.Deref(server.ContentTypes) {
		if strings.EqualFold(apiutil.Deref(pair.Name), path) {
			return apiutil.Deref(pair.Value)
		}
	}

	return ""
}

func contentTypeOptions() []api.NameValuePair {
	return []api.NameValuePair{
		contentTypeOption("Inherit", ""),
		contentTypeOption("Movies", api.CollectionTypeMovies),
		contentTypeOption("Music", api.CollectionTypeMusic),
		contentTypeOption("Shows", api.CollectionTypeTvshows),
		contentTypeOption("Books", api.CollectionTypeBooks),
		contentTypeOption("Home Videos", api.CollectionTypeHomevideos),
		contentTypeOption("Music Videos", api.CollectionTypeMusicvideos),
		contentTypeOption("Photos", api.CollectionTypePhotos),
	}
}

func contentTypeOption(name string, value api.CollectionType) api.NameValuePair {
	return api.NameValuePair{Name: apiutil.Ptr(name), Value: apiutil.Ptr(string(value))}
}
