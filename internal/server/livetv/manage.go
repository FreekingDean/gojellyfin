package livetv

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/channels"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
)

func (s *Server) GoJellyfinListChannels(ctx context.Context, _ api.GoJellyfinListChannelsRequestObject) (api.GoJellyfinListChannelsResponseObject, error) {
	listed, err := s.channels.List(ctx, items.Everyone)
	if err != nil {
		return nil, err
	}

	converted := make(api.GoJellyfinListChannels200JSONResponse, len(listed))
	for i, channel := range listed {
		converted[i] = liveChannel(channel)
	}

	return converted, nil
}

func (s *Server) GoJellyfinCreateChannel(ctx context.Context, request api.GoJellyfinCreateChannelRequestObject) (api.GoJellyfinCreateChannelResponseObject, error) {
	if request.Body == nil {
		return api.GoJellyfinCreateChannel400JSONResponse{Message: "no request body"}, nil
	}

	created, err := s.channels.Create(ctx, definition(*request.Body))
	if errors.Is(err, channels.ErrInvalid) {
		return api.GoJellyfinCreateChannel400JSONResponse{Message: err.Error()}, nil
	}
	if err != nil {
		return nil, err
	}

	return api.GoJellyfinCreateChannel200JSONResponse(liveChannel(created)), nil
}

func (s *Server) GoJellyfinUpdateChannel(ctx context.Context, request api.GoJellyfinUpdateChannelRequestObject) (api.GoJellyfinUpdateChannelResponseObject, error) {
	if request.Body == nil {
		return api.GoJellyfinUpdateChannel400JSONResponse{Message: "no request body"}, nil
	}

	updated, err := s.channels.Update(ctx, request.ChannelId, definition(*request.Body))
	switch {
	case errors.Is(err, channels.ErrNotFound):
		return api.GoJellyfinUpdateChannel404Response{}, nil
	case errors.Is(err, channels.ErrInvalid):
		return api.GoJellyfinUpdateChannel400JSONResponse{Message: err.Error()}, nil
	case err != nil:
		return nil, err
	}

	return api.GoJellyfinUpdateChannel200JSONResponse(liveChannel(updated)), nil
}

func (s *Server) GoJellyfinDeleteChannel(ctx context.Context, request api.GoJellyfinDeleteChannelRequestObject) (api.GoJellyfinDeleteChannelResponseObject, error) {
	err := s.channels.Delete(ctx, request.ChannelId)
	if errors.Is(err, channels.ErrNotFound) {
		return api.GoJellyfinDeleteChannel404Response{}, nil
	}
	if err != nil {
		return nil, err
	}

	return api.GoJellyfinDeleteChannel204Response{}, nil
}

func definition(body api.LiveChannelDefinition) channels.Definition {
	return channels.Definition{
		Number:  body.Number,
		Name:    body.Name,
		Order:   channels.Order(body.Order),
		ItemIDs: append([]uuid.UUID(nil), body.ItemIds...),
	}
}

func liveChannel(channel *channels.Channel) api.LiveChannel {
	return api.LiveChannel{
		Id:      channel.ID,
		Number:  channel.Number,
		Name:    channel.Name,
		Order:   api.LiveChannelOrder(channel.Order),
		ItemIds: channels.EntryIDs(channel),
	}
}
