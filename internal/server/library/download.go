package library

import (
	"context"
	"errors"
	"io"
	"mime"
	"path/filepath"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/auth"
	"github.com/FreekingDean/gojellyfin/internal/filesystem"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
)

type mediaFile struct {
	body        io.ReadCloser
	contentType string
	length      int64
	audio       bool
}

func (s *Server) GetDownload(ctx context.Context, request api.GetDownloadRequestObject) (api.GetDownloadResponseObject, error) {
	allowed, err := s.users.MayDownloadContent(ctx, auth.UserID(ctx))
	if err != nil {
		return nil, err
	}
	if !allowed {
		return api.GetDownload403Response{}, nil
	}

	file, err := s.openItemFile(ctx, request.ItemId)
	if err != nil {
		return nil, err
	}
	if file == nil {
		return api.GetDownload404JSONResponse{}, nil
	}

	if file.audio {
		return api.GetDownload200AudioResponse{
			Body:          file.body,
			ContentType:   file.contentType,
			ContentLength: file.length,
		}, nil
	}

	return api.GetDownload200VideoResponse{
		Body:          file.body,
		ContentType:   file.contentType,
		ContentLength: file.length,
	}, nil
}

func (s *Server) GetFile(ctx context.Context, request api.GetFileRequestObject) (api.GetFileResponseObject, error) {
	allowed, err := s.users.MayDownloadContent(ctx, auth.UserID(ctx))
	if err != nil {
		return nil, err
	}
	if !allowed {
		return api.GetFile403Response{}, nil
	}

	file, err := s.openItemFile(ctx, request.ItemId)
	if err != nil {
		return nil, err
	}
	if file == nil {
		return api.GetFile404JSONResponse{}, nil
	}

	if file.audio {
		return api.GetFile200AudioResponse{
			Body:          file.body,
			ContentType:   file.contentType,
			ContentLength: file.length,
		}, nil
	}

	return api.GetFile200VideoResponse{
		Body:          file.body,
		ContentType:   file.contentType,
		ContentLength: file.length,
	}, nil
}

func (s *Server) openItemFile(ctx context.Context, id uuid.UUID) (*mediaFile, error) {
	records, err := s.itemsByID(ctx, []uuid.UUID{id})
	if err != nil || len(records) == 0 {
		return nil, err
	}

	item := records[0]
	sources, err := s.items.MediaSources(ctx, item.ID)
	if err != nil {
		return nil, err
	}

	// A download is saving bytes rather than decoding them here, so the best
	// copy wins; what the far end can play is its own problem.
	source := items.BestSource(sources)
	if source == nil {
		return nil, nil
	}

	body, size, err := s.filesystem.Open(ctx, source.Path)
	if errors.Is(err, filesystem.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &mediaFile{
		body:        body,
		contentType: contentType(source.Path),
		length:      size,
		audio:       api.MediaType(item.MediaType) == api.MediaTypeAudio,
	}, nil
}

func contentType(path string) string {
	if kind := mime.TypeByExtension(filepath.Ext(path)); kind != "" {
		return kind
	}

	return "application/octet-stream"
}
