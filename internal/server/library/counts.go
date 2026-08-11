package library

import (
	"context"

	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
)

func (s *Server) GetItemCounts(ctx context.Context, request api.GetItemCountsRequestObject) (api.GetItemCountsResponseObject, error) {
	counts, err := s.items.CountByKind(ctx)
	if err != nil {
		return nil, err
	}

	total := int32(0)
	for _, count := range counts {
		total += count
	}

	return api.GetItemCounts200JSONResponse{
		MovieCount:      apiutil.Ptr(counts["Movie"]),
		SeriesCount:     apiutil.Ptr(counts["Series"]),
		EpisodeCount:    apiutil.Ptr(counts["Episode"]),
		BoxSetCount:     apiutil.Ptr(counts["BoxSet"]),
		SongCount:       apiutil.Ptr(counts["Audio"]),
		AlbumCount:      apiutil.Ptr(counts["MusicAlbum"]),
		ArtistCount:     apiutil.Ptr(counts["MusicArtist"]),
		MusicVideoCount: apiutil.Ptr(counts["MusicVideo"]),
		TrailerCount:    apiutil.Ptr(counts["Trailer"]),
		BookCount:       apiutil.Ptr(counts["Book"]),
		ProgramCount:    apiutil.Ptr(int32(0)),
		ItemCount:       apiutil.Ptr(total),
	}, nil
}
