package sources

import (
	"context"
	"log"
	"net/url"
	"strconv"

	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/jobs"
	"github.com/FreekingDean/gojellyfin/internal/sources/lidarr"
	itemmodel "github.com/FreekingDean/gojellyfin/internal/store/item"
)

var artistImages = map[string]items.ImageKind{
	"poster": items.ImageKindPrimary,
	"fanart": items.ImageKindBackdrop,
	"banner": items.ImageKindBanner,
	"logo":   items.ImageKindLogo,
}

var albumImages = map[string]items.ImageKind{
	"cover": items.ImageKindPrimary,
	"disc":  items.ImageKindDisc,
}

func (s *Service) music(ctx context.Context, binding Binding) ([]Title, error) {
	tagged, err := s.tagFilter(ctx, binding)
	if err != nil {
		return nil, err
	}

	artists, err := get[[]lidarr.Artist](ctx, s, binding.Source, lidarr.ArtistsPath, nil)
	if err != nil {
		return nil, err
	}

	titles := make([]Title, 0, len(artists))
	for _, artist := range artists {
		if artist.ForeignArtistID == "" {
			log.Printf("skipping %s: %s reports no MusicBrainz id", artist.ArtistName, binding.Source.Name)

			continue
		}

		jobs.Heartbeat(ctx, artist.ArtistName)

		found, err := s.artistTitles(ctx, binding.Source, artist, tagged(artist.Tags))
		if err != nil {
			return nil, err
		}
		titles = append(titles, found...)
	}

	return titles, nil
}

func (s *Service) artistTitles(ctx context.Context, source Source, artist lidarr.Artist, tagged bool) ([]Title, error) {
	below, err := s.albums(ctx, source, artist, tagged)
	if err != nil || len(below) == 0 {
		return nil, err
	}

	return append([]Title{{
		Key:      items.ArtistKey(artist.ForeignArtistID),
		Tagged:   tagged,
		Kind:     itemmodel.KindMusicArtist,
		Name:     artist.ArtistName,
		SortName: items.SortName(artist.ArtistName),
		Genres:   artist.Genres,
		Images:   images(artist.Images, artistImages),
	}}, below...), nil
}

func (s *Service) albums(ctx context.Context, source Source, artist lidarr.Artist, tagged bool) ([]Title, error) {
	query := url.Values{"artistId": {strconv.Itoa(artist.ID)}}

	albums, err := get[[]lidarr.Album](ctx, s, source, lidarr.AlbumsPath, query)
	if err != nil {
		return nil, err
	}

	tracks, err := get[[]lidarr.Track](ctx, s, source, lidarr.TracksPath, query)
	if err != nil {
		return nil, err
	}

	files, err := get[[]lidarr.File](ctx, s, source, lidarr.FilesPath, query)
	if err != nil {
		return nil, err
	}

	byFile := make(map[int64]lidarr.File, len(files))
	for _, found := range files {
		byFile[found.ID] = found
	}

	byAlbum := map[int][]lidarr.Track{}
	for _, track := range tracks {
		if track.HasFile {
			byAlbum[track.AlbumID] = append(byAlbum[track.AlbumID], track)
		}
	}

	titles := make([]Title, 0, len(tracks))
	for _, album := range albums {
		if album.ForeignAlbumID == "" || len(byAlbum[album.ID]) == 0 {
			continue
		}

		key, year := items.AlbumKey(album.ForeignAlbumID), releaseYear(album.ReleaseDate)
		titles = append(titles, Title{
			Key:       key,
			ParentKey: items.ArtistKey(artist.ForeignArtistID),
			Tagged:    tagged,
			Kind:      itemmodel.KindMusicAlbum,
			Name:      album.Title,
			SortName:  items.SortName(album.Title),
			Year:      year,
			Genres:    album.Genres,
			Images:    images(album.Images, albumImages),
		})

		titles = append(titles, trackTitles(source, key, year, tagged, byAlbum[album.ID], byFile)...)
	}

	return titles, nil
}

func trackTitles(source Source, albumKey string, year *int32, tagged bool, tracks []lidarr.Track, byFile map[int64]lidarr.File) []Title {
	titles := make([]Title, 0, len(tracks))
	for _, track := range tracks {
		recorded, ok := byFile[track.TrackFileID]
		if !ok || track.ForeignTrackID == "" {
			continue
		}

		found, err := file(source, track.ID, recorded)
		if err != nil {
			log.Printf("skipping %s: %v", track.Title, err)

			continue
		}

		titles = append(titles, Title{
			Key:         items.TrackKey(track.ForeignTrackID),
			ParentKey:   albumKey,
			Tagged:      tagged,
			Kind:        itemmodel.KindAudio,
			Name:        track.Title,
			SortName:    items.SortName(track.Title),
			Year:        year,
			Index:       trackNumber(track),
			ParentIndex: known32(track.MediumNumber),
			Files:       []File{found},
		})
	}

	return titles
}

func images(from []lidarr.Image, kinds map[string]items.ImageKind) []items.RemoteImage {
	found := make([]items.RemoteImage, 0, len(from))
	seen := map[items.ImageKind]bool{}
	for _, image := range from {
		kind, ok := kinds[image.CoverType]
		if !ok || image.RemoteURL == "" || seen[kind] {
			continue
		}

		seen[kind] = true
		found = append(found, items.RemoteImage{Kind: kind, URL: image.RemoteURL})
	}

	return found
}

func releaseYear(date string) *int32 {
	if len(date) < 4 {
		return nil
	}

	year, err := strconv.ParseInt(date[:4], 10, 32)
	if err != nil {
		return nil
	}

	return released(int32(year))
}

func trackNumber(track lidarr.Track) *int32 {
	if number, err := strconv.ParseInt(track.TrackNumber, 10, 32); err == nil && number > 0 {
		return ptr(int32(number))
	}

	return known32(track.AbsoluteTrackNumber)
}

func known32(value int32) *int32 {
	if value <= 0 {
		return nil
	}

	return &value
}
