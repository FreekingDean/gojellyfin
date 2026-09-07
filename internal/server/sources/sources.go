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
		entry, err := paramsToModel(source)
		if err != nil {
			return nil, err
		}
		configured[i] = entry
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

	if err := s.sources.Test(ctx, request.Body.Url, request.Body.ApiKeyVariable); err != nil {
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
			Id:        library.LibraryID,
			TagFilter: apiutil.Ptr(library.TagFilter),
		}
	}

	return api.Source{
		Name:           apiutil.Ptr(entry.Source.Name),
		Kind:           apiutil.Ptr(apiKind(entry.Source.Kind)),
		Url:            apiutil.Ptr(entry.Source.URL),
		RootPath:       apiutil.Ptr(entry.Source.RootPath),
		LocalPath:      apiutil.Ptr(entry.Source.LocalPath),
		ApiKeyVariable: apiutil.Ptr(entry.Source.APIKeyVariable),
		Libraries:      apiutil.Ptr(bound),
	}
}

func paramsToModel(req api.Source) (sources.Configured, error) {
	kind, err := modelKind(apiutil.Deref(req.Kind))
	if err != nil {
		return sources.Configured{}, err
	}

	bound := apiutil.Deref(req.Libraries)
	entry := sources.Configured{
		Source: sources.Source{
			Name:           apiutil.Deref(req.Name),
			Kind:           kind,
			URL:            apiutil.Deref(req.Url),
			RootPath:       apiutil.Deref(req.RootPath),
			LocalPath:      apiutil.Deref(req.LocalPath),
			APIKeyVariable: apiutil.Deref(req.ApiKeyVariable),
		},
		Libraries: make([]sources.LibrarySource, len(bound)),
	}

	for i, library := range bound {
		entry.Libraries[i] = sources.LibrarySource{
			LibraryID: library.Id,
			TagFilter: apiutil.Deref(library.TagFilter),
		}
	}

	return entry, nil
}

func apiKind(kind sources.Kind) api.SourceKind {
	if kind == sources.KindSonarr {
		return sonarrKind
	}

	return radarrKind
}

func modelKind(kind api.SourceKind) (sources.Kind, error) {
	switch {
	case strings.EqualFold(string(kind), string(sonarrKind)):
		return sources.KindSonarr, nil
	case strings.EqualFold(string(kind), string(radarrKind)):
		return sources.KindRadarr, nil
	default:
		return "", fmt.Errorf("unsupported source kind %q", kind)
	}
}
