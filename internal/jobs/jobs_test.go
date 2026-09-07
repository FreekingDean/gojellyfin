package jobs

import "testing"

func TestParams_id(t *testing.T) {
	t.Run("no params is the bare job name", func(t *testing.T) {
		if got := (Params{}).id("RefreshMetadata"); got != "RefreshMetadata" {
			t.Errorf("id = %q, want the bare name", got)
		}
	})

	t.Run("names the key a value was given under", func(t *testing.T) {
		library := Params{"library": "b3f"}.id("RefreshMetadata")
		scope := Params{"scope": "b3f"}.id("RefreshMetadata")

		if library == scope {
			t.Errorf("both ids are %q: one value under two keys is one id", library)
		}
	})

	t.Run("separates the pairs unambiguously", func(t *testing.T) {
		split := Params{"a": "x", "b": "y:z"}.id("RefreshMetadata")
		earlier := Params{"a": "x:y", "b": "z"}.id("RefreshMetadata")

		if split == earlier {
			t.Errorf("both ids are %q: the separator is readable as a value", split)
		}
	})

	t.Run("is the same id for the same params", func(t *testing.T) {
		want := Params{"force": "true", "scope": "b3f", "library": "9ac"}.id("RefreshMetadata")

		for range 20 {
			got := Params{"force": "true", "scope": "b3f", "library": "9ac"}.id("RefreshMetadata")
			if got != want {
				t.Fatalf("id = %q, want %q every time", got, want)
			}
		}
	})
}
