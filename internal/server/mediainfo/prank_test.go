package mediainfo

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/auth"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	"github.com/FreekingDean/gojellyfin/internal/sessions"
	itemmodel "github.com/FreekingDean/gojellyfin/internal/store/item"
)

type standIn struct {
	catalogue *items.Service
	id        uuid.UUID
	start     int64
}

func (s standIn) Swap(ctx context.Context, _ uuid.UUID, _ *items.Item) (*items.Item, bool, error) {
	item, err := s.catalogue.ItemByID(ctx, items.Everyone, s.id)

	return item, true, err
}

func (s standIn) Start(context.Context) (int64, error) {
	return s.start, nil
}

func TestServer_GetPostedPlaybackInfo_PlansTheStandIn(t *testing.T) {
	chrome := profileFrom(t, chromeProfile)
	fixture := newFixture(t)
	picked := fixture.addRip(t, itemmodel.KindMovie, "mkv", "aac")
	played := fixture.addRip(t, itemmodel.KindMovie, "mkv", "ac3")
	fixture.server.pranks = standIn{catalogue: fixture.server.items, id: played}
	fixture.server.sessions = sessions.New(fixture.client, nil)

	user := uuid.New()
	session := &sessions.Session{ID: uuid.New(), UserID: &user}
	ctx := auth.ContextWithSession(auth.ContextWithAuthorization(context.Background(), auth.Authorization{Token: "the-token"}), session)

	response, err := fixture.server.GetPostedPlaybackInfo(ctx, api.GetPostedPlaybackInfoRequestObject{ItemId: picked, JSONBody: firstPlay(&chrome)})
	if err != nil {
		t.Fatalf("failed to answer playback info: %v", err)
	}
	source := (*response.(api.GetPostedPlaybackInfo200JSONResponse).MediaSources)[0]

	files, err := fixture.server.items.MediaSources(ctx, played)
	if err != nil || len(files) != 1 {
		t.Fatalf("failed to read the stand-in's file: %v", err)
	}
	url := apiutil.Deref(source.TranscodingUrl)
	if !strings.Contains(url, "/Videos/"+picked.String()+"/") {
		t.Errorf("transcoding url = %q, want it to name the title the client picked", url)
	}
	if !strings.Contains(url, "mediaSourceId="+files[0].ID.String()) {
		t.Errorf("transcoding url = %q, want it to name the stand-in's file", url)
	}
	if got := apiutil.Deref(source.TranscodingSubProtocol); got != api.MediaStreamProtocolHls {
		t.Errorf("transcoding protocol = %q, want the stand-in's AC-3 planned as a conversion", got)
	}
}

func TestServer_GetPostedPlaybackInfo_StartsTheStandInAtItsStart(t *testing.T) {
	chrome := profileFrom(t, chromeProfile)
	user := uuid.New()
	session := &sessions.Session{ID: uuid.New(), UserID: &user}
	ctx := auth.ContextWithSession(auth.ContextWithAuthorization(context.Background(), auth.Authorization{Token: "the-token"}), session)

	plan := func(t *testing.T, container, codec string, start int64, indexed bool) api.MediaSourceInfo {
		t.Helper()

		fixture := newFixture(t)
		if !indexed {
			fixture.keyframes = nil
		}
		picked := fixture.addRip(t, itemmodel.KindEpisode, "mkv", "aac")
		played := fixture.addRip(t, itemmodel.KindMovie, container, codec)
		fixture.server.pranks = standIn{catalogue: fixture.server.items, id: played, start: start}
		fixture.server.sessions = sessions.New(fixture.client, nil)

		response, err := fixture.server.GetPostedPlaybackInfo(ctx, api.GetPostedPlaybackInfoRequestObject{ItemId: picked, JSONBody: firstPlay(&chrome)})
		if err != nil {
			t.Fatalf("failed to answer playback info: %v", err)
		}

		return (*response.(api.GetPostedPlaybackInfo200JSONResponse).MediaSources)[0]
	}

	t.Run("a converted stand-in's playlist starts at the start", func(t *testing.T) {
		source := plan(t, "mkv", "ac3", 30_000_000, true)
		if url := apiutil.Deref(source.TranscodingUrl); !strings.Contains(url, "fromTicks=30000000") {
			t.Errorf("transcoding url = %q, want the playlist cut from the start", url)
		}
	})

	t.Run("a stand-in the client plays untouched is segmented so it can start part way", func(t *testing.T) {
		source := plan(t, "mp4", "aac", 30_000_000, true)
		if got := apiutil.Deref(source.TranscodingSubProtocol); got != api.MediaStreamProtocolHls {
			t.Errorf("transcoding protocol = %q, want hls so the start can be cut", got)
		}
		if apiutil.Deref(source.SupportsDirectPlay) {
			t.Error("direct play offered, want the client kept off the whole file")
		}
		if url := apiutil.Deref(source.TranscodingUrl); !strings.Contains(url, "fromTicks=30000000") {
			t.Errorf("transcoding url = %q, want the playlist cut from the start", url)
		}
	})

	t.Run("a stand-in with no keyframes yet is encoded so it can start part way", aStandInWithNoKeyframesYetIsEncodedSoItCanStartPartWay(plan))

	t.Run("no start leaves an untouched stand-in to direct play", func(t *testing.T) {
		source := plan(t, "mp4", "aac", 0, true)
		if !apiutil.Deref(source.SupportsDirectPlay) {
			t.Error("direct play refused, want the file played whole when nothing is cut")
		}
		if url := apiutil.Deref(source.TranscodingUrl); strings.Contains(url, "fromTicks") {
			t.Errorf("transcoding url = %q, want nothing cut", url)
		}
	})
}

func aStandInWithNoKeyframesYetIsEncodedSoItCanStartPartWay(plan func(t *testing.T, container string, codec string, start int64, indexed bool) api.MediaSourceInfo) func(*testing.T) {
	return func(t *testing.T) {
		source := plan(t, "mp4", "aac", 30_000_000, false)
		if got := apiutil.Deref(source.TranscodingSubProtocol); got != api.MediaStreamProtocolHls {
			t.Errorf("transcoding protocol = %q, want hls so the start can be cut", got)
		}
		url := apiutil.Deref(source.TranscodingUrl)
		if !strings.Contains(url, "videoCodec=h264") {
			t.Errorf("transcoding url = %q, want the picture encoded since no keyframe says where to cut", url)
		}
		if !strings.Contains(url, "fromTicks=30000000") {
			t.Errorf("transcoding url = %q, want the playlist cut from the start", url)
		}
	}
}
