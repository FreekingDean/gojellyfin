package middleware

import (
	"net/url"
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
}
