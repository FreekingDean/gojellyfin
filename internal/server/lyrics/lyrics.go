package lyrics

import (
	"context"
	"errors"
	"unicode/utf8"

	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/lyrics"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	"github.com/FreekingDean/gojellyfin/internal/server/dto"
)

type Server struct {
	items    *items.Service
	policies dto.Access
}

func New(items *items.Service, policies dto.Access) *Server {
	return &Server{items: items, policies: policies}
}

func (s *Server) GetLyrics(ctx context.Context, request api.GetLyricsRequestObject) (api.GetLyricsResponseObject, error) {
	viewer, err := dto.ViewerFor(ctx, s.policies)
	if err != nil {
		return nil, err
	}

	text, err := s.items.Lyrics(ctx, viewer, request.ItemId)
	if errors.Is(err, items.ErrNotFound) {
		return api.GetLyrics404JSONResponse{}, nil
	}
	if err != nil {
		return nil, err
	}

	return api.GetLyrics200JSONResponse(lyricDto(lyrics.Parse(text))), nil
}

func (s *Server) UploadLyrics(ctx context.Context, request api.UploadLyricsRequestObject) (api.UploadLyricsResponseObject, error) {
	if request.Body == nil || len(*request.Body) == 0 || !utf8.Valid(*request.Body) {
		return api.UploadLyrics400JSONResponse{}, nil
	}

	viewer, err := dto.ViewerFor(ctx, s.policies)
	if err != nil {
		return nil, err
	}

	text := string(*request.Body)
	err = s.items.SaveLyrics(ctx, viewer, request.ItemId, text)
	if errors.Is(err, items.ErrNotFound) {
		return api.UploadLyrics404JSONResponse{}, nil
	}
	if err != nil {
		return nil, err
	}

	return api.UploadLyrics200JSONResponse(lyricDto(lyrics.Parse(text))), nil
}

func (s *Server) DeleteLyrics(ctx context.Context, request api.DeleteLyricsRequestObject) (api.DeleteLyricsResponseObject, error) {
	viewer, err := dto.ViewerFor(ctx, s.policies)
	if err != nil {
		return nil, err
	}

	err = s.items.ClearLyrics(ctx, viewer, request.ItemId)
	if errors.Is(err, items.ErrNotFound) {
		return api.DeleteLyrics404JSONResponse{}, nil
	}
	if err != nil {
		return nil, err
	}

	return api.DeleteLyrics204Response{}, nil
}

func lyricDto(parsed lyrics.Lyrics) api.LyricDto {
	lines := make([]api.LyricLine, 0, len(parsed.Lines))
	for _, line := range parsed.Lines {
		lines = append(lines, api.LyricLine{Start: line.Start, Text: apiutil.Ptr(line.Text), Cues: &[]api.LyricLineCue{}})
	}

	return api.LyricDto{
		Lyrics: &lines,
		Metadata: &api.LyricMetadata{
			IsSynced: apiutil.Ptr(parsed.Synced),
			Artist:   apiutil.ZeroOrNilPtr(parsed.Metadata.Artist),
			Album:    apiutil.ZeroOrNilPtr(parsed.Metadata.Album),
			Title:    apiutil.ZeroOrNilPtr(parsed.Metadata.Title),
			Author:   apiutil.ZeroOrNilPtr(parsed.Metadata.Author),
			By:       apiutil.ZeroOrNilPtr(parsed.Metadata.By),
			Offset:   parsed.Metadata.Offset,
			Length:   parsed.Metadata.Length,
		},
	}
}
