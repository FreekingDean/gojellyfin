package subtitle

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/auth"
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
	items *items.Service
}

func New(items *items.Service) *Server {
	return &Server{items: items}
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
	if !stream.IsExternal {
		return nil, "", 0, fmt.Errorf("extracting an embedded subtitle needs ffmpeg: %w", api.ErrNotImplemented)
	}

	target := strings.ToLower(strings.TrimPrefix(format, "."))
	source := strings.ToLower(strings.TrimPrefix(filepath.Ext(stream.Path), "."))
	direct := source == target && window.whole()
	if !direct && (!cueFormats[source] || !cueFormats[target]) {
		return nil, "", 0, fmt.Errorf("converting %s subtitles to %s needs ffmpeg: %w", source, target, api.ErrNotImplemented)
	}

	file, err := os.Open(stream.Path)
	if err != nil {
		return nil, "", 0, fmt.Errorf("failed to open subtitle %s: %w", stream.Path, err)
	}

	if !direct {
		return convert(file, target, window), contentTypes[target], 0, nil
	}

	info, err := file.Stat()
	if err != nil {
		_ = file.Close()

		return nil, "", 0, fmt.Errorf("failed to stat subtitle %s: %w", stream.Path, err)
	}

	return file, contentTypes[target], info.Size(), nil
}

func (s *Server) GetSubtitlePlaylist(ctx context.Context, request api.GetSubtitlePlaylistRequestObject) (api.GetSubtitlePlaylistResponseObject, error) {
	stream, err := s.items.SubtitleStream(ctx, request.ItemId, request.Index)
	if err != nil {
		return nil, err
	}
	if stream == nil {
		return api.GetSubtitlePlaylist404JSONResponse{}, nil
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

// Each segment is fetched from GetSubtitle beside this playlist, so the entries
// are relative to it.
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
