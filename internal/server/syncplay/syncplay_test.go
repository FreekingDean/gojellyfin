package syncplay

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/auth"
	"github.com/FreekingDean/gojellyfin/internal/env"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	"github.com/FreekingDean/gojellyfin/internal/sessions"
	"github.com/FreekingDean/gojellyfin/internal/store"
	devicemodal "github.com/FreekingDean/gojellyfin/internal/store/device"
	sessionmodal "github.com/FreekingDean/gojellyfin/internal/store/session"
	groupmodal "github.com/FreekingDean/gojellyfin/internal/store/syncplaygroup"
	usermodal "github.com/FreekingDean/gojellyfin/internal/store/user"
	"github.com/FreekingDean/gojellyfin/internal/syncplay"
)

type published struct {
	sessionIDs []uuid.UUID
	update     groupUpdate
}

type recorder struct {
	mu   sync.Mutex
	sent []published
}

func (r *recorder) Publish(_ context.Context, sessionIDs []uuid.UUID, messageType string, data any) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if messageType != groupUpdateMessage {
		return fmt.Errorf("unexpected message type %q", messageType)
	}

	r.sent = append(r.sent, published{sessionIDs: sessionIDs, update: data.(groupUpdate)})

	return nil
}

func (r *recorder) of(updateType string) []published {
	r.mu.Lock()
	defer r.mu.Unlock()

	var matching []published
	for _, sent := range r.sent {
		if sent.update.Type == updateType {
			matching = append(matching, sent)
		}
	}

	return matching
}

type fixture struct {
	server    *Server
	client    *store.Client
	auth      *auth.Service
	sessions  *sessions.Service
	published *recorder
	users     []uuid.UUID
	devices   []string
}

func newFixture(t *testing.T) *fixture {
	t.Helper()

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

	ctx := context.Background()
	client := connection.Client()
	sessionService := sessions.New(client)

	recorded := &recorder{}
	f := &fixture{
		server:    New(syncplay.New(client), recorded),
		client:    client,
		auth:      auth.New(sessionService),
		sessions:  sessionService,
		published: recorded,
	}

	t.Cleanup(func() {
		if _, err := client.Session.Delete().
			Where(sessionmodal.HasUserWith(usermodal.IDIn(f.users...))).
			Exec(ctx); err != nil {
			t.Errorf("failed to delete the sessions: %v", err)
		}
		if _, err := client.Device.Delete().Where(devicemodal.ClientIDIn(f.devices...)).Exec(ctx); err != nil {
			t.Errorf("failed to delete the devices: %v", err)
		}
		if _, err := client.User.Delete().Where(usermodal.IDIn(f.users...)).Exec(ctx); err != nil {
			t.Errorf("failed to delete the users: %v", err)
		}
		if _, err := client.SyncPlayGroup.Delete().Where(groupmodal.NameHasPrefix(t.Name())).Exec(ctx); err != nil {
			t.Errorf("failed to delete the groups: %v", err)
		}
		if err := connection.Stop(); err != nil {
			t.Errorf("failed to close the database: %v", err)
		}
	})

	return f
}

func (f *fixture) signIn(t *testing.T, name string) context.Context {
	t.Helper()

	ctx := context.Background()
	unique := uuid.NewString()

	user, err := f.client.User.Create().
		SetName(name).
		SetUsername(fmt.Sprintf("%s-%s-%s", t.Name(), name, unique)).
		SetPasswordHash("hash").
		Save(ctx)
	if err != nil {
		t.Fatalf("failed to create the %s user: %v", name, err)
	}
	f.users = append(f.users, user.ID)

	device := sessions.DeviceInfo{ID: unique, Name: name, AppName: "test", AppVersion: "1"}
	f.devices = append(f.devices, unique)

	if _, err := f.sessions.Create(ctx, user.ID, unique, device); err != nil {
		t.Fatalf("failed to create the %s session: %v", name, err)
	}

	authenticated, err := f.auth.Authenticate(ctx, unique)
	if err != nil {
		t.Fatalf("failed to authenticate the %s session: %v", name, err)
	}

	return authenticated
}

func (f *fixture) create(t *testing.T, ctx context.Context, name string) api.GroupInfoDto {
	t.Helper()

	response, err := f.server.SyncPlayCreateGroup(ctx, api.SyncPlayCreateGroupRequestObject{
		JSONBody: &api.NewGroupRequestDto{GroupName: apiutil.Ptr(name)},
	})
	if err != nil {
		t.Fatalf("failed to create the group: %v", err)
	}

	created, ok := response.(api.SyncPlayCreateGroup200JSONResponse)
	if !ok {
		t.Fatalf("SyncPlayCreateGroup returned %T", response)
	}

	return api.GroupInfoDto(created)
}

func (f *fixture) group(t *testing.T, ctx context.Context, id uuid.UUID) api.GroupInfoDto {
	t.Helper()

	response, err := f.server.SyncPlayGetGroup(ctx, api.SyncPlayGetGroupRequestObject{Id: id})
	if err != nil {
		t.Fatalf("failed to read the group: %v", err)
	}

	found, ok := response.(api.SyncPlayGetGroup200JSONResponse)
	if !ok {
		t.Fatalf("SyncPlayGetGroup returned %T", response)
	}

	return api.GroupInfoDto(found)
}

func (f *fixture) join(t *testing.T, ctx context.Context, id uuid.UUID) api.SyncPlayJoinGroupResponseObject {
	t.Helper()

	response, err := f.server.SyncPlayJoinGroup(ctx, api.SyncPlayJoinGroupRequestObject{
		JSONBody: &api.JoinGroupRequestDto{GroupId: &id},
	})
	if err != nil {
		t.Fatalf("failed to join the group: %v", err)
	}
	if _, ok := response.(api.SyncPlayJoinGroup204Response); !ok {
		t.Fatalf("response = %T, want api.SyncPlayJoinGroup204Response", response)
	}

	return response
}

func (f *fixture) leave(t *testing.T, ctx context.Context) {
	t.Helper()

	response, err := f.server.SyncPlayLeaveGroup(ctx, api.SyncPlayLeaveGroupRequestObject{})
	if err != nil {
		t.Fatalf("failed to leave the group: %v", err)
	}
	if _, ok := response.(api.SyncPlayLeaveGroup204Response); !ok {
		t.Fatalf("response = %T, want api.SyncPlayLeaveGroup204Response", response)
	}
}

func (f *fixture) missing(t *testing.T, ctx context.Context, id uuid.UUID) {
	t.Helper()

	response, err := f.server.SyncPlayGetGroup(ctx, api.SyncPlayGetGroupRequestObject{Id: id})
	if err != nil {
		t.Fatalf("failed to read the group: %v", err)
	}
	if _, ok := response.(api.SyncPlayGetGroup404JSONResponse); !ok {
		t.Fatalf("response = %T, want api.SyncPlayGetGroup404JSONResponse", response)
	}
}

func addressed(t *testing.T, sent []published, want ...uuid.UUID) {
	t.Helper()

	if len(sent) != 1 {
		t.Fatalf("published %d updates, want 1", len(sent))
	}

	got := slices.Clone(sent[0].sessionIDs)
	slices.SortFunc(got, func(a, b uuid.UUID) int { return strings.Compare(a.String(), b.String()) })
	slices.SortFunc(want, func(a, b uuid.UUID) int { return strings.Compare(a.String(), b.String()) })

	if !slices.Equal(got, want) {
		t.Fatalf("addressed to %v, want %v", got, want)
	}
}

func TestServer_SyncPlayCreateGroup(t *testing.T) {
	t.Run("the creator is the only participant", func(t *testing.T) {
		f := newFixture(t)
		owner := f.signIn(t, "owner")

		created := f.create(t, owner, t.Name())

		if apiutil.Deref(created.GroupName) != t.Name() {
			t.Fatalf("group name = %q, want %q", apiutil.Deref(created.GroupName), t.Name())
		}
		if participants := apiutil.Deref(created.Participants); !slices.Equal(participants, []string{"owner"}) {
			t.Fatalf("participants = %v, want [owner]", participants)
		}
		if state := apiutil.Deref(created.State); state != api.GroupStateType("Idle") {
			t.Fatalf("state = %q, want Idle", state)
		}
	})

	t.Run("without a session it is unauthorized", func(t *testing.T) {
		f := newFixture(t)

		response, err := f.server.SyncPlayCreateGroup(context.Background(), api.SyncPlayCreateGroupRequestObject{})
		if err != nil {
			t.Fatalf("failed to create the group: %v", err)
		}
		if _, ok := response.(api.SyncPlayCreateGroup401Response); !ok {
			t.Fatalf("response = %T, want api.SyncPlayCreateGroup401Response", response)
		}
	})

	t.Run("creating one while in another tells the group left behind", func(t *testing.T) {
		f := newFixture(t)
		owner := f.signIn(t, "owner")
		guest := f.signIn(t, "guest")

		first := f.create(t, owner, t.Name())
		f.join(t, guest, *first.GroupId)

		f.create(t, guest, t.Name()+"-second")

		addressed(t, f.published.of(userLeft), auth.SessionFrom(owner).ID)
		addressed(t, f.published.of(groupLeft), auth.SessionFrom(guest).ID)
	})
}

func TestServer_SyncPlayJoinGroup(t *testing.T) {
	t.Run("the joiner becomes a participant", func(t *testing.T) {
		f := newFixture(t)
		owner := f.signIn(t, "owner")
		guest := f.signIn(t, "guest")

		created := f.create(t, owner, t.Name())
		f.join(t, guest, *created.GroupId)

		participants := apiutil.Deref(f.group(t, owner, *created.GroupId).Participants)
		slices.Sort(participants)
		if !slices.Equal(participants, []string{"guest", "owner"}) {
			t.Fatalf("participants = %v, want [guest owner]", participants)
		}
	})

	t.Run("an unknown group is refused to the joiner alone", func(t *testing.T) {
		f := newFixture(t)
		owner := f.signIn(t, "owner")
		missing := uuid.New()

		f.join(t, owner, missing)

		refusals := f.published.of(groupDoesNotExist)
		addressed(t, refusals, auth.SessionFrom(owner).ID)
		if refusals[0].update.Data != missing.String() {
			t.Fatalf("data = %v, want %v", refusals[0].update.Data, missing)
		}
	})

	t.Run("the joiner is told the group and the members are told the user", func(t *testing.T) {
		f := newFixture(t)
		owner := f.signIn(t, "owner")
		guest := f.signIn(t, "guest")

		created := f.create(t, owner, t.Name())
		f.join(t, guest, *created.GroupId)

		joined := f.published.of(groupJoined)
		if len(joined) != 2 {
			t.Fatalf("published %d GroupJoined updates, want 2", len(joined))
		}
		if !slices.Equal(joined[1].sessionIDs, []uuid.UUID{auth.SessionFrom(guest).ID}) {
			t.Fatalf("GroupJoined addressed to %v", joined[1].sessionIDs)
		}

		announced := f.published.of(userJoined)
		addressed(t, announced, auth.SessionFrom(owner).ID)
		if announced[0].update.Data != "guest" {
			t.Fatalf("UserJoined data = %v, want guest", announced[0].update.Data)
		}
	})

	t.Run("a repeated join does not announce the same user twice", func(t *testing.T) {
		f := newFixture(t)
		owner := f.signIn(t, "owner")
		guest := f.signIn(t, "guest")

		created := f.create(t, owner, t.Name())
		f.join(t, guest, *created.GroupId)
		f.join(t, guest, *created.GroupId)

		if announced := f.published.of(userJoined); len(announced) != 1 {
			t.Fatalf("published %d UserJoined updates for one user, want 1", len(announced))
		}

		participants := apiutil.Deref(f.group(t, owner, *created.GroupId).Participants)
		if len(participants) != 2 {
			t.Fatalf("participants = %v, want two", participants)
		}
	})

	t.Run("joining a second group leaves the first and tells it", func(t *testing.T) {
		f := newFixture(t)
		owner := f.signIn(t, "owner")
		guest := f.signIn(t, "guest")
		mover := f.signIn(t, "mover")

		first := f.create(t, owner, t.Name()+"-first")
		second := f.create(t, guest, t.Name()+"-second")
		f.join(t, mover, *first.GroupId)
		f.join(t, mover, *second.GroupId)

		addressed(t, f.published.of(userLeft), auth.SessionFrom(owner).ID)
		if left := f.published.of(groupLeft); left[0].update.Data != first.GroupId.String() {
			t.Fatalf("GroupLeft data = %v, want %v", left[0].update.Data, first.GroupId)
		}

		participants := apiutil.Deref(f.group(t, guest, *second.GroupId).Participants)
		slices.Sort(participants)
		if !slices.Equal(participants, []string{"guest", "mover"}) {
			t.Fatalf("participants = %v, want [guest mover]", participants)
		}
	})

	t.Run("a membership change advances the group's LastUpdatedAt", func(t *testing.T) {
		f := newFixture(t)
		owner := f.signIn(t, "owner")
		guest := f.signIn(t, "guest")

		created := f.create(t, owner, t.Name())
		before := apiutil.Deref(created.LastUpdatedAt)

		f.join(t, guest, *created.GroupId)

		after := apiutil.Deref(f.group(t, owner, *created.GroupId).LastUpdatedAt)
		if !after.After(before) {
			t.Fatalf("LastUpdatedAt = %v, want later than %v", after, before)
		}
	})
}

func TestServer_SyncPlayLeaveGroup(t *testing.T) {
	t.Run("the remaining members are told who left", func(t *testing.T) {
		f := newFixture(t)
		owner := f.signIn(t, "owner")
		guest := f.signIn(t, "guest")

		created := f.create(t, owner, t.Name())
		f.join(t, guest, *created.GroupId)
		f.leave(t, guest)

		addressed(t, f.published.of(groupLeft), auth.SessionFrom(guest).ID)

		announced := f.published.of(userLeft)
		addressed(t, announced, auth.SessionFrom(owner).ID)
		if announced[0].update.Data != "guest" {
			t.Fatalf("UserLeft data = %v, want guest", announced[0].update.Data)
		}
	})

	t.Run("the last member out makes the group unreachable", func(t *testing.T) {
		f := newFixture(t)
		owner := f.signIn(t, "owner")
		guest := f.signIn(t, "guest")

		created := f.create(t, owner, t.Name())
		f.join(t, guest, *created.GroupId)
		f.leave(t, owner)
		f.leave(t, guest)

		f.missing(t, owner, *created.GroupId)
	})

	t.Run("leaving without a group is accepted and tells nobody", func(t *testing.T) {
		f := newFixture(t)
		owner := f.signIn(t, "owner")

		f.leave(t, owner)

		if sent := f.published.of(groupLeft); len(sent) != 0 {
			t.Fatalf("published %d GroupLeft updates, want none", len(sent))
		}
	})
}

func TestServer_SyncPlayGetGroups(t *testing.T) {
	t.Run("a group with members is listed", func(t *testing.T) {
		f := newFixture(t)
		owner := f.signIn(t, "owner")

		created := f.create(t, owner, t.Name())

		response, err := f.server.SyncPlayGetGroups(owner, api.SyncPlayGetGroupsRequestObject{})
		if err != nil {
			t.Fatalf("failed to list the groups: %v", err)
		}

		listed, ok := response.(api.SyncPlayGetGroups200JSONResponse)
		if !ok {
			t.Fatalf("response = %T, want api.SyncPlayGetGroups200JSONResponse", response)
		}

		if !slices.ContainsFunc(listed, func(group api.GroupInfoDto) bool {
			return apiutil.Deref(group.GroupId) == apiutil.Deref(created.GroupId)
		}) {
			t.Fatalf("group %v missing from %d listed groups", apiutil.Deref(created.GroupId), len(listed))
		}
	})

	t.Run("a group whose sessions are all gone is not listed", func(t *testing.T) {
		f := newFixture(t)
		owner := f.signIn(t, "owner")

		created := f.create(t, owner, t.Name())

		if err := f.sessions.DeleteByToken(context.Background(), auth.SessionFrom(owner).AccessToken); err != nil {
			t.Fatalf("failed to delete the session: %v", err)
		}

		response, err := f.server.SyncPlayGetGroups(context.Background(), api.SyncPlayGetGroupsRequestObject{})
		if err != nil {
			t.Fatalf("failed to list the groups: %v", err)
		}

		listed := response.(api.SyncPlayGetGroups200JSONResponse)
		if slices.ContainsFunc(listed, func(group api.GroupInfoDto) bool {
			return apiutil.Deref(group.GroupId) == apiutil.Deref(created.GroupId)
		}) {
			t.Fatalf("group %v is listed with no members left", apiutil.Deref(created.GroupId))
		}
	})

	t.Run("a deleted session stops being a participant", func(t *testing.T) {
		f := newFixture(t)
		owner := f.signIn(t, "owner")
		guest := f.signIn(t, "guest")

		created := f.create(t, owner, t.Name())
		f.join(t, guest, *created.GroupId)

		if err := f.sessions.DeleteByToken(context.Background(), auth.SessionFrom(guest).AccessToken); err != nil {
			t.Fatalf("failed to delete the guest session: %v", err)
		}

		if participants := apiutil.Deref(f.group(t, owner, *created.GroupId).Participants); !slices.Equal(participants, []string{"owner"}) {
			t.Fatalf("participants = %v, want [owner]", participants)
		}
	})
}
