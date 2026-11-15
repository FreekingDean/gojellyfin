package syncplay

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/auth"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	"github.com/FreekingDean/gojellyfin/internal/syncplay"
)

func (s *Server) apply(ctx context.Context, request syncplay.Request) error {
	session := auth.SessionFrom(ctx)
	if session == nil {
		return auth.ErrUnauthorized
	}

	messages, err := s.syncplay.Apply(ctx, session.ID, request)
	if errors.Is(err, syncplay.ErrNotInGroup) {
		s.publish(ctx, []uuid.UUID{session.ID}, uuid.Nil, notInGroup, "")

		return nil
	}
	if err != nil {
		return err
	}

	s.syncplay.Publish(ctx, messages)

	return nil
}

func ids(values *[]uuid.UUID) []uuid.UUID {
	if values == nil {
		return nil
	}

	return *values
}

func (s *Server) SyncPlaySetNewQueue(ctx context.Context, request api.SyncPlaySetNewQueueRequestObject) (api.SyncPlaySetNewQueueResponseObject, error) {
	body := apiutil.Deref(apiutil.Body(request.JSONBody, request.ApplicationWildcardPlusJSONBody))

	return api.SyncPlaySetNewQueue204Response{}, s.apply(ctx, syncplay.Request{
		Type:                syncplay.Play,
		ItemIDs:             ids(body.PlayingQueue),
		PlayingItemPosition: int(apiutil.Deref(body.PlayingItemPosition)),
		StartPositionTicks:  apiutil.Deref(body.StartPositionTicks),
	})
}

func (s *Server) SyncPlaySetPlaylistItem(ctx context.Context, request api.SyncPlaySetPlaylistItemRequestObject) (api.SyncPlaySetPlaylistItemResponseObject, error) {
	body := apiutil.Deref(apiutil.Body(request.JSONBody, request.ApplicationWildcardPlusJSONBody))

	return api.SyncPlaySetPlaylistItem204Response{}, s.apply(ctx, syncplay.Request{
		Type:           syncplay.SetPlaylistItem,
		PlaylistItemID: apiutil.Deref(body.PlaylistItemId),
	})
}

func (s *Server) SyncPlayRemoveFromPlaylist(ctx context.Context, request api.SyncPlayRemoveFromPlaylistRequestObject) (api.SyncPlayRemoveFromPlaylistResponseObject, error) {
	body := apiutil.Deref(apiutil.Body(request.JSONBody, request.ApplicationWildcardPlusJSONBody))

	return api.SyncPlayRemoveFromPlaylist204Response{}, s.apply(ctx, syncplay.Request{
		Type:             syncplay.RemoveFromPlaylist,
		PlaylistItemIDs:  ids(body.PlaylistItemIds),
		ClearPlaylist:    apiutil.Deref(body.ClearPlaylist),
		ClearPlayingItem: apiutil.Deref(body.ClearPlayingItem),
	})
}

func (s *Server) SyncPlayMovePlaylistItem(ctx context.Context, request api.SyncPlayMovePlaylistItemRequestObject) (api.SyncPlayMovePlaylistItemResponseObject, error) {
	body := apiutil.Deref(apiutil.Body(request.JSONBody, request.ApplicationWildcardPlusJSONBody))

	return api.SyncPlayMovePlaylistItem204Response{}, s.apply(ctx, syncplay.Request{
		Type:           syncplay.MovePlaylistItem,
		PlaylistItemID: apiutil.Deref(body.PlaylistItemId),
		NewIndex:       int(apiutil.Deref(body.NewIndex)),
	})
}

func (s *Server) SyncPlayQueue(ctx context.Context, request api.SyncPlayQueueRequestObject) (api.SyncPlayQueueResponseObject, error) {
	body := apiutil.Deref(apiutil.Body(request.JSONBody, request.ApplicationWildcardPlusJSONBody))

	return api.SyncPlayQueue204Response{}, s.apply(ctx, syncplay.Request{
		Type:    syncplay.QueueItems,
		ItemIDs: ids(body.ItemIds),
		Mode:    syncplay.QueueMode(apiutil.Deref(body.Mode)),
	})
}

func (s *Server) SyncPlayUnpause(ctx context.Context, _ api.SyncPlayUnpauseRequestObject) (api.SyncPlayUnpauseResponseObject, error) {
	return api.SyncPlayUnpause204Response{}, s.apply(ctx, syncplay.Request{Type: syncplay.Unpause})
}

func (s *Server) SyncPlayPause(ctx context.Context, _ api.SyncPlayPauseRequestObject) (api.SyncPlayPauseResponseObject, error) {
	return api.SyncPlayPause204Response{}, s.apply(ctx, syncplay.Request{Type: syncplay.Pause})
}

func (s *Server) SyncPlayStop(ctx context.Context, _ api.SyncPlayStopRequestObject) (api.SyncPlayStopResponseObject, error) {
	return api.SyncPlayStop204Response{}, s.apply(ctx, syncplay.Request{Type: syncplay.Stop})
}

func (s *Server) SyncPlaySeek(ctx context.Context, request api.SyncPlaySeekRequestObject) (api.SyncPlaySeekResponseObject, error) {
	body := apiutil.Deref(apiutil.Body(request.JSONBody, request.ApplicationWildcardPlusJSONBody))

	return api.SyncPlaySeek204Response{}, s.apply(ctx, syncplay.Request{
		Type:          syncplay.Seek,
		PositionTicks: apiutil.Deref(body.PositionTicks),
	})
}

func (s *Server) SyncPlayBuffering(ctx context.Context, request api.SyncPlayBufferingRequestObject) (api.SyncPlayBufferingResponseObject, error) {
	body := apiutil.Deref(apiutil.Body(request.JSONBody, request.ApplicationWildcardPlusJSONBody))

	return api.SyncPlayBuffering204Response{}, s.apply(ctx, syncplay.Request{
		Type:           syncplay.Buffer,
		When:           apiutil.Deref(body.When).UTC(),
		PositionTicks:  apiutil.Deref(body.PositionTicks),
		IsPlaying:      apiutil.Deref(body.IsPlaying),
		PlaylistItemID: apiutil.Deref(body.PlaylistItemId),
	})
}

func (s *Server) SyncPlayReady(ctx context.Context, request api.SyncPlayReadyRequestObject) (api.SyncPlayReadyResponseObject, error) {
	body := apiutil.Deref(apiutil.Body(request.JSONBody, request.ApplicationWildcardPlusJSONBody))

	return api.SyncPlayReady204Response{}, s.apply(ctx, syncplay.Request{
		Type:           syncplay.Ready,
		When:           apiutil.Deref(body.When).UTC(),
		PositionTicks:  apiutil.Deref(body.PositionTicks),
		IsPlaying:      apiutil.Deref(body.IsPlaying),
		PlaylistItemID: apiutil.Deref(body.PlaylistItemId),
	})
}

func (s *Server) SyncPlaySetIgnoreWait(ctx context.Context, request api.SyncPlaySetIgnoreWaitRequestObject) (api.SyncPlaySetIgnoreWaitResponseObject, error) {
	body := apiutil.Deref(apiutil.Body(request.JSONBody, request.ApplicationWildcardPlusJSONBody))

	return api.SyncPlaySetIgnoreWait204Response{}, s.apply(ctx, syncplay.Request{
		Type:       syncplay.IgnoreWait,
		IgnoreWait: apiutil.Deref(body.IgnoreWait),
	})
}

func (s *Server) SyncPlayNextItem(ctx context.Context, request api.SyncPlayNextItemRequestObject) (api.SyncPlayNextItemResponseObject, error) {
	body := apiutil.Deref(apiutil.Body(request.JSONBody, request.ApplicationWildcardPlusJSONBody))

	return api.SyncPlayNextItem204Response{}, s.apply(ctx, syncplay.Request{
		Type:           syncplay.NextItem,
		PlaylistItemID: apiutil.Deref(body.PlaylistItemId),
	})
}

func (s *Server) SyncPlayPreviousItem(ctx context.Context, request api.SyncPlayPreviousItemRequestObject) (api.SyncPlayPreviousItemResponseObject, error) {
	body := apiutil.Deref(apiutil.Body(request.JSONBody, request.ApplicationWildcardPlusJSONBody))

	return api.SyncPlayPreviousItem204Response{}, s.apply(ctx, syncplay.Request{
		Type:           syncplay.PreviousItem,
		PlaylistItemID: apiutil.Deref(body.PlaylistItemId),
	})
}

func (s *Server) SyncPlaySetRepeatMode(ctx context.Context, request api.SyncPlaySetRepeatModeRequestObject) (api.SyncPlaySetRepeatModeResponseObject, error) {
	body := apiutil.Deref(apiutil.Body(request.JSONBody, request.ApplicationWildcardPlusJSONBody))

	return api.SyncPlaySetRepeatMode204Response{}, s.apply(ctx, syncplay.Request{
		Type:   syncplay.SetRepeatMode,
		Repeat: syncplay.RepeatMode(apiutil.OrElse(body.Mode, api.GroupRepeatMode(syncplay.RepeatNone))),
	})
}

func (s *Server) SyncPlaySetShuffleMode(ctx context.Context, request api.SyncPlaySetShuffleModeRequestObject) (api.SyncPlaySetShuffleModeResponseObject, error) {
	body := apiutil.Deref(apiutil.Body(request.JSONBody, request.ApplicationWildcardPlusJSONBody))

	return api.SyncPlaySetShuffleMode204Response{}, s.apply(ctx, syncplay.Request{
		Type:    syncplay.SetShuffleMode,
		Shuffle: syncplay.ShuffleMode(apiutil.OrElse(body.Mode, api.GroupShuffleMode(syncplay.Sorted))),
	})
}

func (s *Server) SyncPlayPing(ctx context.Context, request api.SyncPlayPingRequestObject) (api.SyncPlayPingResponseObject, error) {
	body := apiutil.Deref(apiutil.Body(request.JSONBody, request.ApplicationWildcardPlusJSONBody))

	return api.SyncPlayPing204Response{}, s.apply(ctx, syncplay.Request{
		Type: syncplay.Ping,
		Ping: apiutil.Deref(body.Ping),
	})
}
