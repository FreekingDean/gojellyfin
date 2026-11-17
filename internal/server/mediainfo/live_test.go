package mediainfo

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/channels"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	channelmodel "github.com/FreekingDean/gojellyfin/internal/store/channel"
	itemmodel "github.com/FreekingDean/gojellyfin/internal/store/item"
	streammodel "github.com/FreekingDean/gojellyfin/internal/store/mediastream"
)

type onAir struct {
	channel  *channels.Channel
	schedule channels.Schedule
	now      time.Time
}

func (o onAir) Channel(_ context.Context, _ items.Viewer, id uuid.UUID) (*channels.Channel, error) {
	if o.channel == nil || id != o.channel.ID {
		return nil, channels.ErrNotFound
	}

	return o.channel, nil
}

func (o onAir) Schedule(context.Context, *channels.Channel) (channels.Schedule, error) {
	return o.schedule, nil
}

func (o onAir) Now() time.Time {
	return o.now
}

func TestServer_PlaybackInfoForAChannel(t *testing.T) {
	chrome := profileFrom(t, chromeProfile)
	epoch := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	tune := serverPlaybackInfoForAChannelTune(epoch)

	t.Run("a channel is one endless encoded HLS stream", aChannelIsOneEndlessEncodedHLSStream(tune, chrome))
	t.Run("a channel with nothing to air is no compatible stream", aChannelWithNothingToAirIsNoCompatibleStream(tune, chrome))
	t.Run("a title is still answered as itself", aTitleIsStillAnsweredAsItself(tune, chrome))
}

func aChannelIsOneEndlessEncodedHLSStream(tune tuner, chrome api.DeviceProfile) func(t *testing.T) {
	return func(t *testing.T) {
		fixture := newFixture(t)
		film := fixture.addRipped(t, itemmodel.KindMovie, "mkv", "hevc", "eac3")
		sources, err := fixture.server.items.MediaSources(context.Background(), film)
		if err != nil {
			t.Fatal(err)
		}
		id := tune(t, fixture, []channels.Slot{{ItemID: film, SourceID: sources[0].ID, Length: time.Hour}})

		source := fixture.source(t, id, firstPlay(&chrome))

		if apiutil.Deref(source.Id) != id.String() || apiutil.Deref(source.Name) != "Fixture Channel" {
			t.Errorf("source = %s %q, want the channel", apiutil.Deref(source.Id), apiutil.Deref(source.Name))
		}
		if !apiutil.Deref(source.IsInfiniteStream) || source.RunTimeTicks != nil {
			t.Errorf("infinite = %v with runtime %v, want an endless stream", apiutil.Deref(source.IsInfiniteStream), source.RunTimeTicks)
		}
		if apiutil.Deref(source.SupportsDirectPlay) || apiutil.Deref(source.SupportsDirectStream) || !apiutil.Deref(source.SupportsTranscoding) {
			t.Error("a channel was offered as something other than a stream the server cuts")
		}
		if got := apiutil.Deref(source.TranscodingSubProtocol); got != api.MediaStreamProtocolHls {
			t.Errorf("protocol = %q, want hls", got)
		}
		url := apiutil.Deref(source.TranscodingUrl)
		if !strings.HasPrefix(url, "/Videos/"+id.String()+"/master.m3u8?") || !strings.Contains(url, "api_key=") {
			t.Errorf("url = %q, want the channel's master playlist", url)
		}

		codecs := map[streammodel.Kind]string{}
		for _, stream := range apiutil.Deref(source.MediaStreams) {
			codecs[streammodel.Kind(apiutil.Deref(stream.Type))] = apiutil.Deref(stream.Codec)
		}
		if codecs[streammodel.KindVideo] != "h264" || codecs[streammodel.KindAudio] != "aac" {
			t.Errorf("streams = %v, want what the encode makes", codecs)
		}
	}
}

func aChannelWithNothingToAirIsNoCompatibleStream(tune tuner, chrome api.DeviceProfile) func(t *testing.T) {
	return func(t *testing.T) {
		fixture := newFixture(t)
		id := tune(t, fixture, nil)

		response := fixture.answer(t, id, firstPlay(&chrome))
		if apiutil.Deref(response.ErrorCode) != api.NoCompatibleStream || len(apiutil.Deref(response.MediaSources)) != 0 {
			t.Errorf("response = %+v, want no compatible stream", response)
		}
	}
}

func aTitleIsStillAnsweredAsItself(tune tuner, chrome api.DeviceProfile) func(t *testing.T) {
	return func(t *testing.T) {
		fixture := newFixture(t)
		film := fixture.addRip(t, itemmodel.KindMovie, "mkv", "ac3")
		tune(t, fixture, nil)

		source := fixture.source(t, film, firstPlay(&chrome))
		if apiutil.Deref(source.IsInfiniteStream) || !strings.HasPrefix(apiutil.Deref(source.TranscodingUrl), "/Videos/"+film.String()) {
			t.Errorf("the title was answered as %+v", source)
		}
	}
}

type tuner func(t *testing.T, fixture *fixture, slots []channels.Slot) uuid.UUID

func serverPlaybackInfoForAChannelTune(epoch time.Time) tuner {
	return func(t *testing.T, fixture *fixture, slots []channels.Slot) uuid.UUID {
		t.Helper()

		channel := &channels.Channel{ID: uuid.New(), Number: 3, Name: "Fixture Channel"}
		fixture.server.channels = onAir{
			channel:  channel,
			schedule: channels.NewSchedule(channelmodel.OrderSequential, 1, epoch, slots),
			now:      epoch.Add(90 * time.Minute),
		}

		return channel.ID
	}
}
