package sources

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/sources/arr"
	"github.com/FreekingDean/gojellyfin/internal/sources/radarr"
	"github.com/FreekingDean/gojellyfin/internal/sources/sonarr"
	sourcemodel "github.com/FreekingDean/gojellyfin/internal/store/source"
)

type Copy struct {
	TitleID int64
	FileID  int64
}

func (s *Service) Replace(ctx context.Context, sourceID uuid.UUID, broken Copy) error {
	source, err := s.store.Source.Query().Where(sourcemodel.ID(sourceID)).OnlyModel(ctx)
	if err != nil {
		return fmt.Errorf("failed to query the source: %w", err)
	}

	return s.replace(ctx, *source, broken)
}

func (s *Service) replace(ctx context.Context, source Source, broken Copy) error {
	filePath, search := radarr.FilePath, record{"name": radarr.Search, "movieIds": []int64{broken.TitleID}}
	if source.Kind == KindSonarr {
		filePath, search = sonarr.FilePath, record{"name": sonarr.Search, "episodeIds": []int64{broken.TitleID}}
	}

	grab, grabbed, err := s.grabbed(ctx, source, broken.TitleID)
	if err != nil {
		return err
	}

	if err := s.send(ctx, source, http.MethodDelete, filePath+"/"+strconv.FormatInt(broken.FileID, 10), nil); err != nil {
		return err
	}
	if grabbed {
		if err := s.send(ctx, source, http.MethodPost, arr.FailedPath+"/"+strconv.Itoa(grab.ID), nil); err != nil {
			return err
		}
	}

	return s.send(ctx, source, http.MethodPost, arr.CommandPath, search)
}

func (s *Service) grabbed(ctx context.Context, source Source, titleID int64) (arr.Grab, bool, error) {
	id := strconv.FormatInt(titleID, 10)

	if source.Kind == KindSonarr {
		page, err := fetch[sonarr.Grabs](ctx, s, s.client, source, sonarr.GrabsPath, url.Values{
			"episodeId":     {id},
			"eventType":     {sonarr.GrabbedEvent},
			"pageSize":      {"50"},
			"sortKey":       {"date"},
			"sortDirection": {"descending"},
		})
		if err != nil {
			return arr.Grab{}, false, err
		}
		grab, found := arr.Newest(page.Records)

		return grab, found, nil
	}

	grabs, err := fetch[[]arr.Grab](ctx, s, s.client, source, radarr.GrabsPath, url.Values{
		"movieId":   {id},
		"eventType": {radarr.GrabbedEvent},
	})
	if err != nil {
		return arr.Grab{}, false, err
	}
	grab, found := arr.Newest(grabs)

	return grab, found, nil
}
