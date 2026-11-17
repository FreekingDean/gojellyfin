package sources

import (
	"context"
	"net/http"
	"slices"
	"testing"
)

func TestReplace(t *testing.T) {
	ctx := context.Background()
	broken := Copy{TitleID: 12, FileID: 34}

	for _, test := range replaceCases {
		t.Run(test.name, func(t *testing.T) {
			instance := newInstance(t, test.answers)

			err := submitter().replace(ctx, instance.source(test.kind, ""), broken)
			if (err != nil) != test.failing {
				t.Fatalf("err = %v, want one: %v", err, test.failing)
			}
			if got := instance.requests(); !slices.Equal(got, test.want) {
				t.Errorf("asked %v, want %v", got, test.want)
			}
			if test.command == nil {
				return
			}

			bodies := instance.bodies()
			sent := bodies[len(bodies)-1]
			if sent["name"] != test.command["name"] || len(sent) != 2 {
				t.Errorf("command = %v, want %v", sent, test.command)
			}
		})
	}
}

type replaceCase struct {
	name    string
	kind    Kind
	answers map[string]any
	want    []string
	command map[string]any
	failing bool
}

var replaceCases = []replaceCase{
	{
		name: "a movie's file is deleted, its newest grab blocklisted and the movie searched for again",
		kind: KindRadarr,
		answers: map[string]any{
			"GET /api/v3/history/movie": []map[string]any{
				{"id": 7, "date": "2026-01-02T00:00:00Z"},
				{"id": 9, "date": "2026-03-04T00:00:00Z"},
				{"id": 8, "date": "2026-02-03T00:00:00Z"},
			},
			"DELETE /api/v3/moviefile/34":   map[string]any{},
			"POST /api/v3/history/failed/9": map[string]any{},
			"POST /api/v3/command":          map[string]any{"id": 1},
		},
		want: []string{
			"GET /api/v3/history/movie?eventType=grabbed&movieId=12",
			"DELETE /api/v3/moviefile/34",
			"POST /api/v3/history/failed/9",
			"POST /api/v3/command",
		},
		command: map[string]any{"name": "MoviesSearch", "movieIds": []any{float64(12)}},
	},
	{
		name: "a file nobody grabbed is deleted and searched for with nothing to blocklist",
		kind: KindRadarr,
		answers: map[string]any{
			"GET /api/v3/history/movie":   []map[string]any{},
			"DELETE /api/v3/moviefile/34": map[string]any{},
			"POST /api/v3/command":        map[string]any{"id": 1},
		},
		want: []string{
			"GET /api/v3/history/movie?eventType=grabbed&movieId=12",
			"DELETE /api/v3/moviefile/34",
			"POST /api/v3/command",
		},
		command: map[string]any{"name": "MoviesSearch", "movieIds": []any{float64(12)}},
	},
	{
		name: "an episode goes through Sonarr's spellings",
		kind: KindSonarr,
		answers: map[string]any{
			"GET /api/v3/history":           map[string]any{"records": []map[string]any{{"id": 5, "date": "2026-03-04T00:00:00Z"}}},
			"DELETE /api/v3/episodefile/34": map[string]any{},
			"POST /api/v3/history/failed/5": map[string]any{},
			"POST /api/v3/command":          map[string]any{"id": 1},
		},
		want: []string{
			"GET /api/v3/history?episodeId=12&eventType=1&pageSize=50&sortDirection=descending&sortKey=date",
			"DELETE /api/v3/episodefile/34",
			"POST /api/v3/history/failed/5",
			"POST /api/v3/command",
		},
		command: map[string]any{"name": "EpisodeSearch", "episodeIds": []any{float64(12)}},
	},
	{
		name: "a file the instance will not delete is not searched for",
		kind: KindRadarr,
		answers: map[string]any{
			"GET /api/v3/history/movie":   []map[string]any{{"id": 7, "date": "2026-01-02T00:00:00Z"}},
			"DELETE /api/v3/moviefile/34": http.StatusInternalServerError,
		},
		want: []string{
			"GET /api/v3/history/movie?eventType=grabbed&movieId=12",
			"DELETE /api/v3/moviefile/34",
		},
		failing: true,
	},
}
