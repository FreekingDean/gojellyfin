package syncplay

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/auth"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	"github.com/FreekingDean/gojellyfin/internal/store"
	itemmodal "github.com/FreekingDean/gojellyfin/internal/store/item"
	groupmodal "github.com/FreekingDean/gojellyfin/internal/store/syncplaygroup"
	"github.com/FreekingDean/gojellyfin/internal/syncplay"
)

const minute = int64(60 * time.Second / 100)

func (f *fixture) title(t *testing.T, name string) uuid.UUID {
	t.Helper()

	ctx := context.Background()
	item, err := f.client.Item.Create().
		SetKind(itemmodal.KindMovie).
		SetKey("movie:test:" + uuid.NewString()).
		SetName(name).
		SetRunTimeTicks(90 * minute).
		Save(ctx)
	if err != nil {
		t.Fatalf("failed to create the %s item: %v", name, err)
	}

	t.Cleanup(func() {
		if err := f.client.Item.DeleteOneID(item.ID).Exec(ctx); err != nil && !store.IsNotFound(err) {
			t.Errorf("failed to delete the %s item: %v", name, err)
		}
	})

	return item.ID
}

func (r *recorder) commands() []published {
	r.mu.Lock()
	defer r.mu.Unlock()

	var matching []published
	for _, sent := range r.sent {
		if sent.messageType == syncplay.CommandMessage {
			matching = append(matching, sent)
		}
	}

	return matching
}

func (r *recorder) last(updateType string) published {
	sent := r.of(updateType)
	if len(sent) == 0 {
		return published{}
	}

	return sent[len(sent)-1]
}

func (r *recorder) lastCommand() published {
	sent := r.commands()
	if len(sent) == 0 {
		return published{}
	}

	return sent[len(sent)-1]
}

func (f *fixture) state(t *testing.T, ctx context.Context, id uuid.UUID) api.GroupStateType {
	t.Helper()

	return apiutil.Deref(f.group(t, ctx, id).State)
}

func (f *fixture) playing(t *testing.T, ctx context.Context) uuid.UUID {
	t.Helper()

	queue := f.published.last(syncplay.PlayQueueUpdateType)
	update, ok := queue.update.Data.(syncplay.PlayQueueUpdate)
	if !ok || update.PlayingItemIndex < 0 {
		t.Fatalf("no play queue has a playing item: %+v", queue)
	}

	return update.Playlist[update.PlayingItemIndex].PlaylistItemID
}

func (f *fixture) ready(t *testing.T, ctx context.Context, playlistItemID uuid.UUID, positionTicks int64) {
	t.Helper()

	if _, err := f.server.SyncPlayReady(ctx, api.SyncPlayReadyRequestObject{JSONBody: &api.ReadyRequestDto{
		When:           apiutil.Ptr(time.Now()),
		PositionTicks:  apiutil.Ptr(positionTicks),
		IsPlaying:      apiutil.Ptr(false),
		PlaylistItemId: &playlistItemID,
	}}); err != nil {
		t.Fatalf("failed to report ready: %v", err)
	}
}

type party struct {
	*fixture
	owner, guest context.Context
	group        uuid.UUID
	film         uuid.UUID
}

func newParty(t *testing.T) party {
	t.Helper()

	f := newFixture(t)
	p := party{fixture: f, owner: f.signIn(t, "owner"), guest: f.signIn(t, "guest")}
	p.group = *f.create(t, p.owner, t.Name()).GroupId
	f.join(t, p.guest, p.group)
	p.film = f.title(t, "film")

	return p
}

func (p party) play(t *testing.T, startTicks int64, films ...uuid.UUID) {
	t.Helper()

	if len(films) == 0 {
		films = []uuid.UUID{p.film}
	}

	if _, err := p.server.SyncPlaySetNewQueue(p.owner, api.SyncPlaySetNewQueueRequestObject{JSONBody: &api.PlayRequestDto{
		PlayingQueue:        &films,
		PlayingItemPosition: apiutil.Ptr(int32(0)),
		StartPositionTicks:  apiutil.Ptr(startTicks),
	}}); err != nil {
		t.Fatalf("failed to set the queue: %v", err)
	}
}

func (p party) bothReady(t *testing.T, positionTicks int64) {
	t.Helper()

	playlistItemID := p.playing(t, p.owner)
	p.ready(t, p.owner, playlistItemID, positionTicks)
	p.ready(t, p.guest, playlistItemID, positionTicks)
}

func TestServer_SyncPlayPlayback(t *testing.T) {
	t.Run("a new queue reaches every member and waits for them", func(t *testing.T) {
		p := newParty(t)

		p.play(t, 0)

		queue := p.published.last(syncplay.PlayQueueUpdateType)
		addressed(t, []published{queue}, auth.SessionFrom(p.owner).ID, auth.SessionFrom(p.guest).ID)

		update := queue.update.Data.(syncplay.PlayQueueUpdate)
		if update.Reason != syncplay.NewPlaylist || len(update.Playlist) != 1 || update.Playlist[0].ItemID != p.film {
			t.Fatalf("play queue = %+v, want a new playlist holding the film", update)
		}
		if state := p.state(t, p.owner, p.group); state != api.GroupStateType(syncplay.Waiting) {
			t.Fatalf("state = %q, want Waiting", state)
		}
	})

	t.Run("the group plays once every member is ready", func(t *testing.T) {
		p := newParty(t)
		p.play(t, 0)

		playlistItemID := p.playing(t, p.owner)
		p.ready(t, p.owner, playlistItemID, 0)

		if state := p.state(t, p.owner, p.group); state != api.GroupStateType(syncplay.Waiting) {
			t.Fatalf("state with one member ready = %q, want Waiting", state)
		}

		p.ready(t, p.guest, playlistItemID, 0)

		if state := p.state(t, p.owner, p.group); state != api.GroupStateType(syncplay.Playing) {
			t.Fatalf("state = %q, want Playing", state)
		}

		unpause := p.published.lastCommand()
		addressed(t, []published{unpause}, auth.SessionFrom(p.owner).ID, auth.SessionFrom(p.guest).ID)
		if unpause.command.Command != syncplay.UnpauseCommand || unpause.command.PlaylistItemID != playlistItemID {
			t.Fatalf("command = %+v, want Unpause of the playing item", unpause.command)
		}
		if !unpause.command.When.After(unpause.command.EmittedAt) {
			t.Fatalf("Unpause is scheduled at %v, want after it was emitted at %v", unpause.command.When, unpause.command.EmittedAt)
		}
	})

	t.Run("a pause stops everyone where the group had reached", func(t *testing.T) {
		p := newParty(t)
		p.play(t, 10*minute)
		p.bothReady(t, 10*minute)

		if _, err := p.server.SyncPlayPause(p.guest, api.SyncPlayPauseRequestObject{}); err != nil {
			t.Fatalf("failed to pause: %v", err)
		}

		if state := p.state(t, p.owner, p.group); state != api.GroupStateType(syncplay.Paused) {
			t.Fatalf("state = %q, want Paused", state)
		}

		pause := p.published.lastCommand()
		if pause.command.Command != syncplay.PauseCommand || pause.command.PositionTicks < 10*minute {
			t.Fatalf("command = %+v, want Pause at or after ten minutes", pause.command)
		}

		update := p.published.last(syncplay.StateUpdateType).update.Data.(syncplay.StateUpdate)
		if update.State != syncplay.Paused || update.Reason != syncplay.Pause {
			t.Fatalf("state update = %+v, want Paused by Pause", update)
		}
	})

	t.Run("a seek sends everyone to the position and waits for them again", func(t *testing.T) {
		p := newParty(t)
		p.play(t, 0)
		p.bothReady(t, 0)

		if _, err := p.server.SyncPlaySeek(p.owner, api.SyncPlaySeekRequestObject{JSONBody: &api.SeekRequestDto{
			PositionTicks: apiutil.Ptr(30 * minute),
		}}); err != nil {
			t.Fatalf("failed to seek: %v", err)
		}

		seek := p.published.lastCommand()
		if seek.command.Command != syncplay.SeekCommand || seek.command.PositionTicks != 30*minute {
			t.Fatalf("command = %+v, want Seek to thirty minutes", seek.command)
		}
		if state := p.state(t, p.owner, p.group); state != api.GroupStateType(syncplay.Waiting) {
			t.Fatalf("state = %q, want Waiting", state)
		}

		p.bothReady(t, 30*minute)

		if state := p.state(t, p.owner, p.group); state != api.GroupStateType(syncplay.Playing) {
			t.Fatalf("state after both are ready = %q, want Playing", state)
		}
	})

	t.Run("a seek past the end is held to the runtime", func(t *testing.T) {
		p := newParty(t)
		p.play(t, 0)
		p.bothReady(t, 0)

		if _, err := p.server.SyncPlaySeek(p.owner, api.SyncPlaySeekRequestObject{JSONBody: &api.SeekRequestDto{
			PositionTicks: apiutil.Ptr(1000 * minute),
		}}); err != nil {
			t.Fatalf("failed to seek: %v", err)
		}

		if seek := p.published.lastCommand(); seek.command.PositionTicks != 90*minute {
			t.Fatalf("seek position = %d, want the runtime %d", seek.command.PositionTicks, 90*minute)
		}
	})

	t.Run("a stop returns the group to idle", func(t *testing.T) {
		p := newParty(t)
		p.play(t, 0)
		p.bothReady(t, 0)

		if _, err := p.server.SyncPlayStop(p.owner, api.SyncPlayStopRequestObject{}); err != nil {
			t.Fatalf("failed to stop: %v", err)
		}

		if state := p.state(t, p.owner, p.group); state != api.GroupStateType(syncplay.Idle) {
			t.Fatalf("state = %q, want Idle", state)
		}
		if stop := p.published.lastCommand(); stop.command.Command != syncplay.StopCommand || len(stop.sessionIDs) != 2 {
			t.Fatalf("command = %+v to %v, want Stop to both", stop.command, stop.sessionIDs)
		}
	})

	t.Run("next item moves the queue on and waits", func(t *testing.T) {
		p := newParty(t)
		sequel := p.title(t, "sequel")
		p.play(t, 0, p.film, sequel)
		p.bothReady(t, 0)

		current := p.playing(t, p.owner)
		if _, err := p.server.SyncPlayNextItem(p.owner, api.SyncPlayNextItemRequestObject{JSONBody: &api.NextItemRequestDto{
			PlaylistItemId: &current,
		}}); err != nil {
			t.Fatalf("failed to skip: %v", err)
		}

		update := p.published.last(syncplay.PlayQueueUpdateType).update.Data.(syncplay.PlayQueueUpdate)
		if update.Reason != syncplay.NextItemReason || update.PlayingItemIndex != 1 || update.Playlist[1].ItemID != sequel {
			t.Fatalf("play queue = %+v, want the sequel playing", update)
		}
	})

	t.Run("queued items land after the queue and the queue survives a reload", func(t *testing.T) {
		p := newParty(t)
		sequel := p.title(t, "sequel")
		p.play(t, 0)

		if _, err := p.server.SyncPlayQueue(p.guest, api.SyncPlayQueueRequestObject{JSONBody: &api.QueueRequestDto{
			ItemIds: &[]uuid.UUID{sequel},
			Mode:    apiutil.Ptr(api.GroupQueueMode(syncplay.QueueLast)),
		}}); err != nil {
			t.Fatalf("failed to queue: %v", err)
		}

		queued := p.published.last(syncplay.PlayQueueUpdateType).update.Data.(syncplay.PlayQueueUpdate)
		if queued.Reason != syncplay.QueueAppended || len(queued.Playlist) != 2 || queued.Playlist[1].ItemID != sequel {
			t.Fatalf("play queue = %+v, want the sequel queued second", queued)
		}

		if _, err := p.server.SyncPlaySetRepeatMode(p.owner, api.SyncPlaySetRepeatModeRequestObject{JSONBody: &api.SetRepeatModeRequestDto{
			Mode: apiutil.Ptr(api.GroupRepeatMode(syncplay.RepeatAll)),
		}}); err != nil {
			t.Fatalf("failed to set the repeat mode: %v", err)
		}

		reloaded := p.published.last(syncplay.PlayQueueUpdateType).update.Data.(syncplay.PlayQueueUpdate)
		if reloaded.RepeatMode != syncplay.RepeatAll || len(reloaded.Playlist) != 2 || reloaded.Playlist[0] != queued.Playlist[0] || reloaded.Playlist[1] != queued.Playlist[1] {
			t.Fatalf("play queue = %+v, want the same two entries repeating", reloaded)
		}
	})

	t.Run("a playback request from outside any group is told so", func(t *testing.T) {
		f := newFixture(t)
		loner := f.signIn(t, "loner")

		if _, err := f.server.SyncPlayPause(loner, api.SyncPlayPauseRequestObject{}); err != nil {
			t.Fatalf("failed to pause: %v", err)
		}

		addressed(t, f.published.of(notInGroup), auth.SessionFrom(loner).ID)
	})

	t.Run("a member who joins a playing group makes it wait for them", func(t *testing.T) {
		p := newParty(t)
		p.play(t, 0)
		p.bothReady(t, 0)

		late := p.signIn(t, "late")
		p.join(t, late, p.group)

		if state := p.state(t, p.owner, p.group); state != api.GroupStateType(syncplay.Waiting) {
			t.Fatalf("state = %q, want Waiting", state)
		}

		queue := p.published.last(syncplay.PlayQueueUpdateType)
		addressed(t, []published{queue}, auth.SessionFrom(late).ID)
	})

	t.Run("a member who never reports ready is given up on", func(t *testing.T) {
		p := newParty(t)
		p.play(t, 0)
		p.ready(t, p.owner, p.playing(t, p.owner), 0)

		if err := p.client.SyncPlayGroup.UpdateOneID(p.group).
			SetWaitDeadline(time.Now().Add(-time.Second)).
			Exec(context.Background()); err != nil {
			t.Fatalf("failed to expire the wait: %v", err)
		}

		if err := p.service.ExpireWaits(context.Background()); err != nil {
			t.Fatalf("failed to expire the waits: %v", err)
		}

		if state := p.state(t, p.owner, p.group); state != api.GroupStateType(syncplay.Playing) {
			t.Fatalf("state = %q, want Playing without the guest", state)
		}

		group, err := p.client.SyncPlayGroup.Query().Where(groupmodal.ID(p.group)).Only(context.Background())
		if err != nil {
			t.Fatalf("failed to read the group: %v", err)
		}
		if group.WaitDeadline != nil {
			t.Fatalf("wait deadline = %v, want cleared", group.WaitDeadline)
		}
	})

	t.Run("a queue a member cannot see keeps the group as it was", func(t *testing.T) {
		p := newParty(t)

		if _, err := p.client.UserPolicy.Create().
			SetUserID(auth.UserID(p.guest)).
			SetEnableAllFolders(false).
			Save(context.Background()); err != nil {
			t.Fatalf("failed to restrict the guest: %v", err)
		}

		p.play(t, 0)

		if state := p.state(t, p.owner, p.group); state != api.GroupStateType(syncplay.Idle) {
			t.Fatalf("state = %q, want Idle", state)
		}
		if sent := p.published.of(syncplay.PlayQueueUpdateType); len(sent) != 0 {
			t.Fatalf("published %d play queues, want none", len(sent))
		}
	})

	t.Run("a group playing what the joiner cannot see refuses them", func(t *testing.T) {
		p := newParty(t)
		p.play(t, 0)

		outsider := p.signIn(t, "outsider")
		if _, err := p.client.UserPolicy.Create().
			SetUserID(auth.UserID(outsider)).
			SetEnableAllFolders(false).
			Save(context.Background()); err != nil {
			t.Fatalf("failed to restrict the outsider: %v", err)
		}

		p.join(t, outsider, p.group)

		addressed(t, p.published.of(libraryAccessDenied), auth.SessionFrom(outsider).ID)

		response, err := p.server.SyncPlayGetGroup(outsider, api.SyncPlayGetGroupRequestObject{Id: p.group})
		if err != nil {
			t.Fatalf("failed to read the group: %v", err)
		}
		if _, ok := response.(api.SyncPlayGetGroup404JSONResponse); !ok {
			t.Fatalf("response = %T, want the group hidden from the outsider", response)
		}
	})
}
