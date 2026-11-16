package dto

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/channels"
	"github.com/FreekingDean/gojellyfin/internal/config"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	itemmodel "github.com/FreekingDean/gojellyfin/internal/store/item"
)

var LiveTvViewID = uuid.MustParse("c4a77e15-1e7f-4c0a-9b1d-000000000001")

const liveTvViewName = "Live TV"

func LiveTvView() api.BaseItemDto {
	collectionType := api.CollectionTypeLivetv

	return api.BaseItemDto{
		Id:                   apiutil.Ptr(LiveTvViewID),
		ServerId:             apiutil.Ptr(config.ServerID),
		Name:                 apiutil.Ptr(liveTvViewName),
		SortName:             apiutil.Ptr(strings.ToLower(liveTvViewName)),
		Type:                 apiutil.Ptr(api.BaseItemKindUserView),
		CollectionType:       &collectionType,
		DisplayPreferencesId: displayPreferencesID(LiveTvViewID),
		IsFolder:             apiutil.Ptr(true),
		LocationType:         apiutil.Ptr(api.Virtual),
		ImageTags:            &map[string]*string{},
		BackdropImageTags:    &[]string{},
	}
}

func ChannelDto(channel *channels.Channel) api.BaseItemDto {
	number := strconv.Itoa(int(channel.Number))

	return api.BaseItemDto{
		Id:                apiutil.Ptr(channel.ID),
		ServerId:          apiutil.Ptr(config.ServerID),
		Name:              apiutil.Ptr(channel.Name),
		SortName:          apiutil.Ptr(strings.ToLower(channel.Name)),
		Type:              apiutil.Ptr(api.BaseItemKindTvChannel),
		ChannelType:       apiutil.Ptr(api.TV),
		ChannelNumber:     apiutil.Ptr(number),
		Number:            apiutil.Ptr(number),
		MediaType:         apiutil.Ptr(api.MediaTypeVideo),
		IsFolder:          apiutil.Ptr(false),
		LocationType:      apiutil.Ptr(api.Remote),
		PlayAccess:        apiutil.Ptr(api.PlayAccessFull),
		ImageTags:         &map[string]*string{},
		BackdropImageTags: &[]string{},
	}
}

func ChannelDtos(ctx context.Context, catalogue *items.Service, listed []*channels.Channel, airing map[uuid.UUID]channels.Program) ([]api.BaseItemDto, error) {
	current := make([]channels.Program, 0, len(airing))
	for _, channel := range listed {
		if program, found := airing[channel.ID]; found {
			current = append(current, program)
		}
	}

	programs, err := ProgramDtos(ctx, catalogue, current)
	if err != nil {
		return nil, err
	}
	byChannel := make(map[uuid.UUID]api.BaseItemDto, len(programs))
	for i, program := range current {
		byChannel[program.Channel.ID] = programs[i]
	}

	converted := make([]api.BaseItemDto, len(listed))
	for i, channel := range listed {
		converted[i] = ChannelDto(channel)
		if program, found := byChannel[channel.ID]; found {
			converted[i].CurrentProgram = &program
		}
	}

	return converted, nil
}

func ProgramDtos(ctx context.Context, catalogue *items.Service, programs []channels.Program) ([]api.BaseItemDto, error) {
	ids := make([]uuid.UUID, 0, len(programs))
	for _, program := range programs {
		ids = append(ids, program.ItemID)
	}

	found, err := catalogue.ItemsByIDs(ctx, items.Everyone, ids)
	if err != nil {
		return nil, err
	}
	records := make([]*items.Item, 0, len(found))
	for _, record := range found {
		records = append(records, record)
	}

	described, err := ItemDtos(ctx, catalogue, records)
	if err != nil {
		return nil, err
	}
	byItem := make(map[uuid.UUID]api.BaseItemDto, len(described))
	for _, converted := range described {
		byItem[*converted.Id] = converted
	}

	converted := make([]api.BaseItemDto, len(programs))
	for i, program := range programs {
		converted[i] = programDto(program, byItem[program.ItemID])
	}

	return converted, nil
}

func programDto(program channels.Program, item api.BaseItemDto) api.BaseItemDto {
	id := program.ID()
	episode := item.Type != nil && *item.Type == api.BaseItemKind(itemmodel.KindEpisode)

	converted := api.BaseItemDto{
		Id:                      apiutil.Ptr(id),
		ServerId:                apiutil.Ptr(config.ServerID),
		Type:                    apiutil.Ptr(api.BaseItemKindProgram),
		Name:                    item.Name,
		Overview:                item.Overview,
		ProductionYear:          item.ProductionYear,
		PremiereDate:            item.PremiereDate,
		OfficialRating:          item.OfficialRating,
		CommunityRating:         item.CommunityRating,
		Genres:                  item.Genres,
		IndexNumber:             item.IndexNumber,
		ParentIndexNumber:       item.ParentIndexNumber,
		ImageTags:               item.ImageTags,
		BackdropImageTags:       item.BackdropImageTags,
		PrimaryImageAspectRatio: item.PrimaryImageAspectRatio,
		ChannelId:               apiutil.Ptr(program.Channel.ID),
		ChannelName:             apiutil.Ptr(program.Channel.Name),
		ChannelNumber:           apiutil.Ptr(strconv.Itoa(int(program.Channel.Number))),
		StartDate:               apiutil.Ptr(program.Start),
		EndDate:                 apiutil.Ptr(program.End),
		RunTimeTicks:            apiutil.Ptr(int64(program.End.Sub(program.Start) / (100 * time.Nanosecond))),
		IsMovie:                 apiutil.Ptr(!episode),
		IsSeries:                apiutil.Ptr(episode),
		IsLive:                  apiutil.Ptr(false),
		IsNews:                  apiutil.Ptr(false),
		IsKids:                  apiutil.Ptr(false),
		IsSports:                apiutil.Ptr(false),
		IsPremiere:              apiutil.Ptr(false),
		IsRepeat:                apiutil.Ptr(true),
		MediaType:               apiutil.Ptr(api.MediaTypeVideo),
		IsFolder:                apiutil.Ptr(false),
		LocationType:            apiutil.Ptr(api.Virtual),
		PlayAccess:              apiutil.Ptr(api.PlayAccessFull),
	}
	if episode {
		converted.Name = item.SeriesName
		converted.EpisodeTitle = item.Name
		converted.SeriesName = item.SeriesName
	}

	return converted
}
