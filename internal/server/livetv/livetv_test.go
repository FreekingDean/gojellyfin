package livetv

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/FreekingDean/gojellyfin/internal/server/api"
)

func TestReadPathsAnswerAnEmptyQueryResult(t *testing.T) {
	s := New()
	ctx := context.Background()

	for _, tc := range []struct {
		name string
		call func() (any, error)
	}{
		{"channels", func() (any, error) { return s.GetLiveTvChannels(ctx, api.GetLiveTvChannelsRequestObject{}) }},
		{"programs", func() (any, error) { return s.GetLiveTvPrograms(ctx, api.GetLiveTvProgramsRequestObject{}) }},
		{"programs by post", func() (any, error) { return s.GetPrograms(ctx, api.GetProgramsRequestObject{}) }},
		{"recommended programs", func() (any, error) {
			return s.GetRecommendedPrograms(ctx, api.GetRecommendedProgramsRequestObject{})
		}},
		{"recordings", func() (any, error) { return s.GetRecordings(ctx, api.GetRecordingsRequestObject{}) }},
		{"recording folders", func() (any, error) {
			return s.GetRecordingFolders(ctx, api.GetRecordingFoldersRequestObject{})
		}},
		{"timers", func() (any, error) { return s.GetTimers(ctx, api.GetTimersRequestObject{}) }},
		{"series timers", func() (any, error) { return s.GetSeriesTimers(ctx, api.GetSeriesTimersRequestObject{}) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			response, err := tc.call()
			if err != nil {
				t.Fatal(err)
			}

			encoded, err := json.Marshal(response)
			if err != nil {
				t.Fatal(err)
			}

			for _, want := range []string{`"Items":[]`, `"TotalRecordCount":0`} {
				if !strings.Contains(string(encoded), want) {
					t.Errorf("got %s, want it to contain %s", encoded, want)
				}
			}
		})
	}
}

func TestGetTunerHostTypesAnswersAnEmptyArray(t *testing.T) {
	response, err := New().GetTunerHostTypes(context.Background(), api.GetTunerHostTypesRequestObject{})
	if err != nil {
		t.Fatal(err)
	}

	encoded, err := json.Marshal(response)
	if err != nil {
		t.Fatal(err)
	}
	if string(encoded) != "[]" {
		t.Errorf("got %s, want []", encoded)
	}
}
