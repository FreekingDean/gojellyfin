package syncplay

import (
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	"github.com/FreekingDean/gojellyfin/internal/syncplay"
)

func groupInfos(groups []*syncplay.Group) []api.GroupInfoDto {
	converted := make([]api.GroupInfoDto, 0, len(groups))
	for _, group := range groups {
		converted = append(converted, groupInfo(group))
	}

	return converted
}

func groupInfo(group *syncplay.Group) api.GroupInfoDto {
	return api.GroupInfoDto{
		GroupId:       &group.ID,
		GroupName:     apiutil.Ptr(group.Name),
		LastUpdatedAt: apiutil.Ptr(group.UpdatedAt),
		Participants:  apiutil.Ptr(syncplay.Participants(group)),
		State:         apiutil.Ptr(api.GroupStateType(group.State)),
	}
}
