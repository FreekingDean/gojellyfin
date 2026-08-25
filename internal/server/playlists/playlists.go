package playlists

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/auth"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/playlists"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	"github.com/FreekingDean/gojellyfin/internal/server/dto"
)

type Server struct {
	playlists *playlists.Service
	items     *items.Service
}

func New(playlists *playlists.Service, items *items.Service) *Server {
	return &Server{playlists: playlists, items: items}
}

func (s *Server) CreatePlaylist(ctx context.Context, request api.CreatePlaylistRequestObject) (api.CreatePlaylistResponseObject, error) {
	owner := auth.UserID(ctx)
	if owner == uuid.Nil {
		return api.CreatePlaylist403Response{}, nil
	}

	body := apiutil.Body(request.JSONBody, request.ApplicationWildcardPlusJSONBody)
	if body == nil {
		body = &api.CreatePlaylistDto{}
	}

	params := playlists.CreateParams{
		Name:       body.Name,
		MediaType:  mediaType(body.MediaType),
		OwnerID:    owner,
		OpenAccess: apiutil.Deref(body.IsPublic),
		ItemIDs:    apiutil.Deref(body.Ids),
		Shares:     permissions(body.Users),
	}
	if request.Params.Name != nil {
		params.Name = *request.Params.Name
	}
	if request.Params.MediaType != nil {
		params.MediaType = mediaType(request.Params.MediaType)
	}
	if request.Params.Ids != nil {
		params.ItemIDs = *request.Params.Ids
	}

	item, err := s.playlists.Create(ctx, params)
	if errors.Is(err, playlists.ErrInvalidShare) {
		return api.CreatePlaylist403Response{}, nil
	}
	if err != nil {
		return nil, err
	}

	return api.CreatePlaylist200JSONResponse{Id: apiutil.Ptr(item.ID.String())}, nil
}

func (s *Server) GetPlaylist(ctx context.Context, request api.GetPlaylistRequestObject) (api.GetPlaylistResponseObject, error) {
	access, err := s.access(ctx, request.PlaylistId)
	if err != nil {
		return api.GetPlaylist404JSONResponse{}, nil
	}
	if !access.CanView {
		return api.GetPlaylist403Response{}, nil
	}

	playlist, err := s.playlists.PlaylistByItemID(ctx, request.PlaylistId)
	if err != nil {
		return nil, err
	}

	entries, _, err := s.playlists.Entries(ctx, request.PlaylistId, 0, 0)
	if err != nil {
		return nil, err
	}

	shares, err := s.playlists.Shares(ctx, request.PlaylistId)
	if err != nil {
		return nil, err
	}

	return api.GetPlaylist200JSONResponse(playlistDto(playlist, entries, shares)), nil
}

func (s *Server) UpdatePlaylist(ctx context.Context, request api.UpdatePlaylistRequestObject) (api.UpdatePlaylistResponseObject, error) {
	body := apiutil.Body(request.JSONBody, request.ApplicationWildcardPlusJSONBody)
	if body == nil {
		return api.UpdatePlaylist403JSONResponse{}, nil
	}

	access, err := s.access(ctx, request.PlaylistId)
	if err != nil {
		return api.UpdatePlaylist404JSONResponse{}, nil
	}
	if !access.CanEdit || (body.Users != nil && !access.Owner) {
		return api.UpdatePlaylist403JSONResponse{}, nil
	}

	params := playlists.UpdateParams{
		Name:       body.Name,
		OpenAccess: body.IsPublic,
		ItemIDs:    body.Ids,
	}
	if body.Users != nil {
		params.Shares = apiutil.Ptr(permissions(body.Users))
	}

	if err := s.playlists.Update(ctx, request.PlaylistId, params); err != nil {
		if errors.Is(err, playlists.ErrInvalidShare) {
			return api.UpdatePlaylist403JSONResponse{}, nil
		}

		return nil, err
	}

	return api.UpdatePlaylist204Response{}, nil
}

func (s *Server) GetPlaylistItems(ctx context.Context, request api.GetPlaylistItemsRequestObject) (api.GetPlaylistItemsResponseObject, error) {
	access, err := s.access(ctx, request.PlaylistId)
	if err != nil {
		return api.GetPlaylistItems404JSONResponse{}, nil
	}
	if !access.CanView {
		return api.GetPlaylistItems403JSONResponse{}, nil
	}

	startIndex := int(apiutil.Deref(request.Params.StartIndex))

	entries, total, err := s.playlists.Entries(ctx, request.PlaylistId, startIndex, int(apiutil.Deref(request.Params.Limit)))
	if err != nil {
		return nil, err
	}

	records := make([]*items.Item, 0, len(entries))
	entryIDs := make([]string, 0, len(entries))
	for _, entry := range entries {
		item := playlists.EntryItem(entry)
		if item == nil {
			continue
		}
		records = append(records, item)
		entryIDs = append(entryIDs, entry.ID.String())
	}

	converted, err := dto.ItemDtos(ctx, s.items, records)
	if err != nil {
		return nil, err
	}
	for index := range converted {
		converted[index].PlaylistItemId = &entryIDs[index]
	}

	return api.GetPlaylistItems200JSONResponse{
		Items:            &converted,
		StartIndex:       apiutil.Ptr(int32(startIndex)),
		TotalRecordCount: apiutil.Ptr(int32(total)),
	}, nil
}

func (s *Server) AddItemToPlaylist(ctx context.Context, request api.AddItemToPlaylistRequestObject) (api.AddItemToPlaylistResponseObject, error) {
	access, err := s.access(ctx, request.PlaylistId)
	if err != nil {
		return api.AddItemToPlaylist404JSONResponse{}, nil
	}
	if !access.CanEdit {
		return api.AddItemToPlaylist403JSONResponse{}, nil
	}

	if err := s.playlists.AddItems(ctx, request.PlaylistId, apiutil.Deref(request.Params.Ids)); err != nil {
		return nil, err
	}

	return api.AddItemToPlaylist204Response{}, nil
}

func (s *Server) RemoveItemFromPlaylist(ctx context.Context, request api.RemoveItemFromPlaylistRequestObject) (api.RemoveItemFromPlaylistResponseObject, error) {
	playlistID, err := uuid.Parse(request.PlaylistId)
	if err != nil {
		return api.RemoveItemFromPlaylist404JSONResponse{}, nil
	}

	entryIDs, err := uuids(request.Params.EntryIds)
	if err != nil {
		return api.RemoveItemFromPlaylist404JSONResponse{}, nil
	}

	access, err := s.access(ctx, playlistID)
	if err != nil {
		return api.RemoveItemFromPlaylist404JSONResponse{}, nil
	}
	if !access.CanEdit {
		return api.RemoveItemFromPlaylist403JSONResponse{}, nil
	}

	if err := s.playlists.RemoveEntries(ctx, playlistID, entryIDs); err != nil {
		return nil, err
	}

	return api.RemoveItemFromPlaylist204Response{}, nil
}

func (s *Server) MoveItem(ctx context.Context, request api.MoveItemRequestObject) (api.MoveItemResponseObject, error) {
	playlistID, err := uuid.Parse(request.PlaylistId)
	if err != nil {
		return api.MoveItem404JSONResponse{}, nil
	}

	entryID, err := uuid.Parse(request.ItemId)
	if err != nil {
		return api.MoveItem404JSONResponse{}, nil
	}

	access, err := s.access(ctx, playlistID)
	if err != nil {
		return api.MoveItem404JSONResponse{}, nil
	}
	if !access.CanEdit {
		return api.MoveItem403JSONResponse{}, nil
	}

	if err := s.playlists.MoveEntry(ctx, playlistID, entryID, int(request.NewIndex)); err != nil {
		return api.MoveItem404JSONResponse{}, nil
	}

	return api.MoveItem204Response{}, nil
}

func (s *Server) GetPlaylistUsers(ctx context.Context, request api.GetPlaylistUsersRequestObject) (api.GetPlaylistUsersResponseObject, error) {
	access, err := s.access(ctx, request.PlaylistId)
	if err != nil {
		return api.GetPlaylistUsers404JSONResponse{}, nil
	}
	if !access.Owner {
		return api.GetPlaylistUsers403JSONResponse{}, nil
	}

	shares, err := s.playlists.Shares(ctx, request.PlaylistId)
	if err != nil {
		return nil, err
	}

	return api.GetPlaylistUsers200JSONResponse(userPermissions(shares)), nil
}

func (s *Server) GetPlaylistUser(ctx context.Context, request api.GetPlaylistUserRequestObject) (api.GetPlaylistUserResponseObject, error) {
	access, err := s.access(ctx, request.PlaylistId)
	if err != nil {
		return api.GetPlaylistUser404JSONResponse{}, nil
	}
	if !access.Owner && request.UserId != auth.UserID(ctx) {
		return api.GetPlaylistUser403JSONResponse{}, nil
	}

	share, err := s.playlists.ShareFor(ctx, request.PlaylistId, request.UserId)
	if err != nil {
		return api.GetPlaylistUser404JSONResponse{}, nil
	}

	return api.GetPlaylistUser200JSONResponse(userPermission(share)), nil
}

func (s *Server) UpdatePlaylistUser(ctx context.Context, request api.UpdatePlaylistUserRequestObject) (api.UpdatePlaylistUserResponseObject, error) {
	body := apiutil.Body(request.JSONBody, request.ApplicationWildcardPlusJSONBody)
	if body == nil {
		return api.UpdatePlaylistUser403JSONResponse{}, nil
	}

	access, err := s.access(ctx, request.PlaylistId)
	if err != nil {
		return api.UpdatePlaylistUser404JSONResponse{}, nil
	}
	if !access.Owner {
		return api.UpdatePlaylistUser403JSONResponse{}, nil
	}

	permission := playlists.Permission{UserID: request.UserId, CanEdit: apiutil.Deref(body.CanEdit)}
	if err := s.playlists.SetShare(ctx, request.PlaylistId, permission); err != nil {
		if errors.Is(err, playlists.ErrInvalidShare) {
			return api.UpdatePlaylistUser404JSONResponse{}, nil
		}

		return nil, err
	}

	return api.UpdatePlaylistUser204Response{}, nil
}

func (s *Server) RemoveUserFromPlaylist(ctx context.Context, request api.RemoveUserFromPlaylistRequestObject) (api.RemoveUserFromPlaylistResponseObject, error) {
	access, err := s.access(ctx, request.PlaylistId)
	if err != nil {
		return api.RemoveUserFromPlaylist404JSONResponse{}, nil
	}
	if !access.Owner {
		return api.RemoveUserFromPlaylist403JSONResponse{}, nil
	}

	if err := s.playlists.RemoveShare(ctx, request.PlaylistId, request.UserId); err != nil {
		return nil, err
	}

	return api.RemoveUserFromPlaylist204Response{}, nil
}

func (s *Server) access(ctx context.Context, playlistID uuid.UUID) (playlists.Access, error) {
	return s.playlists.Access(ctx, playlistID, auth.UserID(ctx))
}

func uuids(values *[]string) ([]uuid.UUID, error) {
	if values == nil {
		return nil, nil
	}

	parsed := make([]uuid.UUID, 0, len(*values))
	for _, value := range *values {
		id, err := uuid.Parse(value)
		if err != nil {
			return nil, err
		}
		parsed = append(parsed, id)
	}

	return parsed, nil
}
