package mediasegment

import (
	"context"
	"testing"

	"github.com/FreekingDean/gojellyfin/internal/store"
	"github.com/FreekingDean/gojellyfin/internal/storetest"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/ffmpeg"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/segments"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	itemmodel "github.com/FreekingDean/gojellyfin/internal/store/item"
	segmentmodel "github.com/FreekingDean/gojellyfin/internal/store/mediasegment"
	downloadermodel "github.com/FreekingDean/gojellyfin/internal/store/source"
	"github.com/FreekingDean/gojellyfin/internal/users"
)

func TestServer_GetItemSegments(t *testing.T) {
	ctx := context.Background()

	connection := storetest.Open(t)
	client := connection.Client()

	name := t.Name() + "-" + uuid.NewString()
	library, err := client.Library.Create().SetName(name).Save(ctx)
	if err != nil {
		t.Fatalf("failed to create the library: %v", err)
	}
	downloader, err := client.Source.Create().
		SetName(name).
		SetURL("http://" + uuid.NewString() + ".invalid").
		SetAPIKeyVariable("SOURCE_API_KEY_TEST").
		SetKind(downloadermodel.KindSonarr).
		Save(ctx)
	if err != nil {
		t.Fatalf("failed to create the source: %v", err)
	}

	catalogue := items.New(client)
	episode, err := catalogue.SaveScanned(ctx, items.Item{Kind: itemmodel.KindEpisode, Name: name, SortName: name, Key: "test:" + name})
	if err != nil {
		t.Fatalf("failed to save the episode: %v", err)
	}
	t.Cleanup(serverGetItemSegmentsCleanup(t, client, episode, downloader, library))
	if err := catalogue.SaveMembership(ctx, library.ID, downloader.ID, []uuid.UUID{episode.ID}); err != nil {
		t.Fatalf("failed to place the episode in the library: %v", err)
	}

	file, err := client.ItemSource.Create().SetItemID(episode.ID).SetSourceID(downloader.ID).SetName(name).SetPath("/media/" + name).Save(ctx)
	if err != nil {
		t.Fatalf("failed to create the file: %v", err)
	}
	for _, segment := range []serverGetItemSegmentsCase{
		{segmentmodel.KindIntro, segmentmodel.OriginFingerprint, 10, 20},
		{segmentmodel.KindIntro, segmentmodel.OriginChapter, 11, 21},
		{segmentmodel.KindOutro, segmentmodel.OriginChapter, 90, 100},
	} {
		err := client.MediaSegment.Create().SetItemSourceID(file.ID).SetKind(segment.kind).SetOrigin(segment.origin).
			SetStartTicks(segment.start).SetEndTicks(segment.end).Exec(ctx)
		if err != nil {
			t.Fatal(err)
		}
	}

	found := segments.New(client, ffmpeg.New())

	hidden, err := New(catalogue, found, access{}).GetItemSegments(ctx, api.GetItemSegmentsRequestObject{ItemId: episode.ID})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := hidden.(api.GetItemSegments404JSONResponse); !ok {
		t.Errorf("response = %T, want a 404 for a user who cannot see the library", hidden)
	}

	for _, id := range []uuid.UUID{episode.ID, file.ID} {
		response, err := New(catalogue, found, access{All: true}).GetItemSegments(ctx, api.GetItemSegmentsRequestObject{ItemId: id})
		if err != nil {
			t.Fatal(err)
		}
		listed, ok := response.(api.GetItemSegments200JSONResponse)
		if !ok {
			t.Fatalf("response = %T, want the segments", response)
		}
		if len(*listed.Items) != 2 || *listed.TotalRecordCount != 2 {
			t.Fatalf("segments = %d, want the intro and the outro", len(*listed.Items))
		}
		intro := (*listed.Items)[0]
		if *intro.Type != api.MediaSegmentTypeIntro || *intro.StartTicks != 11 || *intro.EndTicks != 21 || *intro.ItemId != episode.ID {
			t.Errorf("intro = %v %d-%d of %v, want the chapter's 11-21 of the episode", *intro.Type, *intro.StartTicks, *intro.EndTicks, *intro.ItemId)
		}
	}

	only := []api.MediaSegmentType{api.MediaSegmentTypeOutro}
	response, err := New(catalogue, found, access{All: true}).GetItemSegments(ctx, api.GetItemSegmentsRequestObject{
		ItemId: episode.ID,
		Params: api.GetItemSegmentsParams{IncludeSegmentTypes: &only},
	})
	if err != nil {
		t.Fatal(err)
	}
	if listed := response.(api.GetItemSegments200JSONResponse); len(*listed.Items) != 1 || *(*listed.Items)[0].Type != api.MediaSegmentTypeOutro {
		t.Errorf("segments = %v, want only the outro", *listed.Items)
	}
}

func serverGetItemSegmentsCleanup(t *testing.T, client *store.Client, episode *items.Item, downloader *store.Source, library *store.Library) func() {
	t.Helper()

	return func() {
		ctx := context.Background()
		if err := client.Item.DeleteOneID(episode.ID).Exec(ctx); err != nil {
			t.Errorf("failed to delete the episode: %v", err)
		}
		if err := client.Source.DeleteOne(downloader).Exec(ctx); err != nil {
			t.Errorf("failed to delete the source: %v", err)
		}
		if err := client.Library.DeleteOne(library).Exec(ctx); err != nil {
			t.Errorf("failed to delete the library: %v", err)
		}
	}
}

type serverGetItemSegmentsCase struct {
	kind       segmentmodel.Kind
	origin     segmentmodel.Origin
	start, end int64
}

type access users.Access

func (a access) Access(context.Context, uuid.UUID) (users.Access, error) {
	return users.Access(a), nil
}
