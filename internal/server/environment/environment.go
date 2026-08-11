package environment

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
)

const rootPath = "/"

type Server struct{}

func New() *Server {
	return &Server{}
}

func (s *Server) GetDirectoryContents(ctx context.Context, request api.GetDirectoryContentsRequestObject) (api.GetDirectoryContentsResponseObject, error) {
	entries, err := os.ReadDir(request.Params.Path)
	if err != nil {
		return api.GetDirectoryContents200JSONResponse{}, nil
	}

	includeFiles := apiutil.Deref(request.Params.IncludeFiles)
	includeDirectories := apiutil.Deref(request.Params.IncludeDirectories)

	contents := make([]api.FileSystemEntryInfo, 0, len(entries))
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		path := filepath.Join(request.Params.Path, entry.Name())
		if isDirectory(entry, path) {
			if includeDirectories {
				contents = append(contents, entryInfo(entry.Name(), path, api.FileSystemEntryTypeDirectory))
			}
			continue
		}
		if includeFiles {
			contents = append(contents, entryInfo(entry.Name(), path, api.FileSystemEntryTypeFile))
		}
	}

	return api.GetDirectoryContents200JSONResponse(contents), nil
}

func (s *Server) GetDrives(ctx context.Context, request api.GetDrivesRequestObject) (api.GetDrivesResponseObject, error) {
	return api.GetDrives200JSONResponse{
		entryInfo(rootPath, rootPath, api.FileSystemEntryTypeDirectory),
	}, nil
}

func (s *Server) GetNetworkShares(ctx context.Context, request api.GetNetworkSharesRequestObject) (api.GetNetworkSharesResponseObject, error) {
	return api.GetNetworkShares200JSONResponse{}, nil
}

func (s *Server) GetParentPath(ctx context.Context, request api.GetParentPathRequestObject) (api.GetParentPathResponseObject, error) {
	path := filepath.Clean(request.Params.Path)
	parent := filepath.Dir(path)
	if parent == path {
		return api.GetParentPath200JSONResponse(""), nil
	}

	return api.GetParentPath200JSONResponse(parent), nil
}

func (s *Server) GetDefaultDirectoryBrowser(ctx context.Context, request api.GetDefaultDirectoryBrowserRequestObject) (api.GetDefaultDirectoryBrowserResponseObject, error) {
	return api.GetDefaultDirectoryBrowser200JSONResponse{}, nil
}

func entryInfo(name, path string, kind api.FileSystemEntryType) api.FileSystemEntryInfo {
	return api.FileSystemEntryInfo{
		Name: apiutil.Ptr(name),
		Path: apiutil.Ptr(path),
		Type: apiutil.Ptr(kind),
	}
}

func isDirectory(entry fs.DirEntry, path string) bool {
	if entry.Type()&fs.ModeSymlink == 0 {
		return entry.IsDir()
	}

	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// Used by the dashboard before it will accept a media path.
func (s *Server) ValidatePath(ctx context.Context, request api.ValidatePathRequestObject) (api.ValidatePathResponseObject, error) {
	req := apiutil.Body(request.JSONBody, request.ApplicationWildcardPlusJSONBody)
	if req == nil || req.Path == nil || *req.Path == "" {
		return api.ValidatePath404JSONResponse{}, nil
	}

	info, err := os.Stat(*req.Path)
	if err != nil {
		return api.ValidatePath404JSONResponse{}, nil
	}
	if apiutil.Deref(req.IsFile) && info.IsDir() {
		return api.ValidatePath404JSONResponse{}, nil
	}

	return api.ValidatePath204Response{}, nil
}
