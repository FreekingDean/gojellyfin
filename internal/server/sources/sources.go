package sources

import (
	"context"
	"fmt"
	"strings"

	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	"github.com/FreekingDean/gojellyfin/internal/sources"
)

const (
	radarrKind api.SourceKind = "Radarr"
	sonarrKind api.SourceKind = "Sonarr"
)

type Server struct {
	sources *sources.Service
}

func New(sources *sources.Service) *Server {
	return &Server{
		sources: sources,
	}
}

func (s *Server) GoJellyfinListSources(
	ctx context.Context,
	_ api.GoJellyfinListSourcesRequestObject,
) (api.GoJellyfinListSourcesResponseObject, error) {
	configured, err := s.sources.List(ctx)
	if err != nil {
		return nil, err
	}

	resp := make([]api.Source, len(configured))
	for i, entry := range configured {
		resp[i] = modelToParams(entry)
	}

	return api.GoJellyfinListSources200JSONResponse(resp), nil
}

func (s *Server) GoJellyfinUpdateSources(
	ctx context.Context,
	request api.GoJellyfinUpdateSourcesRequestObject,
) (api.GoJellyfinUpdateSourcesResponseObject, error) {
	if request.Body == nil {
		return nil, fmt.Errorf("no request body")
	}

	configured := make([]sources.Configured, len(*request.Body))
	for i, source := range *request.Body {
		configured[i] = paramsToModel(source)
	}

	if err := s.sources.Update(ctx, configured); err != nil {
		return nil, err
	}

	return api.GoJellyfinUpdateSources204Response{}, nil
}

func (s *Server) GoJellyfinTestSource(
	ctx context.Context,
	request api.GoJellyfinTestSourceRequestObject,
) (api.GoJellyfinTestSourceResponseObject, error) {
	if request.Body == nil {
		return unreachable("no request body"), nil
	}

	if err := s.sources.Test(ctx, request.Body.Url, request.Body.ApiKey); err != nil {
		return unreachable(err.Error()), nil
	}

	return api.GoJellyfinTestSource200JSONResponse{
		Reachable: true,
	}, nil
}

func unreachable(reason string) api.GoJellyfinTestSource200JSONResponse {
	return api.GoJellyfinTestSource200JSONResponse{
		Reachable: false,
		Error:     apiutil.Ptr(reason),
	}
}

func modelToParams(entry sources.Configured) api.Source {
	bound := make([]api.SourceLibrary, len(entry.Libraries))
	for i, library := range entry.Libraries {
		bound[i] = api.SourceLibrary{
			Id:         library.ID,
			TagFilter:  apiutil.Ptr(library.TagFilter),
			SourcePath: apiutil.Ptr(library.SourcePath),
			TargetPath: apiutil.Ptr(library.TargetPath),
		}
	}

	return api.Source{
		Name:      apiutil.Ptr(entry.Source.Name),
		Kind:      apiutil.Ptr(apiKind(entry.Source.Kind)),
		Url:       apiutil.Ptr(entry.Source.URL),
		ApiKey:    apiutil.Ptr(entry.Source.APIKey),
		Libraries: apiutil.Ptr(bound),
	}
}

func paramsToModel(req api.Source) sources.Configured {
	bound := apiutil.Deref(req.Libraries)

	entry := sources.Configured{
		Source: sources.Source{
			Name:   apiutil.Deref(req.Name),
			Kind:   modelKind(apiutil.Deref(req.Kind)),
			URL:    apiutil.Deref(req.Url),
			APIKey: apiutil.Deref(req.ApiKey),
		},
		Libraries: make([]sources.Library, len(bound)),
	}

	for i, library := range bound {
		entry.Libraries[i] = sources.Library{
			ID:         library.Id,
			TagFilter:  apiutil.Deref(library.TagFilter),
			SourcePath: apiutil.Deref(library.SourcePath),
			TargetPath: apiutil.Deref(library.TargetPath),
		}
	}

	return entry
}

func apiKind(kind sources.Kind) api.SourceKind {
	if kind == sources.KindSonarr {
		return sonarrKind
	}

	return radarrKind
}

func modelKind(kind api.SourceKind) sources.Kind {
	if strings.EqualFold(string(kind), string(sonarrKind)) {
		return sources.KindSonarr
	}

	return sources.KindRadarr
}
