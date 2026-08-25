package library

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/env"
	"github.com/FreekingDean/gojellyfin/internal/filesystem"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/jobs"
	"github.com/FreekingDean/gojellyfin/internal/libraries"
	"github.com/FreekingDean/gojellyfin/internal/metadata"
	"github.com/FreekingDean/gojellyfin/internal/scanner"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	itemmodal "github.com/FreekingDean/gojellyfin/internal/store/item"
	"github.com/FreekingDean/gojellyfin/internal/users"
)

// The registry holds only the job the case expects, so a handler that started
// the other one fails on the lookup rather than on the missing connection.
type namedJob string

func (n namedJob) Name() string                             { return string(n) }
func (n namedJob) Category() string                         { return "Library" }
func (n namedJob) Description() string                      { return "" }
func (n namedJob) Steps() []any                             { return nil }
func (n namedJob) Children() []any                          { return nil }
func (n namedJob) Run(_ jobs.Context, _ jobs.Options) error { return nil }

func (f *fixture) expecting(t *testing.T, job string) *Server {
	t.Helper()

	registry := jobs.NewRegistry()
	registry.Register(namedJob(job))

	return New(
		items.New(f.client),
		libraries.New(f.client),
		users.New(f.client),
		filesystem.New(env.Config{MediaDirectories: []string{filesystem.Root}}),
		jobs.NewService(disconnected(t), registry),
	)
}

func refreshRequest(id uuid.UUID, mode api.MetadataRefreshMode, replace bool) api.RefreshItemRequestObject {
	return api.RefreshItemRequestObject{
		ItemId: id,
		Params: api.RefreshItemParams{
			MetadataRefreshMode: &mode,
			ReplaceAllMetadata:  &replace,
		},
	}
}

func TestServer_RefreshItem(t *testing.T) {
	t.Run("answers 404 for an id that is neither a library nor an item", func(t *testing.T) {
		fixed := newFixture(t)

		response, err := fixed.server.RefreshItem(
			context.Background(),
			refreshRequest(uuid.New(), api.MetadataRefreshModeFullRefresh, true),
		)
		if err != nil {
			t.Fatalf("failed to refresh: %v", err)
		}
		if _, missing := response.(api.RefreshItem404JSONResponse); !missing {
			t.Errorf("response = %T, want a 404", response)
		}
	})

	t.Run("answers 204 without queuing when the mode asks for nothing", func(t *testing.T) {
		fixed := newFixture(t)
		movie := fixed.add(t, seed{kind: itemmodal.KindMovie, name: "The Matrix"})

		response, err := fixed.server.RefreshItem(
			context.Background(),
			refreshRequest(movie, api.MetadataRefreshModeNone, true),
		)
		if err != nil {
			t.Fatalf("failed to refresh: %v", err)
		}
		if _, queued := response.(api.RefreshItem204Response); !queued {
			t.Errorf("response = %T, want a 204", response)
		}
	})

	t.Run("starts the metadata job for an item", func(t *testing.T) {
		fixed := newFixture(t)
		movie := fixed.add(t, seed{kind: itemmodal.KindMovie, name: "The Matrix"})

		if _, err := fixed.expecting(t, metadata.RefreshMetadataJobID).RefreshItem(
			context.Background(),
			refreshRequest(movie, api.MetadataRefreshModeFullRefresh, true),
		); !errors.Is(err, jobs.ErrNotConfigured) {
			t.Fatalf("err = %v, want only the queue missing", err)
		}
	})

	t.Run("starts the metadata job for a library", func(t *testing.T) {
		fixed := newFixture(t)

		if _, err := fixed.expecting(t, metadata.RefreshMetadataJobID).RefreshItem(
			context.Background(),
			refreshRequest(fixed.libraryID, api.MetadataRefreshModeFullRefresh, true),
		); !errors.Is(err, jobs.ErrNotConfigured) {
			t.Fatalf("err = %v, want a library id accepted", err)
		}
	})

	t.Run("starts the scan when the mode asks to look for new files", func(t *testing.T) {
		fixed := newFixture(t)
		movie := fixed.add(t, seed{kind: itemmodal.KindMovie, name: "The Matrix"})

		if _, err := fixed.expecting(t, scanner.RefreshLibraryJobID).RefreshItem(
			context.Background(),
			api.RefreshItemRequestObject{
				ItemId: movie,
				Params: api.RefreshItemParams{
					MetadataRefreshMode: apiutil.Ptr(api.MetadataRefreshModeDefault),
				},
			},
		); !errors.Is(err, jobs.ErrNotConfigured) {
			t.Fatalf("err = %v, want only the queue missing", err)
		}
	})
}
