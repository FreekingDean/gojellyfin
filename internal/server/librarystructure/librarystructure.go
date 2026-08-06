package librarystructure

import (
	"context"
	"encoding/json"

	"github.com/FreekingDean/gojellyfin/internal/libraries"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
)

type Server struct {
	libraries *libraries.Service
}

func New(libraries *libraries.Service) *Server {
	return &Server{libraries: libraries}
}

func (s *Server) GetVirtualFolders(ctx context.Context, request api.GetVirtualFoldersRequestObject) (api.GetVirtualFoldersResponseObject, error) {
	libraries, err := s.libraries.ListLibraries(ctx)
	if err != nil {
		return nil, err
	}

	folders := make([]api.VirtualFolderInfo, 0, len(libraries))
	for _, library := range libraries {
		folder, err := virtualFolderInfo(&library)
		if err != nil {
			return nil, err
		}
		folders = append(folders, folder)
	}

	return api.GetVirtualFolders200JSONResponse(folders), nil
}

func (s *Server) AddVirtualFolder(ctx context.Context, request api.AddVirtualFolderRequestObject) (api.AddVirtualFolderResponseObject, error) {
	if request.Params.Name == nil {
		return api.AddVirtualFolder403Response{}, nil
	}

	library := &libraries.Library{Name: *request.Params.Name}
	if request.Params.CollectionType != nil {
		library.CollectionType = string(*request.Params.CollectionType)
	}

	if req := apiutil.Body(request.JSONBody, request.ApplicationWildcardPlusJSONBody); req != nil && req.LibraryOptions != nil {
		options, err := json.Marshal(req.LibraryOptions)
		if err != nil {
			return nil, err
		}
		library.Options = options
	}

	if err := s.libraries.CreateLibrary(ctx, library); err != nil {
		return nil, err
	}

	for _, path := range apiutil.Deref(request.Params.Paths) {
		if err := s.libraries.AddLibraryPath(ctx, library.ID, path); err != nil {
			return nil, err
		}
	}

	return api.AddVirtualFolder204Response{}, nil
}

func (s *Server) RemoveVirtualFolder(ctx context.Context, request api.RemoveVirtualFolderRequestObject) (api.RemoveVirtualFolderResponseObject, error) {
	library, err := s.libraryByName(ctx, request.Params.Name)
	if err != nil {
		return api.RemoveVirtualFolder204Response{}, nil
	}

	if err := s.libraries.DeleteLibrary(ctx, library.ID); err != nil {
		return nil, err
	}

	return api.RemoveVirtualFolder204Response{}, nil
}

func (s *Server) RenameVirtualFolder(ctx context.Context, request api.RenameVirtualFolderRequestObject) (api.RenameVirtualFolderResponseObject, error) {
	if request.Params.NewName == nil {
		return api.RenameVirtualFolder404JSONResponse{}, nil
	}

	library, err := s.libraryByName(ctx, request.Params.Name)
	if err != nil {
		return api.RenameVirtualFolder404JSONResponse{}, nil
	}

	library.Name = *request.Params.NewName
	if err := s.libraries.UpdateLibrary(ctx, library); err != nil {
		return nil, err
	}

	return api.RenameVirtualFolder204Response{}, nil
}

func (s *Server) UpdateLibraryOptions(ctx context.Context, request api.UpdateLibraryOptionsRequestObject) (api.UpdateLibraryOptionsResponseObject, error) {
	req := apiutil.Body(request.JSONBody, request.ApplicationWildcardPlusJSONBody)
	if req == nil || req.Id == nil {
		return api.UpdateLibraryOptions404JSONResponse{}, nil
	}

	library, err := s.libraries.GetLibrary(ctx, *req.Id)
	if err != nil {
		return api.UpdateLibraryOptions404JSONResponse{}, nil
	}

	if library.Options, err = json.Marshal(req.LibraryOptions); err != nil {
		return nil, err
	}
	if err := s.libraries.UpdateLibrary(ctx, library); err != nil {
		return nil, err
	}

	return api.UpdateLibraryOptions204Response{}, nil
}

func (s *Server) AddMediaPath(ctx context.Context, request api.AddMediaPathRequestObject) (api.AddMediaPathResponseObject, error) {
	req := apiutil.Body(request.JSONBody, request.ApplicationWildcardPlusJSONBody)
	if req == nil {
		return api.AddMediaPath403Response{}, nil
	}

	library, err := s.libraryByName(ctx, &req.Name)
	if err != nil {
		return api.AddMediaPath403Response{}, nil
	}

	path := apiutil.Deref(req.Path)
	if req.PathInfo != nil {
		path = apiutil.Deref(req.PathInfo.Path)
	}
	if path == "" {
		return api.AddMediaPath403Response{}, nil
	}

	if err := s.libraries.AddLibraryPath(ctx, library.ID, path); err != nil {
		return nil, err
	}

	return api.AddMediaPath204Response{}, nil
}

func (s *Server) RemoveMediaPath(ctx context.Context, request api.RemoveMediaPathRequestObject) (api.RemoveMediaPathResponseObject, error) {
	library, err := s.libraryByName(ctx, request.Params.Name)
	if err != nil {
		return api.RemoveMediaPath204Response{}, nil
	}

	if err := s.libraries.RemoveLibraryPath(ctx, library.ID, apiutil.Deref(request.Params.Path)); err != nil {
		return nil, err
	}

	return api.RemoveMediaPath204Response{}, nil
}

func (s *Server) UpdateMediaPath(ctx context.Context, request api.UpdateMediaPathRequestObject) (api.UpdateMediaPathResponseObject, error) {
	req := apiutil.Body(request.JSONBody, request.ApplicationWildcardPlusJSONBody)
	if req == nil {
		return api.UpdateMediaPath403Response{}, nil
	}

	if _, err := s.libraryByName(ctx, &req.Name); err != nil {
		return api.UpdateMediaPath403Response{}, nil
	}

	return api.UpdateMediaPath204Response{}, nil
}

func (s *Server) libraryByName(ctx context.Context, name *string) (*libraries.Library, error) {
	return s.libraries.GetLibraryByName(ctx, apiutil.Deref(name))
}

func virtualFolderInfo(library *libraries.Library) (api.VirtualFolderInfo, error) {
	options := defaultLibraryOptions()
	if len(library.Options) > 0 {
		if err := json.Unmarshal(library.Options, &options); err != nil {
			return api.VirtualFolderInfo{}, err
		}
	}

	locations := make([]string, 0, len(library.Paths))
	pathInfos := make([]api.MediaPathInfo, 0, len(library.Paths))
	for _, path := range library.Paths {
		locations = append(locations, path.Path)
		pathInfos = append(pathInfos, api.MediaPathInfo{Path: apiutil.Ptr(path.Path)})
	}
	options.PathInfos = &pathInfos

	collectionType := api.CollectionTypeOptions(library.CollectionType)

	return api.VirtualFolderInfo{
		Name:           apiutil.Ptr(library.Name),
		ItemId:         apiutil.Ptr(library.ID.String()),
		Locations:      &locations,
		CollectionType: &collectionType,
		LibraryOptions: &options,
		RefreshStatus:  apiutil.Ptr("Idle"),
	}, nil
}

func defaultLibraryOptions() api.LibraryOptions {
	return api.LibraryOptions{
		Enabled:                       apiutil.Ptr(true),
		EnablePhotos:                  apiutil.Ptr(true),
		EnableRealtimeMonitor:         apiutil.Ptr(true),
		EnableChapterImageExtraction:  apiutil.Ptr(false),
		EnableInternetProviders:       apiutil.Ptr(false),
		EnableAutomaticSeriesGrouping: apiutil.Ptr(false),
		EnableEmbeddedTitles:          apiutil.Ptr(false),
		SaveLocalMetadata:             apiutil.Ptr(false),
		PreferredMetadataLanguage:     apiutil.Ptr("en"),
		MetadataCountryCode:           apiutil.Ptr("US"),
		SeasonZeroDisplayName:         apiutil.Ptr("Specials"),
		AutomaticRefreshIntervalDays:  apiutil.Ptr(int32(0)),
		PathInfos:                     &[]api.MediaPathInfo{},
		MetadataSavers:                &[]string{},
		DisabledLocalMetadataReaders:  &[]string{},
		LocalMetadataReaderOrder:      &[]string{},
		DisabledSubtitleFetchers:      &[]string{},
		SubtitleFetcherOrder:          &[]string{},
		SubtitleDownloadLanguages:     &[]string{},
		DisabledLyricFetchers:         &[]string{},
		LyricFetcherOrder:             &[]string{},
		CustomTagDelimiters:           &[]string{},
		DelimiterWhitelist:            &[]string{},
		TypeOptions:                   &[]api.TypeOptions{},
	}
}
