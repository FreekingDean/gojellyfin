package library

import (
	"context"

	"github.com/FreekingDean/gojellyfin/internal/jobs"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
)

func (s *Server) PostUpdatedMedia(ctx context.Context, _ api.PostUpdatedMediaRequestObject) (api.PostUpdatedMediaResponseObject, error) {
	return api.PostUpdatedMedia204Response{}, s.refresh(ctx)
}

func (s *Server) PostAddedMovies(ctx context.Context, _ api.PostAddedMoviesRequestObject) (api.PostAddedMoviesResponseObject, error) {
	return api.PostAddedMovies204Response{}, s.refresh(ctx)
}

func (s *Server) PostUpdatedMovies(ctx context.Context, _ api.PostUpdatedMoviesRequestObject) (api.PostUpdatedMoviesResponseObject, error) {
	return api.PostUpdatedMovies204Response{}, s.refresh(ctx)
}

func (s *Server) PostAddedSeries(ctx context.Context, _ api.PostAddedSeriesRequestObject) (api.PostAddedSeriesResponseObject, error) {
	return api.PostAddedSeries204Response{}, s.refresh(ctx)
}

func (s *Server) PostUpdatedSeries(ctx context.Context, _ api.PostUpdatedSeriesRequestObject) (api.PostUpdatedSeriesResponseObject, error) {
	return api.PostUpdatedSeries204Response{}, s.refresh(ctx)
}

func (s *Server) refresh(ctx context.Context) error {
	return s.tasks.Start(ctx, jobs.RefreshLibraries)
}
