package environment

import (
	"context"
	"path"
	"strings"

	"github.com/FreekingDean/gojellyfin/internal/filesystem"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
)

type Server struct {
	filesystem *filesystem.Service
}

func New(filesystem *filesystem.Service) *Server {
	return &Server{filesystem: filesystem}
}

// Used by the dashboard before it will accept a media path.
func (s *Server) ValidatePath(ctx context.Context, request api.ValidatePathRequestObject) (api.ValidatePathResponseObject, error) {
	req := apiutil.Body(request.JSONBody, request.ApplicationWildcardPlusJSONBody)
	if req == nil || req.Path == nil || *req.Path == "" {
		return api.ValidatePath404JSONResponse{}, nil
	}

	cleanPath := path.Clean(*req.Path)
	if !path.IsAbs(cleanPath) || strings.Contains("/"+cleanPath+"/", "/../") {
		return api.ValidatePath404JSONResponse{}, nil
	}

	file, err := s.filesystem.Stat(ctx, cleanPath)
	if err != nil {
		return api.ValidatePath404JSONResponse{}, nil
	}
	if apiutil.Deref(req.IsFile) && file.Dir {
		return api.ValidatePath404JSONResponse{}, nil
	}

	return api.ValidatePath204Response{}, nil
}

func (s *Server) GetDirectoryContents(ctx context.Context, request api.GetDirectoryContentsRequestObject) (api.GetDirectoryContentsResponseObject, error) {
	files, err := s.filesystem.List(ctx, request.Params.Path)
	if err != nil {
		return nil, err
	}

	includeFiles := apiutil.Deref(request.Params.IncludeFiles)
	includeDirectories := apiutil.Deref(request.Params.IncludeDirectories)

	contents := make([]api.FileSystemEntryInfo, 0, len(files))
	for _, file := range files {
		if file.Dir && !includeDirectories {
			continue
		}
		if !file.Dir && !includeFiles {
			continue
		}

		contents = append(contents, entryInfo(file, path.Join(request.Params.Path, file.Name)))
	}

	return api.GetDirectoryContents200JSONResponse(contents), nil
}

func (s *Server) GetDrives(ctx context.Context, request api.GetDrivesRequestObject) (api.GetDrivesResponseObject, error) {
	drives, err := s.filesystem.Drives(ctx)
	if err != nil {
		return nil, err
	}

	entries := make([]api.FileSystemEntryInfo, 0, len(drives))
	for _, drive := range drives {
		entries = append(entries, entryInfo(drive, drive.Name))
	}

	return api.GetDrives200JSONResponse(entries), nil
}

func (s *Server) GetParentPath(ctx context.Context, request api.GetParentPathRequestObject) (api.GetParentPathResponseObject, error) {
	if request.Params.Path == "" {
		return api.GetParentPath200JSONResponse(filesystem.Root), nil
	}

	return api.GetParentPath200JSONResponse(path.Dir(path.Clean(request.Params.Path))), nil
}

func (s *Server) GetDefaultDirectoryBrowser(ctx context.Context, request api.GetDefaultDirectoryBrowserRequestObject) (api.GetDefaultDirectoryBrowserResponseObject, error) {
	return api.GetDefaultDirectoryBrowser200JSONResponse{Path: apiutil.Ptr(filesystem.Root)}, nil
}
