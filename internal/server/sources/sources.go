package sources

import (
	"context"
	"errors"
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
		resp[i] = sourceDto(entry)
	}

	return api.GoJellyfinListSources200JSONResponse(resp), nil
}

func (s *Server) GoJellyfinGetSource(
	ctx context.Context,
	request api.GoJellyfinGetSourceRequestObject,
) (api.GoJellyfinGetSourceResponseObject, error) {
	entry, err := s.sources.Get(ctx, request.SourceId)
	if errors.Is(err, sources.ErrNotFound) {
		return api.GoJellyfinGetSource404Response{}, nil
	}
	if err != nil {
		return nil, err
	}

	return api.GoJellyfinGetSource200JSONResponse(sourceDto(entry)), nil
}

func (s *Server) GoJellyfinCreateSource(
	ctx context.Context,
	request api.GoJellyfinCreateSourceRequestObject,
) (api.GoJellyfinCreateSourceResponseObject, error) {
	if request.Body == nil {
		return api.GoJellyfinCreateSource400JSONResponse{Message: "no request body"}, nil
	}

	entry, err := configured(*request.Body)
	if err != nil {
		return api.GoJellyfinCreateSource400JSONResponse{Message: err.Error()}, nil
	}

	saved, err := s.sources.Create(ctx, entry)
	if errors.Is(err, sources.ErrInvalid) {
		return api.GoJellyfinCreateSource400JSONResponse{Message: err.Error()}, nil
	}
	if err != nil {
		return nil, err
	}

	return api.GoJellyfinCreateSource200JSONResponse(sourceDto(saved)), nil
}

func (s *Server) GoJellyfinUpdateSource(
	ctx context.Context,
	request api.GoJellyfinUpdateSourceRequestObject,
) (api.GoJellyfinUpdateSourceResponseObject, error) {
	if request.Body == nil {
		return api.GoJellyfinUpdateSource400JSONResponse{Message: "no request body"}, nil
	}

	entry, err := configured(*request.Body)
	if err != nil {
		return api.GoJellyfinUpdateSource400JSONResponse{Message: err.Error()}, nil
	}

	saved, err := s.sources.Update(ctx, request.SourceId, entry)
	switch {
	case errors.Is(err, sources.ErrNotFound):
		return api.GoJellyfinUpdateSource404Response{}, nil
	case errors.Is(err, sources.ErrInvalid):
		return api.GoJellyfinUpdateSource400JSONResponse{Message: err.Error()}, nil
	case err != nil:
		return nil, err
	}

	return api.GoJellyfinUpdateSource200JSONResponse(sourceDto(saved)), nil
}

func (s *Server) GoJellyfinDeleteSource(
	ctx context.Context,
	request api.GoJellyfinDeleteSourceRequestObject,
) (api.GoJellyfinDeleteSourceResponseObject, error) {
	err := s.sources.Delete(ctx, request.SourceId)
	if errors.Is(err, sources.ErrNotFound) {
		return api.GoJellyfinDeleteSource404Response{}, nil
	}
	if err != nil {
		return nil, err
	}

	return api.GoJellyfinDeleteSource204Response{}, nil
}

func (s *Server) GoJellyfinListSourceApiKeyVariables(
	_ context.Context,
	_ api.GoJellyfinListSourceApiKeyVariablesRequestObject,
) (api.GoJellyfinListSourceApiKeyVariablesResponseObject, error) {
	return api.GoJellyfinListSourceApiKeyVariables200JSONResponse(s.sources.APIKeyVariables()), nil
}

func (s *Server) GoJellyfinTestSource(
	ctx context.Context,
	request api.GoJellyfinTestSourceRequestObject,
) (api.GoJellyfinTestSourceResponseObject, error) {
	if request.Body == nil {
		return unreachable("no request body"), nil
	}

	found, err := s.sources.Test(ctx, request.Body.Url, request.Body.ApiKeyVariable)
	if err != nil {
		return unreachable(err.Error()), nil
	}

	return api.GoJellyfinTestSource200JSONResponse{
		Reachable:   true,
		Version:     apiutil.Ptr(found.Version),
		RootFolders: apiutil.Ptr(found.RootFolders),
	}, nil
}

func unreachable(reason string) api.GoJellyfinTestSource200JSONResponse {
	return api.GoJellyfinTestSource200JSONResponse{
		Reachable: false,
		Error:     apiutil.Ptr(reason),
	}
}

func sourceDto(entry sources.Configured) api.Source {
	bound := make([]api.SourceLibrary, len(entry.Libraries))
	for i, library := range entry.Libraries {
		bound[i] = api.SourceLibrary{
			Id:           library.LibraryID,
			TagFilter:    apiutil.Ptr(library.TagFilter),
			LastSyncedAt: library.LastSyncedAt,
			LastError:    apiutil.ZeroOrNilPtr(library.LastError),
		}
	}

	paths := make([]api.SourcePath, len(entry.Source.Paths))
	for i, mapping := range entry.Source.Paths {
		paths[i] = api.SourcePath{
			RemotePath: mapping.RemotePath,
			LocalPath:  mapping.LocalPath,
		}
	}

	return api.Source{
		Id:             apiutil.Ptr(entry.Source.ID),
		Name:           apiutil.Ptr(entry.Source.Name),
		Kind:           apiutil.Ptr(apiKind(entry.Source.Kind)),
		Url:            apiutil.Ptr(entry.Source.URL),
		ApiKeyVariable: apiutil.Ptr(entry.Source.APIKeyVariable),
		Paths:          apiutil.Ptr(paths),
		Libraries:      apiutil.Ptr(bound),
		Version:        apiutil.ZeroOrNilPtr(entry.Source.Version),
		ContactedAt:    entry.Source.ContactedAt,
		LastError:      apiutil.ZeroOrNilPtr(entry.Source.LastError),
	}
}

func configured(req api.Source) (sources.Configured, error) {
	kind, err := modelKind(apiutil.Deref(req.Kind))
	if err != nil {
		return sources.Configured{}, err
	}

	mapped := apiutil.Deref(req.Paths)
	paths := make([]*sources.Path, len(mapped))
	for i, mapping := range mapped {
		paths[i] = &sources.Path{
			RemotePath: strings.TrimSpace(mapping.RemotePath),
			LocalPath:  strings.TrimSpace(mapping.LocalPath),
		}
	}

	bound := apiutil.Deref(req.Libraries)
	entry := sources.Configured{
		Source: sources.Source{
			Name:           strings.TrimSpace(apiutil.Deref(req.Name)),
			Kind:           kind,
			URL:            strings.TrimSpace(apiutil.Deref(req.Url)),
			APIKeyVariable: strings.TrimSpace(apiutil.Deref(req.ApiKeyVariable)),
			Paths:          paths,
		},
		Libraries: make([]sources.LibrarySource, len(bound)),
	}

	for i, library := range bound {
		entry.Libraries[i] = sources.LibrarySource{
			LibraryID: library.Id,
			TagFilter: strings.TrimSpace(apiutil.Deref(library.TagFilter)),
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
