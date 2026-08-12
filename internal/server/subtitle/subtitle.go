package subtitle

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/auth"
	"github.com/FreekingDean/gojellyfin/internal/filesystem"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
)

var contentTypes = map[string]string{
	"vtt": "text/vtt",
	"srt": "application/x-subrip",
	"ass": "text/x-ssa",
	"ssa": "text/x-ssa",
	"sub": "text/plain",
}

type Server struct {
	items      *items.Service
	filesystem *filesystem.Service
}

func New(items *items.Service, filesystem *filesystem.Service) *Server {
	return &Server{items: items, filesystem: filesystem}
}

func (s *Server) GetSubtitle(ctx context.Context, request api.GetSubtitleRequestObject) (api.GetSubtitleResponseObject, error) {
	body, contentType, length, err := s.subtitle(ctx, request.RouteItemId, request.RouteIndex, request.RouteFormat, window{
		start:          apiutil.Deref(request.Params.StartPositionTicks),
		end:            apiutil.Deref(request.Params.EndPositionTicks),
		copyTimestamps: apiutil.Deref(request.Params.CopyTimestamps),
		addTimeMap:     apiutil.Deref(request.Params.AddVttTimeMap),
	})
	if err != nil {
		return nil, err
	}

	return api.GetSubtitle200TextResponse{Body: body, ContentType: contentType, ContentLength: length}, nil
}

func (s *Server) GetSubtitleWithTicks(ctx context.Context, request api.GetSubtitleWithTicksRequestObject) (api.GetSubtitleWithTicksResponseObject, error) {
	body, contentType, length, err := s.subtitle(ctx, request.RouteItemId, request.RouteIndex, request.RouteFormat, window{
		start:          request.RouteStartPositionTicks,
		end:            apiutil.Deref(request.Params.EndPositionTicks),
		copyTimestamps: apiutil.Deref(request.Params.CopyTimestamps),
		addTimeMap:     apiutil.Deref(request.Params.AddVttTimeMap),
	})
	if err != nil {
		return nil, err
	}

	return api.GetSubtitleWithTicks200TextResponse{Body: body, ContentType: contentType, ContentLength: length}, nil
}

func (s *Server) subtitle(ctx context.Context, itemID uuid.UUID, index int32, format string, window window) (io.Reader, string, int64, error) {
	stream, err := s.items.SubtitleStream(ctx, itemID, index)
	if err != nil {
		return nil, "", 0, err
	}
	if stream == nil {
		return nil, "", 0, fmt.Errorf("item %s has no subtitle stream %d", itemID, index)
	}

	target := strings.ToLower(strings.TrimPrefix(format, "."))
	direct := sourceFormat(stream) == target && window.whole()
	if err := servable(stream, target, direct); err != nil {
		return nil, "", 0, err
	}

	body, size, err := s.filesystem.Open(ctx, stream.Path)
	if err != nil {
		return nil, "", 0, fmt.Errorf("failed to open subtitle %s: %w", stream.Path, err)
	}

	if direct {
		return body, contentTypes[target], size, nil
	}

	return convert(body, target, window), contentTypes[target], 0, nil
}

func servable(stream *items.MediaStream, target string, direct bool) error {
	if !stream.IsExternal {
		return fmt.Errorf("extracting an embedded subtitle needs ffmpeg: %w", api.ErrNotImplemented)
	}

	source := sourceFormat(stream)
	if !direct && (!cueFormats[source] || !cueFormats[target]) {
		return fmt.Errorf("converting %s subtitles to %s needs ffmpeg: %w", source, target, api.ErrNotImplemented)
	}

	return nil
}

func sourceFormat(stream *items.MediaStream) string {
	return strings.ToLower(strings.TrimPrefix(filepath.Ext(stream.Path), "."))
}

func (s *Server) GetSubtitlePlaylist(ctx context.Context, request api.GetSubtitlePlaylistRequestObject) (api.GetSubtitlePlaylistResponseObject, error) {
	stream, err := s.items.SubtitleStream(ctx, request.ItemId, request.Index)
	if err != nil {
		return nil, err
	}
	if stream == nil {
		return api.GetSubtitlePlaylist404JSONResponse{}, nil
	}
	if err := servable(stream, "vtt", false); err != nil {
		return nil, err
	}

	item, err := s.items.ItemByID(ctx, request.ItemId)
	if err != nil {
		return nil, err
	}

	runtime := apiutil.Deref(item.RunTimeTicks)
	segment := int64(request.Params.SegmentLength) * ticksPerSecond
	if runtime <= 0 || segment <= 0 {
		return api.GetSubtitlePlaylist404JSONResponse{}, nil
	}

	playlist := playlist(runtime, segment, auth.AuthorizationFrom(ctx).Token)

	return api.GetSubtitlePlaylist200ApplicationxMpegURLResponse{
		Body:          strings.NewReader(playlist),
		ContentLength: int64(len(playlist)),
	}, nil
}

// The segments are fetched from GetSubtitle beside this playlist, and an HLS
// player cannot send a header, so the token rides in the query.
func playlist(runtime, segment int64, token string) string {
	lines := []string{
		"#EXTM3U",
		fmt.Sprintf("#EXT-X-TARGETDURATION:%d", segment/ticksPerSecond),
		"#EXT-X-VERSION:3",
		"#EXT-X-MEDIA-SEQUENCE:0",
		"#EXT-X-PLAYLIST-TYPE:VOD",
	}

	for position := int64(0); position < runtime; position += segment {
		end := min(position+segment, runtime)
		lines = append(lines,
			fmt.Sprintf("#EXTINF:%.3f,", float64(end-position)/ticksPerSecond),
			fmt.Sprintf("Stream.vtt?StartPositionTicks=%d&EndPositionTicks=%d&api_key=%s", position, end, url.QueryEscape(token)),
		)
	}

	return strings.Join(append(lines, "#EXT-X-ENDLIST", ""), "\n")
}
