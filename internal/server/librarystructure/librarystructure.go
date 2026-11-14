package librarystructure

import (
	"context"

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
		folders = append(folders, virtualFolderInfo(library))
	}

	return api.GetVirtualFolders200JSONResponse(folders), nil
}

func (s *Server) AddVirtualFolder(_ context.Context, _ api.AddVirtualFolderRequestObject) (api.AddVirtualFolderResponseObject, error) {
	return api.AddVirtualFolder403Response{}, nil
}

func (s *Server) RemoveVirtualFolder(_ context.Context, _ api.RemoveVirtualFolderRequestObject) (api.RemoveVirtualFolderResponseObject, error) {
	return api.RemoveVirtualFolder403Response{}, nil
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

	return api.UpdateLibraryOptions204Response{}, nil
}

func (s *Server) AddMediaPath(_ context.Context, _ api.AddMediaPathRequestObject) (api.AddMediaPathResponseObject, error) {
	return api.AddMediaPath403Response{}, nil
}

func (s *Server) RemoveMediaPath(_ context.Context, _ api.RemoveMediaPathRequestObject) (api.RemoveMediaPathResponseObject, error) {
	return api.RemoveMediaPath403Response{}, nil
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
