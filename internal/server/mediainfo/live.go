package mediainfo

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/auth"
	"github.com/FreekingDean/gojellyfin/internal/channels"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	"github.com/FreekingDean/gojellyfin/internal/server/dto"
	"github.com/FreekingDean/gojellyfin/internal/transcode"
)

type Channels interface {
	Channel(ctx context.Context, viewer items.Viewer, id uuid.UUID) (*channels.Channel, error)
	Schedule(ctx context.Context, channel *channels.Channel) (channels.Schedule, error)
	Now() time.Time
}

func (s *Server) tuned(ctx context.Context, channel *channels.Channel, session string) (api.PlaybackInfoResponse, error) {
	response := api.PlaybackInfoResponse{
		MediaSources:  &[]api.MediaSourceInfo{},
		PlaySessionId: apiutil.Ptr(session),
	}

	schedule, err := s.channels.Schedule(ctx, channel)
	if err != nil {
		return api.PlaybackInfoResponse{}, err
	}
	airing, found := schedule.At(s.channels.Now())
	if !found {
		log.Printf("nothing airs on channel %d", channel.Number)
		response.ErrorCode = apiutil.Ptr(api.NoCompatibleStream)

		return response, nil
	}

	sources, err := s.items.MediaSources(ctx, airing.ItemID)
	if err != nil {
		return api.PlaybackInfoResponse{}, err
	}
	for _, source := range sources {
		if source.ID == airing.SourceID {
			response.MediaSources = &[]api.MediaSourceInfo{liveSource(channel, source, auth.AuthorizationFrom(ctx).Token, session)}

			return response, nil
		}
	}

	log.Printf("channel %d airs a file that is gone", channel.Number)
	response.ErrorCode = apiutil.Ptr(api.NoCompatibleStream)

	return response, nil
}

func liveSource(channel *channels.Channel, source *items.MediaSource, token, session string) api.MediaSourceInfo {
	encoded := *source
	encoded.Streams = nil
	if picture := items.Picture(source); picture != nil {
		output := *picture
		output.Codec = transcode.VideoCodec
		encoded.Streams = append(encoded.Streams, &output)
	}
	if sound := items.Sound(source); sound != nil {
		output := *sound
		output.Codec = transcode.SegmentAudioCodec
		encoded.Streams = append(encoded.Streams, &output)
	}

	info := dto.MediaSourceDto(&encoded)
	info.Id = apiutil.Ptr(channel.ID.String())
	info.Name = apiutil.Ptr(channel.Name)
	info.Path = nil
	info.Container = apiutil.Ptr(transcode.VideoContainer)
	info.Size = nil
	info.Bitrate = nil
	info.RunTimeTicks = nil
	info.IsInfiniteStream = apiutil.Ptr(true)
	info.HasSegments = apiutil.Ptr(false)
	info.SupportsDirectPlay = apiutil.Ptr(false)
	info.SupportsDirectStream = apiutil.Ptr(false)
	info.SupportsTranscoding = apiutil.Ptr(true)
	info.TranscodingUrl = apiutil.Ptr(liveURL(channel.ID, token, session))
	info.TranscodingSubProtocol = apiutil.Ptr(api.MediaStreamProtocolHls)
	info.TranscodingContainer = apiutil.Ptr(transcode.VideoContainer)

	return info
}

func liveURL(channelID uuid.UUID, token, session string) string {
	query := url.Values{}
	query.Set("mediaSourceId", channelID.String())
	query.Set("playSessionId", session)
	query.Set("api_key", token)
	query.Set("videoCodec", transcode.VideoCodec)
	query.Set("audioCodec", transcode.SegmentAudioCodec)
	query.Set("maxHeight", strconv.Itoa(transcode.MaxHeight))
	query.Set("allowVideoStreamCopy", "false")
	query.Set("allowAudioStreamCopy", "false")

	return fmt.Sprintf("/Videos/%s/master.m3u8?%s", channelID, query.Encode())
}
