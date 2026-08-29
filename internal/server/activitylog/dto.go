package activitylog

import (
	"github.com/FreekingDean/gojellyfin/internal/activity"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
)

func entryDto(entry *activity.Entry) api.ActivityLogEntry {
	dto := api.ActivityLogEntry{
		Date:     apiutil.Ptr(entry.CreatedAt),
		Name:     apiutil.Ptr(entry.Name),
		Type:     apiutil.Ptr(entry.Kind),
		Severity: apiutil.Ptr(api.LogLevel(entry.Severity)),
	}

	if entry.Overview != "" {
		dto.Overview = apiutil.Ptr(entry.Overview)
	}
	if entry.ShortOverview != "" {
		dto.ShortOverview = apiutil.Ptr(entry.ShortOverview)
	}
	if user := entry.Edges.User; user != nil {
		dto.UserId = &user.ID
	}
	if item := entry.Edges.Item; item != nil {
		dto.ItemId = apiutil.Ptr(item.ID.String())
	}

	return dto
}
