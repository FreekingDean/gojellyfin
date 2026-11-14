package activitylog

import (
	"github.com/FreekingDean/gojellyfin/internal/activity"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
)

func entryDto(entry *activity.Entry) api.ActivityLogEntry {
	dto := api.ActivityLogEntry{
		Date:          apiutil.Ptr(entry.CreatedAt),
		Name:          apiutil.Ptr(entry.Name),
		Type:          apiutil.Ptr(entry.Kind),
		Severity:      apiutil.Ptr(api.LogLevel(entry.Severity)),
		UserId:        entry.UserID,
		Overview:      apiutil.ZeroOrNilPtr(entry.Overview),
		ShortOverview: apiutil.ZeroOrNilPtr(entry.ShortOverview),
	}

	if entry.ItemID != nil {
		dto.ItemId = apiutil.Ptr(entry.ItemID.String())
	}

	return dto
}

func query(params api.GetLogEntriesParams) activity.Query {
	converted := activity.Query{
		StartIndex:    int(apiutil.Deref(params.StartIndex)),
		Limit:         int(apiutil.Deref(params.Limit)),
		MinDate:       params.MinDate,
		MaxDate:       params.MaxDate,
		HasUserID:     params.HasUserId,
		Name:          apiutil.Deref(params.Name),
		Overview:      apiutil.Deref(params.Overview),
		ShortOverview: apiutil.Deref(params.ShortOverview),
		Kind:          apiutil.Deref(params.Type),
		Username:      apiutil.Deref(params.Username),
		ItemID:        params.ItemId,
	}
	if params.Severity != nil {
		converted.Severity = apiutil.Ptr(activity.Severity(*params.Severity))
	}

	orders := apiutil.Deref(params.SortOrder)
	for i, sortBy := range apiutil.Deref(params.SortBy) {
		field, ok := sortFields[sortBy]
		if !ok {
			continue
		}
		converted.Sort = append(converted.Sort, activity.Sort{
			Field:      field,
			Descending: i < len(orders) && orders[i] == api.Descending,
		})
	}

	return converted
}

var sortFields = map[api.ActivityLogSortBy]activity.SortField{
	api.ActivityLogSortByDateCreated:   activity.SortByCreated,
	api.ActivityLogSortByLogSeverity:   activity.SortBySeverity,
	api.ActivityLogSortByName:          activity.SortByName,
	api.ActivityLogSortByOveriew:       activity.SortByOverview,
	api.ActivityLogSortByShortOverview: activity.SortByShortOverview,
	api.ActivityLogSortByType:          activity.SortByKind,
	api.ActivityLogSortByUsername:      activity.SortByUsername,
}
