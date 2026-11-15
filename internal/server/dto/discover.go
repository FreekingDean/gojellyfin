package dto

import (
	"context"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/auth"
	"github.com/FreekingDean/gojellyfin/internal/config"
	"github.com/FreekingDean/gojellyfin/internal/discover"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/requests"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
)

const (
	requestedTagline = "Requested"
	pendingTagline   = "Awaiting approval"
)

type Requests interface {
	Marks(ctx context.Context, userID uuid.UUID, keys []string) (map[string]requests.Mark, error)
}

func UnownedDtos(ctx context.Context, asked Requests, titles []discover.Title) ([]api.BaseItemDto, error) {
	keys := make([]string, 0, len(titles))
	for _, title := range titles {
		keys = append(keys, title.Key)
	}

	marks, err := asked.Marks(ctx, auth.UserID(ctx), keys)
	if err != nil {
		return nil, err
	}

	converted := make([]api.BaseItemDto, 0, len(titles))
	for _, title := range titles {
		converted = append(converted, UnownedDto(title, marks[title.Key]))
	}

	return converted, nil
}

func UnownedDto(title discover.Title, mark requests.Mark) api.BaseItemDto {
	kind := api.BaseItemKind(title.Kind)
	metadata := title.Metadata
	name := apiutil.Deref(metadata.Name)

	imageTags := map[string]*string{}
	if url, found := title.Image(items.ImageKindPrimary); found {
		imageTags[string(api.Primary)] = apiutil.Ptr(discover.Tag(url))
	}
	backdrops := []string{}
	if url, found := title.Image(items.ImageKindBackdrop); found {
		backdrops = append(backdrops, discover.Tag(url))
	}

	dto := api.BaseItemDto{
		Id:                apiutil.Ptr(title.ID),
		ServerId:          apiutil.Ptr(config.ServerID),
		Name:              apiutil.Ptr(name),
		SortName:          apiutil.Ptr(items.SortName(name)),
		Type:              &kind,
		IsFolder:          apiutil.Ptr(items.IsFolder(title.Kind)),
		LocationType:      apiutil.Ptr(api.Virtual),
		PlayAccess:        apiutil.Ptr(api.PlayAccessNone),
		CanDelete:         apiutil.Ptr(false),
		CanDownload:       apiutil.Ptr(false),
		Overview:          metadata.Overview,
		ProductionYear:    metadata.ProductionYear,
		PremiereDate:      metadata.PremiereDate,
		EndDate:           metadata.EndDate,
		RunTimeTicks:      metadata.RunTimeTicks,
		Status:            metadata.Status,
		Taglines:          taglines(metadata.Taglines, mark),
		Genres:            metadata.Genres,
		ImageTags:         &imageTags,
		BackdropImageTags: &backdrops,
		UserData:          apiutil.Ptr(UserItemDataDto(&items.Datum{ItemID: title.ID, IsFavorite: mark.Mine})),
	}

	if _, ok := imageTags[string(api.Primary)]; ok {
		dto.PrimaryImageAspectRatio = apiutil.Ptr(primaryAspect(kind))
	}
	if metadata.OfficialRating != nil {
		dto.OfficialRating = apiutil.Ptr(string(*metadata.OfficialRating))
	}
	if metadata.CommunityRating != nil {
		dto.CommunityRating = apiutil.Ptr(float32(*metadata.CommunityRating))
	}
	if items.IsFolder(title.Kind) {
		dto.ChildCount = apiutil.Ptr(int32(0))
	} else {
		dto.MediaType = apiutil.Ptr(api.MediaType(items.MediaTypeOf(title.Kind)))
	}

	return dto
}

func taglines(provided *[]string, mark requests.Mark) *[]string {
	switch {
	case mark.Requested:
		return apiutil.Ptr(append([]string{requestedTagline}, apiutil.Deref(provided)...))
	case mark.Pending:
		return apiutil.Ptr(append([]string{pendingTagline}, apiutil.Deref(provided)...))
	default:
		return provided
	}
}
