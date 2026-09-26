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
