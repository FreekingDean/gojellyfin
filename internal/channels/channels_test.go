package channels

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"testing"
	"time"

	"github.com/FreekingDean/gojellyfin/internal/storetest"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/store"
	channelmodel "github.com/FreekingDean/gojellyfin/internal/store/channel"
	itemmodel "github.com/FreekingDean/gojellyfin/internal/store/item"
	sourcemodel "github.com/FreekingDean/gojellyfin/internal/store/source"
)

type fixture struct {
	client     *store.Client
	service    *Service
	downloader uuid.UUID
	name       string
	created    []uuid.UUID
	channels   []uuid.UUID
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	ctx := context.Background()

	connection := storetest.Open(t)
	client := connection.Client()

	name := t.Name() + "-" + uuid.NewString()
	downloader, err := client.Source.Create().
		SetName(name).
		SetURL("http://" + uuid.NewString() + ".invalid").
		SetAPIKeyVariable("SOURCE_API_KEY_TEST").
		SetKind(sourcemodel.KindSonarr).
		Save(ctx)
	if err != nil {
		t.Fatalf("failed to create the source: %v", err)
	}

	f := &fixture{client: client, service: New(client, items.New(client)), downloader: downloader.ID, name: name}
	t.Cleanup(func() {
		ctx := context.Background()
		if _, err := client.Channel.Delete().Where(channelmodel.IDIn(f.channels...)).Exec(ctx); err != nil {
			t.Errorf("failed to delete the channels: %v", err)
		}
		for i := len(f.created) - 1; i >= 0; i-- {
			if err := client.Item.DeleteOneID(f.created[i]).Exec(ctx); err != nil && !store.IsNotFound(err) {
				t.Errorf("failed to delete an item: %v", err)
			}
		}
		if err := client.Source.DeleteOne(downloader).Exec(ctx); err != nil {
			t.Errorf("failed to delete the source: %v", err)
		}
	})

	return f
}

func (f *fixture) item(t *testing.T, kind items.Kind, parent *uuid.UUID, index int32, minutes int) uuid.UUID {
	t.Helper()

	return f.episode(t, kind, parent, 0, index, minutes)
}

func (f *fixture) episode(t *testing.T, kind items.Kind, parent *uuid.UUID, season, index int32, minutes int) uuid.UUID {
	t.Helper()
	ctx := context.Background()

	key := fmt.Sprintf("%s:%s:%d:%d", f.name, kind, len(f.created), index)
	item, err := f.client.Item.Create().
		SetKind(kind).
		SetNillableParentID(parent).
		SetIndexNumber(index).
		SetParentIndexNumber(season).
		SetKey(key).SetName(key).SetSortName(key).
		Save(ctx)
	if err != nil {
		t.Fatalf("failed to create an item: %v", err)
	}
	f.created = append(f.created, item.ID)

	if minutes > 0 {
		_, err := f.client.ItemSource.Create().
			SetItemID(item.ID).
			SetSourceID(f.downloader).
			SetName(key).
			SetPath("/media/" + uuid.NewString() + ".mkv").
			SetRunTimeTicks(int64(time.Duration(minutes) * time.Minute / tick)).
			Save(ctx)
		if err != nil {
			t.Fatalf("failed to create a file: %v", err)
		}
	}

	return item.ID
}

func (f *fixture) create(t *testing.T, definition Definition) *Channel {
	t.Helper()

	channel, err := f.service.Create(context.Background(), definition)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	f.channels = append(f.channels, channel.ID)

	return channel
}

func number() int32 {
	return rand.Int32N(60000) + 1000
}

func TestSchedule_AirsEpisodesInOrderThenMovies(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	series := f.item(t, itemmodel.KindSeries, nil, 0, 0)
	second := f.item(t, itemmodel.KindSeason, &series, 2, 0)
	first := f.item(t, itemmodel.KindSeason, &series, 1, 0)
	s2e1 := f.episode(t, itemmodel.KindEpisode, &second, 2, 1, 20)
	s1e2 := f.episode(t, itemmodel.KindEpisode, &first, 1, 2, 20)
	s1e1 := f.episode(t, itemmodel.KindEpisode, &first, 1, 1, 20)
	f.episode(t, itemmodel.KindEpisode, &first, 1, 3, 0)
	movie := f.item(t, itemmodel.KindMovie, nil, 0, 100)

	channel := f.create(t, Definition{Number: number(), Name: f.name, Order: channelmodel.OrderSequential, ItemIDs: []uuid.UUID{series, movie}})

	schedule, err := f.service.Schedule(ctx, channel)
	if err != nil {
		t.Fatalf("Schedule: %v", err)
	}
	airings := schedule.Between(channel.Epoch, channel.Epoch.Add(160*time.Minute))

	want := []uuid.UUID{s1e1, s1e2, s2e1, movie}
	if len(airings) != len(want) {
		t.Fatalf("aired %d titles, want %d", len(airings), len(want))
	}
	for i, id := range want {
		if airings[i].ItemID != id {
			t.Errorf("airing %d is %s, want %s", i, airings[i].ItemID, id)
		}
	}
	if airings[3].End.Sub(airings[3].Start) != 100*time.Minute {
		t.Errorf("the movie airs for %s", airings[3].End.Sub(airings[3].Start))
	}
}

func TestProgram_ResolvesWhatAirs(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	movie := f.item(t, itemmodel.KindMovie, nil, 0, 90)
	channel := f.create(t, Definition{Number: number(), Name: f.name, Order: channelmodel.OrderShuffle, ItemIDs: []uuid.UUID{movie}})

	programs, err := f.service.Programs(ctx, Guide{
		Viewer:     items.Everyone,
		ChannelIDs: []uuid.UUID{channel.ID},
		From:       channel.Epoch.Add(10 * time.Hour),
		To:         channel.Epoch.Add(13 * time.Hour),
	})
	if err != nil {
		t.Fatalf("Programs: %v", err)
	}
	if len(programs) != 3 {
		t.Fatalf("listed %d programs, want 3", len(programs))
	}

	program, err := f.service.Program(ctx, items.Everyone, programs[1].ID())
	if err != nil {
		t.Fatalf("Program: %v", err)
	}
	if program.ItemID != movie || !program.Start.Equal(programs[1].Start) {
		t.Errorf("Program = %s at %s, want %s at %s", program.ItemID, program.Start, movie, programs[1].Start)
	}

	if _, err := f.service.Program(ctx, items.Everyone, ProgramID(channel.Number, programs[1].Start.Add(time.Minute))); !errors.Is(err, ErrNotFound) {
		t.Errorf("a program starting at no airing answered %v", err)
	}
}

func TestList_HidesAChannelTheViewerCannotSeeAllOf(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	movie := f.item(t, itemmodel.KindMovie, nil, 0, 90)
	channel := f.create(t, Definition{Number: number(), Name: f.name, Order: channelmodel.OrderShuffle, ItemIDs: []uuid.UUID{movie}})

	if _, err := f.service.Channel(ctx, items.Viewer{Libraries: []uuid.UUID{uuid.New()}}, channel.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("a viewer of another library found the channel: %v", err)
	}
	if _, err := f.service.Channel(ctx, items.Everyone, channel.ID); err != nil {
		t.Errorf("an administrator could not find the channel: %v", err)
	}
}

func TestCreate_RefusesWhatCannotAir(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	movie := f.item(t, itemmodel.KindMovie, nil, 0, 90)
	taken := f.create(t, Definition{Number: number(), Name: f.name, Order: channelmodel.OrderShuffle, ItemIDs: []uuid.UUID{movie}})

	cases := map[string]Definition{
		"no number":    {Name: "a", Order: channelmodel.OrderShuffle, ItemIDs: []uuid.UUID{movie}},
		"no name":      {Number: number(), Order: channelmodel.OrderShuffle, ItemIDs: []uuid.UUID{movie}},
		"no titles":    {Number: number(), Name: "b", Order: channelmodel.OrderShuffle},
		"twice":        {Number: number(), Name: "c", Order: channelmodel.OrderShuffle, ItemIDs: []uuid.UUID{movie, movie}},
		"unknown":      {Number: number(), Name: "d", Order: channelmodel.OrderShuffle, ItemIDs: []uuid.UUID{uuid.New()}},
		"bad order":    {Number: number(), Name: "e", Order: "Random", ItemIDs: []uuid.UUID{movie}},
		"taken number": {Number: taken.Number, Name: f.name + "-other", Order: channelmodel.OrderShuffle, ItemIDs: []uuid.UUID{movie}},
	}
	for name, definition := range cases {
		if _, err := f.service.Create(ctx, definition); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: Create answered %v, want ErrInvalid", name, err)
		}
	}
}

func TestUpdate_KeepsTheScheduleSeed(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	movie := f.item(t, itemmodel.KindMovie, nil, 0, 90)
	other := f.item(t, itemmodel.KindMovie, nil, 0, 30)
	channel := f.create(t, Definition{Number: number(), Name: f.name, Order: channelmodel.OrderShuffle, ItemIDs: []uuid.UUID{movie}})

	updated, err := f.service.Update(ctx, channel.ID, Definition{Number: channel.Number, Name: f.name + "-renamed", Order: channelmodel.OrderSequential, ItemIDs: []uuid.UUID{other, movie}})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Seed != channel.Seed || !updated.Epoch.Equal(channel.Epoch) {
		t.Error("an update moved the schedule's seed or epoch")
	}
	if ids := EntryIDs(updated); len(ids) != 2 || ids[0] != other {
		t.Errorf("entries after update = %v", ids)
	}

	if err := f.client.Item.DeleteOneID(other).Exec(ctx); err != nil {
		t.Fatalf("failed to delete a title: %v", err)
	}
	remaining, err := f.service.Channel(ctx, items.Everyone, channel.ID)
	if err != nil {
		t.Fatalf("Channel: %v", err)
	}
	if ids := EntryIDs(remaining); len(ids) != 1 || ids[0] != movie {
		t.Errorf("deleting a title left entries %v", ids)
	}

	if err := f.service.Delete(ctx, channel.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if err := f.service.Delete(ctx, channel.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("a second Delete answered %v", err)
	}
}

func TestSchedule_InterleaveSpreadsEachTitle(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	series := f.item(t, itemmodel.KindSeries, nil, 0, 0)
	season := f.item(t, itemmodel.KindSeason, &series, 1, 0)
	e1 := f.episode(t, itemmodel.KindEpisode, &season, 1, 1, 20)
	e2 := f.episode(t, itemmodel.KindEpisode, &season, 1, 2, 20)
	e3 := f.episode(t, itemmodel.KindEpisode, &season, 1, 3, 20)
	movie := f.item(t, itemmodel.KindMovie, nil, 0, 100)

	channel := f.create(t, Definition{Number: number(), Name: f.name, Order: channelmodel.OrderInterleave, ItemIDs: []uuid.UUID{series, movie}})

	schedule, err := f.service.Schedule(ctx, channel)
	if err != nil {
		t.Fatalf("Schedule: %v", err)
	}
	airings := schedule.Between(channel.Epoch, channel.Epoch.Add(160*time.Minute))

	want := []uuid.UUID{e1, e2, movie, e3}
	if len(airings) != len(want) {
		t.Fatalf("aired %d titles, want %d", len(airings), len(want))
	}
	for i, id := range want {
		if airings[i].ItemID != id {
			t.Errorf("airing %d is %s, want %s", i, airings[i].ItemID, id)
		}
	}
}
