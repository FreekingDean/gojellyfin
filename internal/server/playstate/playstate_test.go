package playstate

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/channels"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	channelmodel "github.com/FreekingDean/gojellyfin/internal/store/channel"
)

type onAir struct {
	channel *channels.Channel
}

func (o onAir) Channel(_ context.Context, _ items.Viewer, id uuid.UUID) (*channels.Channel, error) {
	if id != o.channel.ID {
		return nil, channels.ErrNotFound
	}

	return o.channel, nil
}

func (o onAir) Schedule(context.Context, *channels.Channel) (channels.Schedule, error) {
	slots := []channels.Slot{{ItemID: uuid.New(), SourceID: uuid.New(), Length: time.Hour}}

	return channels.NewSchedule(channelmodel.OrderSequential, 1, time.Unix(0, 0), slots), nil
}

func (o onAir) Now() time.Time {
	return time.Unix(0, 0).Add(10 * time.Minute)
}

func TestServer_AChannelKeepsNoWatchState(t *testing.T) {
	channel := &channels.Channel{ID: uuid.New(), Number: 1, Name: "Fixture"}
	server := &Server{channels: onAir{channel: channel}}
	ctx := context.Background()
	report := &api.PlaybackProgressInfo{ItemId: &channel.ID, PositionTicks: apiutil.Ptr(int64(1))}

	if _, err := server.ReportPlaybackStart(ctx, api.ReportPlaybackStartRequestObject{JSONBody: &api.PlaybackStartInfo{ItemId: &channel.ID}}); err != nil {
		t.Errorf("start: %v", err)
	}
	if _, err := server.ReportPlaybackProgress(ctx, api.ReportPlaybackProgressRequestObject{JSONBody: report}); err != nil {
		t.Errorf("progress: %v", err)
	}
	if _, err := server.ReportPlaybackStopped(ctx, api.ReportPlaybackStoppedRequestObject{JSONBody: &api.PlaybackStopInfo{ItemId: &channel.ID}}); err != nil {
		t.Errorf("stop: %v", err)
	}
}
