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
}

func (s standIn) Swap(ctx context.Context, _ uuid.UUID, _ *items.Item) (*items.Item, bool, error) {
	item, err := s.catalogue.ItemByID(ctx, items.Everyone, s.id)

	return item, true, err
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
