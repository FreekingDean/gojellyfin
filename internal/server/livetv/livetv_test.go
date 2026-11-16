package livetv

import (
	"context"
	"encoding/json"
	"math/rand/v2"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/channels"
	"github.com/FreekingDean/gojellyfin/internal/env"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	"github.com/FreekingDean/gojellyfin/internal/store"
	channelmodel "github.com/FreekingDean/gojellyfin/internal/store/channel"
	itemmodel "github.com/FreekingDean/gojellyfin/internal/store/item"
	sourcemodel "github.com/FreekingDean/gojellyfin/internal/store/source"
	"github.com/FreekingDean/gojellyfin/internal/users"
)

type everyone struct{}

func (everyone) Access(context.Context, uuid.UUID) (users.Access, error) {
	return users.Access{All: true}, nil
}

type fixture struct {
	server   *Server
	client   *store.Client
	name     string
	movies   []uuid.UUID
	channels []uuid.UUID
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	ctx := context.Background()

	config, err := env.Load()
	if err != nil {
		t.Fatalf("failed to read the environment: %v", err)
	}
	connection, err := store.NewStore(config)
	if err != nil {
		t.Fatalf("failed to open the database: %v", err)
	}
	if err := connection.Start(); err != nil {
		t.Fatalf("failed to reach the database, set DATABASE_URL: %v", err)
	}
	client := connection.Client()

	name := t.Name() + "-" + uuid.NewString()
	downloader, err := client.Source.Create().
		SetName(name).
		SetURL("http://" + uuid.NewString() + ".invalid").
		SetAPIKeyVariable("SOURCE_API_KEY_TEST").
		SetKind(sourcemodel.KindRadarr).
		Save(ctx)
	if err != nil {
		t.Fatalf("failed to create the source: %v", err)
	}

	catalogue := items.New(client)
	f := &fixture{server: New(channels.New(client, catalogue), catalogue, everyone{}), client: client, name: name}
	for i, minutes := range []int{40, 50} {
		key := name + ":" + strconv.Itoa(i)
		movie, err := client.Item.Create().SetKind(itemmodel.KindMovie).SetKey(key).SetName(key).SetSortName(key).Save(ctx)
		if err != nil {
			t.Fatalf("failed to create a movie: %v", err)
		}
		f.movies = append(f.movies, movie.ID)
		if _, err := client.ItemSource.Create().
			SetItemID(movie.ID).
			SetSourceID(downloader.ID).
			SetName(key).
			SetPath("/media/" + uuid.NewString() + ".mkv").
			SetRunTimeTicks(int64(time.Duration(minutes) * time.Minute / 100)).
			Save(ctx); err != nil {
			t.Fatalf("failed to create a file: %v", err)
		}
	}

	t.Cleanup(func() {
		ctx := context.Background()
		if _, err := client.Channel.Delete().Where(channelmodel.IDIn(f.channels...)).Exec(ctx); err != nil {
			t.Errorf("failed to delete the channels: %v", err)
		}
		if _, err := client.Item.Delete().Where(itemmodel.IDIn(f.movies...)).Exec(ctx); err != nil {
			t.Errorf("failed to delete the movies: %v", err)
		}
		if err := client.Source.DeleteOne(downloader).Exec(ctx); err != nil {
			t.Errorf("failed to delete the source: %v", err)
		}
		if err := connection.Stop(); err != nil {
			t.Errorf("failed to close the database: %v", err)
		}
	})

	return f
}

func (f *fixture) create(t *testing.T) api.LiveChannel {
	t.Helper()

	response, err := f.server.GoJellyfinCreateChannel(context.Background(), api.GoJellyfinCreateChannelRequestObject{Body: &api.LiveChannelDefinition{
		Number:  rand.Int32N(60000) + 1000,
		Name:    f.name,
		Order:   api.LiveChannelOrderSequential,
		ItemIds: f.movies,
	}})
	if err != nil {
		t.Fatalf("GoJellyfinCreateChannel: %v", err)
	}
	created, ok := response.(api.GoJellyfinCreateChannel200JSONResponse)
	if !ok {
		t.Fatalf("GoJellyfinCreateChannel answered %T", response)
	}
	f.channels = append(f.channels, created.Id)

	return api.LiveChannel(created)
}

func TestServer_ChannelsCarryWhatIsOn(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	channel := f.create(t)

	response, err := f.server.GetLiveTvChannels(ctx, api.GetLiveTvChannelsRequestObject{})
	if err != nil {
		t.Fatalf("GetLiveTvChannels: %v", err)
	}
	var found *api.BaseItemDto
	for _, listed := range *response.(api.GetLiveTvChannels200JSONResponse).Items {
		if *listed.Id == channel.Id {
			found = &listed
		}
	}
	if found == nil {
		t.Fatal("the channel is not listed")
	}
	if *found.Type != api.BaseItemKindTvChannel || *found.ChannelNumber != strconv.Itoa(int(channel.Number)) {
		t.Errorf("channel = %s %s", *found.Type, *found.ChannelNumber)
	}
	current := found.CurrentProgram
	if current == nil {
		t.Fatal("the channel carries no current program")
	}
	if *current.Type != api.BaseItemKindProgram || *current.ChannelId != channel.Id || !strings.HasPrefix(*current.Name, f.name) {
		t.Errorf("current program = %s %s %s", *current.Type, *current.ChannelId, *current.Name)
	}
	if now := time.Now(); current.StartDate.After(now) || !current.EndDate.After(now) {
		t.Errorf("current program runs %s to %s, not now", current.StartDate, current.EndDate)
	}

	byID, err := f.server.GetChannel(ctx, api.GetChannelRequestObject{ChannelId: channel.Id})
	if err != nil {
		t.Fatalf("GetChannel: %v", err)
	}
	if _, ok := byID.(api.GetChannel200JSONResponse); !ok {
		t.Errorf("GetChannel answered %T", byID)
	}
	missing, err := f.server.GetChannel(ctx, api.GetChannelRequestObject{ChannelId: uuid.New()})
	if err != nil {
		t.Fatalf("GetChannel: %v", err)
	}
	if _, ok := missing.(api.GetChannel404JSONResponse); !ok {
		t.Errorf("GetChannel of nothing answered %T", missing)
	}
}

func TestServer_GuideIsContiguousAndResolvable(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	channel := f.create(t)

	from := time.Now()
	to := from.Add(4 * time.Hour)
	response, err := f.server.GetPrograms(ctx, api.GetProgramsRequestObject{JSONBody: &api.GetProgramsDto{
		ChannelIds:   &[]uuid.UUID{channel.Id},
		MinEndDate:   &from,
		MaxStartDate: &to,
	}})
	if err != nil {
		t.Fatalf("GetPrograms: %v", err)
	}
	programs := *response.(api.GetPrograms200JSONResponse).Items
	if len(programs) < 5 {
		t.Fatalf("four hours of forty and fifty minute films listed %d programs", len(programs))
	}
	for i := 1; i < len(programs); i++ {
		if !programs[i].StartDate.Equal(*programs[i-1].EndDate) {
			t.Errorf("program %d starts at %s, the last ended at %s", i, programs[i].StartDate, programs[i-1].EndDate)
		}
	}
	if programs[0].StartDate.After(from) || programs[len(programs)-1].StartDate.After(to) {
		t.Error("the guide runs outside the window asked for")
	}

	program, err := f.server.GetProgram(ctx, api.GetProgramRequestObject{ProgramId: programs[2].Id.String()})
	if err != nil {
		t.Fatalf("GetProgram: %v", err)
	}
	resolved, ok := program.(api.GetProgram200JSONResponse)
	if !ok || *resolved.Name != *programs[2].Name || !resolved.StartDate.Equal(*programs[2].StartDate) {
		t.Errorf("GetProgram answered %T %v", program, resolved.Name)
	}
	unknown, err := f.server.GetProgram(ctx, api.GetProgramRequestObject{ProgramId: "nonsense"})
	if err != nil {
		t.Fatalf("GetProgram: %v", err)
	}
	if _, ok := unknown.(api.GetProgram404JSONResponse); !ok {
		t.Errorf("GetProgram of nonsense answered %T", unknown)
	}

	airing, err := f.server.GetLiveTvPrograms(ctx, api.GetLiveTvProgramsRequestObject{Params: api.GetLiveTvProgramsParams{
		ChannelIds: &[]uuid.UUID{channel.Id},
		IsAiring:   apiutil.Ptr(true),
	}})
	if err != nil {
		t.Fatalf("GetLiveTvPrograms: %v", err)
	}
	if count := len(*airing.(api.GetLiveTvPrograms200JSONResponse).Items); count != 1 {
		t.Errorf("one channel is airing %d programs", count)
	}
}

func TestServer_ManageChannels(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	channel := f.create(t)

	refused, err := f.server.GoJellyfinCreateChannel(ctx, api.GoJellyfinCreateChannelRequestObject{Body: &api.LiveChannelDefinition{
		Number: channel.Number, Name: f.name + "-again", Order: api.LiveChannelOrderShuffle, ItemIds: f.movies,
	}})
	if err != nil {
		t.Fatalf("GoJellyfinCreateChannel: %v", err)
	}
	if _, ok := refused.(api.GoJellyfinCreateChannel400JSONResponse); !ok {
		t.Errorf("a second channel on one number answered %T", refused)
	}

	updated, err := f.server.GoJellyfinUpdateChannel(ctx, api.GoJellyfinUpdateChannelRequestObject{ChannelId: channel.Id, Body: &api.LiveChannelDefinition{
		Number: channel.Number, Name: f.name + "-renamed", Order: api.LiveChannelOrderShuffle, ItemIds: f.movies[:1],
	}})
	if err != nil {
		t.Fatalf("GoJellyfinUpdateChannel: %v", err)
	}
	if saved, ok := updated.(api.GoJellyfinUpdateChannel200JSONResponse); !ok || saved.Name != f.name+"-renamed" || len(saved.ItemIds) != 1 {
		t.Errorf("GoJellyfinUpdateChannel answered %#v", updated)
	}

	listed, err := f.server.GoJellyfinListChannels(ctx, api.GoJellyfinListChannelsRequestObject{})
	if err != nil {
		t.Fatalf("GoJellyfinListChannels: %v", err)
	}
	found := false
	for _, candidate := range listed.(api.GoJellyfinListChannels200JSONResponse) {
		found = found || candidate.Id == channel.Id
	}
	if !found {
		t.Error("the channel is not listed")
	}

	for _, want := range []any{api.GoJellyfinDeleteChannel204Response{}, api.GoJellyfinDeleteChannel404Response{}} {
		deleted, err := f.server.GoJellyfinDeleteChannel(ctx, api.GoJellyfinDeleteChannelRequestObject{ChannelId: channel.Id})
		if err != nil {
			t.Fatalf("GoJellyfinDeleteChannel: %v", err)
		}
		if deleted != want {
			t.Errorf("GoJellyfinDeleteChannel answered %T, want %T", deleted, want)
		}
	}
}

func TestServer_RecordingsAreEmpty(t *testing.T) {
	s := &Server{}
	ctx := context.Background()

	for _, tc := range []struct {
		name string
		call func() (any, error)
	}{
		{"recordings", func() (any, error) { return s.GetRecordings(ctx, api.GetRecordingsRequestObject{}) }},
		{"recording folders", func() (any, error) {
			return s.GetRecordingFolders(ctx, api.GetRecordingFoldersRequestObject{})
		}},
		{"timers", func() (any, error) { return s.GetTimers(ctx, api.GetTimersRequestObject{}) }},
		{"series timers", func() (any, error) { return s.GetSeriesTimers(ctx, api.GetSeriesTimersRequestObject{}) }},
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

			for _, want := range []string{`"Items":[]`, `"TotalRecordCount":0`} {
				if !strings.Contains(string(encoded), want) {
					t.Errorf("got %s, want it to contain %s", encoded, want)
				}
			}
		})
	}
}
