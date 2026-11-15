package middleware

import (
	"net/url"
	"slices"
	"testing"
)

func TestCanonicalQuery(t *testing.T) {
	t.Run("fixes casing", func(t *testing.T) {
		got, changed := canonicalQuery(url.Values{"ParentId": {"abc"}})

		if !changed || got.Get("parentId") != "abc" {
			t.Errorf("ParentId not canonicalised: %v", got)
		}
	})

	t.Run("drops empty values", func(t *testing.T) {
		got, changed := canonicalQuery(url.Values{
			"AudioStreamIndex": {""},
			"UserId":           {""},
			"Limit":            {"12"},
		})

		if !changed {
			t.Error("expected the query to change")
		}
		if _, ok := got["audioStreamIndex"]; ok {
			t.Error("empty audioStreamIndex should have been dropped")
		}
		if _, ok := got["userId"]; ok {
			t.Error("empty userId should have been dropped")
		}
		if got.Get("limit") != "12" {
			t.Errorf("limit = %q, want 12", got.Get("limit"))
		}
	})

	t.Run("splits a comma-joined array", func(t *testing.T) {
		got, changed := canonicalQuery(url.Values{"IncludeItemTypes": {"Movie,Series,"}})

		if !changed {
			t.Error("expected the query to change")
		}
		if want := []string{"Movie", "Series"}; !slices.Equal(got["includeItemTypes"], want) {
			t.Errorf("includeItemTypes = %q, want %q", got["includeItemTypes"], want)
		}
	})

	t.Run("keeps repeated keys", func(t *testing.T) {
		got, _ := canonicalQuery(url.Values{"includeItemTypes": {"Movie", "Series"}})

		if want := []string{"Movie", "Series"}; !slices.Equal(got["includeItemTypes"], want) {
			t.Errorf("includeItemTypes = %q, want %q", got["includeItemTypes"], want)
		}
	})

	t.Run("leaves free text whole", func(t *testing.T) {
		got, changed := canonicalQuery(url.Values{
			"searchTerm": {"red, white and blue"},
			"genres":     {"Sci-Fi, Fantasy"},
		})

		if changed {
			t.Error("expected the query to be left alone")
		}
		if got.Get("searchTerm") != "red, white and blue" || len(got["genres"]) != 1 {
			t.Errorf("free text was split: %v", got)
		}
	})
}
