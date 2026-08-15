package syncplay

import (
	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	"github.com/FreekingDean/gojellyfin/internal/syncplay"
)

// The spec models these as SyncPlayGroupUpdate and its variants, but nothing in
// it references them, so oapi-codegen prunes them and they are declared here.
const (
	groupUpdateMessage = "SyncPlayGroupUpdate"

	userJoined        = "UserJoined"
	userLeft          = "UserLeft"
	groupJoined       = "GroupJoined"
	groupLeft         = "GroupLeft"
	groupDoesNotExist = "GroupDoesNotExist"
)

type groupUpdate struct {
	GroupId uuid.UUID `json:"GroupId"`
	Type    string    `json:"Type"`
	Data    any       `json:"Data"`
}

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
		Participants:  apiutil.Ptr(participantNames(group)),
		State:         apiutil.Ptr(api.GroupStateType(group.State)),
	}
}

func participantNames(group *syncplay.Group) []string {
	participants := syncplay.Participants(group)

	names := make([]string, 0, len(participants))
	for _, participant := range participants {
		names = append(names, participant.UserName)
	}

	return names
}

func sessionIDs(participants []syncplay.Participant, except uuid.UUID) []uuid.UUID {
	ids := make([]uuid.UUID, 0, len(participants))
	for _, participant := range participants {
		if participant.SessionID == except {
			continue
		}
		ids = append(ids, participant.SessionID)
	}

	return ids
}

func userNameOf(participants []syncplay.Participant, sessionID uuid.UUID) string {
	for _, participant := range participants {
		if participant.SessionID == sessionID {
			return participant.UserName
		}
	}

	return ""
}
