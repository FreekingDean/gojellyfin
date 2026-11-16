package userviews

import (
	"context"
	"slices"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/auth"
	"github.com/FreekingDean/gojellyfin/internal/channels"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/libraries"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	"github.com/FreekingDean/gojellyfin/internal/server/dto"
	"github.com/FreekingDean/gojellyfin/internal/shelves"
	"github.com/FreekingDean/gojellyfin/internal/users"
)

const liveTvAccess = "LiveTvAccess"

type Grants interface {
	Satisfies(ctx context.Context, id uuid.UUID, scopes []string) (bool, error)
}

type Channels interface {
	List(ctx context.Context, viewer items.Viewer) ([]*channels.Channel, error)
}

type Server struct {
	libraries *libraries.Service
	shelves   *shelves.Service
	channels  Channels
	grants    Grants
	policies  dto.Access
	cards     dto.Cards
}

func New(libraries *libraries.Service, shelves *shelves.Service, channels *channels.Service, grants *users.Service, policies dto.Access, cards dto.Cards) *Server {
	return &Server{libraries: libraries, shelves: shelves, channels: channels, grants: grants, policies: policies, cards: cards}
}

func (s *Server) liveTv(ctx context.Context, viewer items.Viewer) (bool, error) {
	user := auth.UserID(ctx)
	if user == uuid.Nil {
		return false, nil
	}

	granted, err := s.grants.Satisfies(ctx, user, []string{liveTvAccess})
	if err != nil || !granted {
		return false, err
	}

	listed, err := s.channels.List(ctx, viewer)

	return len(listed) > 0, err
}

func (s *Server) GetUserViews(ctx context.Context, request api.GetUserViewsRequestObject) (api.GetUserViewsResponseObject, error) {
	viewer, err := dto.ViewerFor(ctx, s.policies)
	if err != nil {
		return nil, err
	}

	records, err := s.libraries.ShownLibraries(ctx)
	if err != nil {
		return nil, err
	}
	records = slices.DeleteFunc(records, func(library *libraries.Library) bool { return !viewer.Sees(library.ID) })

	views, err := dto.LibraryViews(ctx, s.cards, records)
	if err != nil {
		return nil, err
	}

	shelved, err := s.shelves.Shelves(ctx, viewer, auth.UserID(ctx))
	if err != nil {
		return nil, err
	}
	for _, shelf := range shelved {
		view, err := dto.ShelfView(ctx, s.cards, shelf)
		if err != nil {
			return nil, err
		}
		views = append(views, view)
	}

	shown, err := s.liveTv(ctx, viewer)
	if err != nil {
		return nil, err
	}
	if shown {
		views = append(views, dto.LiveTvView())
	}

	return api.GetUserViews200JSONResponse{
		Items:            &views,
		StartIndex:       apiutil.Ptr(int32(0)),
		TotalRecordCount: apiutil.Ptr(int32(len(views))),
	}, nil
}

func (s *Server) GetGroupingOptions(ctx context.Context, request api.GetGroupingOptionsRequestObject) (api.GetGroupingOptionsResponseObject, error) {
	records, err := s.libraries.GroupableLibraries(ctx)
	if err != nil {
		return nil, err
	}

	options := make([]api.SpecialViewOptionDto, 0, len(records))
	for _, library := range records {
		options = append(options, groupingOption(library))
	}

	return api.GetGroupingOptions200JSONResponse(options), nil
}
