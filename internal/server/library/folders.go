package library

import (
	"context"

	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	"github.com/FreekingDean/gojellyfin/internal/server/dto"
)

func (s *Server) GetMediaFolders(ctx context.Context, request api.GetMediaFoldersRequestObject) (api.GetMediaFoldersResponseObject, error) {
	records, err := s.libraries.ShownLibraries(ctx)
	if err != nil {
		return nil, err
	}

	folders, err := dto.LibraryViews(ctx, s.collages, records)
	if err != nil {
		return nil, err
	}

	return api.GetMediaFolders200JSONResponse{
		Items:            &folders,
		StartIndex:       apiutil.Ptr(int32(0)),
		TotalRecordCount: apiutil.Ptr(int32(len(folders))),
	}, nil
}

func (s *Server) GetLibraryOptionsInfo(ctx context.Context, request api.GetLibraryOptionsInfoRequestObject) (api.GetLibraryOptionsInfoResponseObject, error) {
	return api.GetLibraryOptionsInfo200JSONResponse{
		MetadataSavers:   &[]api.LibraryOptionInfoDto{},
		MetadataReaders:  &[]api.LibraryOptionInfoDto{},
		SubtitleFetchers: &[]api.LibraryOptionInfoDto{},
		LyricFetchers:    &[]api.LibraryOptionInfoDto{},
		TypeOptions:      &[]api.LibraryTypeOptionsDto{},
	}, nil
}
