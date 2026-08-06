package library

import (
	"context"

	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	serveritems "github.com/FreekingDean/gojellyfin/internal/server/items"
)

// The configured libraries, presented as the folders they are.
func (s *Server) GetMediaFolders(ctx context.Context, request api.GetMediaFoldersRequestObject) (api.GetMediaFoldersResponseObject, error) {
	records, err := s.libraries.ListLibraries(ctx)
	if err != nil {
		return nil, err
	}

	folders := make([]api.BaseItemDto, 0, len(records))
	for _, library := range records {
		folders = append(folders, serveritems.LibraryView(&library))
	}

	return api.GetMediaFolders200JSONResponse{
		Items:            &folders,
		StartIndex:       apiutil.Ptr(int32(0)),
		TotalRecordCount: apiutil.Ptr(int32(len(folders))),
	}, nil
}

// Nothing supplies metadata, subtitles or lyrics yet, so there is nothing to
// choose between.
func (s *Server) GetLibraryOptionsInfo(ctx context.Context, request api.GetLibraryOptionsInfoRequestObject) (api.GetLibraryOptionsInfoResponseObject, error) {
	return api.GetLibraryOptionsInfo200JSONResponse{
		MetadataSavers:   &[]api.LibraryOptionInfoDto{},
		MetadataReaders:  &[]api.LibraryOptionInfoDto{},
		SubtitleFetchers: &[]api.LibraryOptionInfoDto{},
		LyricFetchers:    &[]api.LibraryOptionInfoDto{},
		TypeOptions:      &[]api.LibraryTypeOptionsDto{},
	}, nil
}
