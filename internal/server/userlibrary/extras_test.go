package userlibrary

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/FreekingDean/gojellyfin/internal/server/api"
)

func TestServer_GetIntros(t *testing.T) {
	response, err := New(nil).GetIntros(context.Background(), api.GetIntrosRequestObject{})
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
}

func TestServer_Extras(t *testing.T) {
	s := New(nil)
	ctx := context.Background()

	for _, tc := range []struct {
		name string
		call func() (any, error)
	}{
		{"local trailers", func() (any, error) {
			return s.GetLocalTrailers(ctx, api.GetLocalTrailersRequestObject{})
		}},
		{"special features", func() (any, error) {
			return s.GetSpecialFeatures(ctx, api.GetSpecialFeaturesRequestObject{})
		}},
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
			if string(encoded) != "[]" {
				t.Errorf("got %s, want []", encoded)
			}
		})
	}
}
