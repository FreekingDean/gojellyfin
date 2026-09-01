package librarystructure

import (
	"context"

	"github.com/FreekingDean/gojellyfin/internal/libraries"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	librarymodal "github.com/FreekingDean/gojellyfin/internal/store/library"
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
		folders = append(folders, virtualFolderInfo(library))
	}

	return api.GetVirtualFolders200JSONResponse(folders), nil
}

func (s *Server) AddVirtualFolder(ctx context.Context, request api.AddVirtualFolderRequestObject) (api.AddVirtualFolderResponseObject, error) {
	if request.Params.Name == nil {
		return api.AddVirtualFolder403Response{}, nil
	}

	collectionType := librarymodal.CollectionTypeMixed
	if request.Params.CollectionType != nil {
		collectionType = libraries.CollectionType(*request.Params.CollectionType)
	}

	var options *api.LibraryOptions
	if req := apiutil.Body(request.JSONBody, request.ApplicationWildcardPlusJSONBody); req != nil {
		options = req.LibraryOptions
	}

	locations := requestedPaths(request.Params.Paths, options)

	library, err := s.libraries.CreateLibrary(ctx, *request.Params.Name, collectionType, locations)
	if err != nil {
		return nil, err
	}

	if err := s.saveOptions(ctx, library.ID, options); err != nil {
		return nil, err
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

	if err := s.libraries.Rename(ctx, library.ID, *request.Params.NewName); err != nil {
		return nil, err
	}

	return api.RenameVirtualFolder204Response{}, nil
}

func (s *Server) UpdateLibraryOptions(ctx context.Context, request api.UpdateLibraryOptionsRequestObject) (api.UpdateLibraryOptionsResponseObject, error) {
	req := apiutil.Body(request.JSONBody, request.ApplicationWildcardPlusJSONBody)
	if req == nil || req.Id == nil {
		return api.UpdateLibraryOptions404JSONResponse{}, nil
	}

	library, err := s.libraries.Library(ctx, *req.Id)
	if err != nil {
		return api.UpdateLibraryOptions404JSONResponse{}, nil
	}

	if err := s.saveOptions(ctx, library.ID, req.LibraryOptions); err != nil {
		return nil, err
	}

	if paths := requestedPaths(nil, req.LibraryOptions); len(paths) > 0 {
		if err := s.libraries.SetLocations(ctx, library.ID, paths); err != nil {
			return nil, err
		}
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

	if err := s.libraries.AddLocation(ctx, library.ID, path); err != nil {
		return nil, err
	}

	return api.AddMediaPath204Response{}, nil
}

func (s *Server) RemoveMediaPath(ctx context.Context, request api.RemoveMediaPathRequestObject) (api.RemoveMediaPathResponseObject, error) {
	library, err := s.libraryByName(ctx, request.Params.Name)
	if err != nil {
		return api.RemoveMediaPath204Response{}, nil
	}

	if err := s.libraries.RemoveLocation(ctx, library.ID, apiutil.Deref(request.Params.Path)); err != nil {
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
	return s.libraries.LibraryByName(ctx, apiutil.Deref(name))
}
