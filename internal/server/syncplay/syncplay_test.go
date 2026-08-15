package syncplay

import (
	"context"
	"fmt"
	"slices"
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

// Stands in for notify so the tests assert what each member is told, which is
// the half of the fan-out that does not need a second pod to observe.
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

func TestCreateGroupNamesTheCreator(t *testing.T) {
	f := newFixture(t)
	owner := f.signIn(t, "owner")

	created := f.create(t, owner, t.Name())

	if apiutil.Deref(created.GroupName) != t.Name() {
		t.Fatalf("group name is %q", apiutil.Deref(created.GroupName))
	}
	if participants := apiutil.Deref(created.Participants); !slices.Equal(participants, []string{"owner"}) {
		t.Fatalf("participants are %v", participants)
	}
	if state := apiutil.Deref(created.State); state != api.GroupStateType("Idle") {
		t.Fatalf("state is %q", state)
	}
}

func TestCreateGroupWithoutASessionIsUnauthorized(t *testing.T) {
	f := newFixture(t)

	response, err := f.server.SyncPlayCreateGroup(context.Background(), api.SyncPlayCreateGroupRequestObject{})
	if err != nil {
		t.Fatalf("failed to create the group: %v", err)
	}
	if _, ok := response.(api.SyncPlayCreateGroup401Response); !ok {
		t.Fatalf("SyncPlayCreateGroup returned %T", response)
	}
}

func TestJoinGroupAddsAParticipant(t *testing.T) {
	f := newFixture(t)
	owner := f.signIn(t, "owner")
	guest := f.signIn(t, "guest")

	created := f.create(t, owner, t.Name())

	response, err := f.server.SyncPlayJoinGroup(guest, api.SyncPlayJoinGroupRequestObject{
		JSONBody: &api.JoinGroupRequestDto{GroupId: created.GroupId},
	})
	if err != nil {
		t.Fatalf("failed to join the group: %v", err)
	}
	if _, ok := response.(api.SyncPlayJoinGroup204Response); !ok {
		t.Fatalf("SyncPlayJoinGroup returned %T", response)
	}

	participants := apiutil.Deref(f.group(t, owner, *created.GroupId).Participants)
	slices.Sort(participants)
	if !slices.Equal(participants, []string{"guest", "owner"}) {
		t.Fatalf("participants are %v", participants)
	}
}

func TestJoinAnUnknownGroupTellsOnlyTheJoiner(t *testing.T) {
	f := newFixture(t)
	owner := f.signIn(t, "owner")
	missing := uuid.New()

	response, err := f.server.SyncPlayJoinGroup(owner, api.SyncPlayJoinGroupRequestObject{
		JSONBody: &api.JoinGroupRequestDto{GroupId: &missing},
	})
	if err != nil {
		t.Fatalf("failed to join the group: %v", err)
	}
	if _, ok := response.(api.SyncPlayJoinGroup204Response); !ok {
		t.Fatalf("SyncPlayJoinGroup returned %T", response)
	}

	refusals := f.published.of(groupDoesNotExist)
	if len(refusals) != 1 {
		t.Fatalf("published %d GroupDoesNotExist updates", len(refusals))
	}
	if !slices.Equal(refusals[0].sessionIDs, []uuid.UUID{auth.SessionFrom(owner).ID}) {
		t.Fatalf("GroupDoesNotExist addressed to %v", refusals[0].sessionIDs)
	}
	if refusals[0].update.Data != missing.String() {
		t.Fatalf("GroupDoesNotExist carried %v", refusals[0].update.Data)
	}
}

func TestJoinTellsTheJoinerAndTheMembersApart(t *testing.T) {
	f := newFixture(t)
	owner := f.signIn(t, "owner")
	guest := f.signIn(t, "guest")

	created := f.create(t, owner, t.Name())
	if _, err := f.server.SyncPlayJoinGroup(guest, api.SyncPlayJoinGroupRequestObject{
		JSONBody: &api.JoinGroupRequestDto{GroupId: created.GroupId},
	}); err != nil {
		t.Fatalf("failed to join the group: %v", err)
	}

	joined := f.published.of(groupJoined)
	if len(joined) != 2 {
		t.Fatalf("published %d GroupJoined updates", len(joined))
	}
	if !slices.Equal(joined[1].sessionIDs, []uuid.UUID{auth.SessionFrom(guest).ID}) {
		t.Fatalf("GroupJoined addressed to %v", joined[1].sessionIDs)
	}

	announced := f.published.of(userJoined)
	if len(announced) != 1 {
		t.Fatalf("published %d UserJoined updates", len(announced))
	}
	if !slices.Equal(announced[0].sessionIDs, []uuid.UUID{auth.SessionFrom(owner).ID}) {
		t.Fatalf("UserJoined addressed to %v", announced[0].sessionIDs)
	}
	if announced[0].update.Data != "guest" {
		t.Fatalf("UserJoined named %v", announced[0].update.Data)
	}
}

func TestLeaveTellsTheRemainingMembers(t *testing.T) {
	f := newFixture(t)
	owner := f.signIn(t, "owner")
	guest := f.signIn(t, "guest")

	created := f.create(t, owner, t.Name())
	if _, err := f.server.SyncPlayJoinGroup(guest, api.SyncPlayJoinGroupRequestObject{
		JSONBody: &api.JoinGroupRequestDto{GroupId: created.GroupId},
	}); err != nil {
		t.Fatalf("failed to join the group: %v", err)
	}

	if _, err := f.server.SyncPlayLeaveGroup(guest, api.SyncPlayLeaveGroupRequestObject{}); err != nil {
		t.Fatalf("failed to leave the group: %v", err)
	}

	left := f.published.of(groupLeft)
	if len(left) != 1 || !slices.Equal(left[0].sessionIDs, []uuid.UUID{auth.SessionFrom(guest).ID}) {
		t.Fatalf("GroupLeft published as %+v", left)
	}

	announced := f.published.of(userLeft)
	if len(announced) != 1 {
		t.Fatalf("published %d UserLeft updates", len(announced))
	}
	if !slices.Equal(announced[0].sessionIDs, []uuid.UUID{auth.SessionFrom(owner).ID}) {
		t.Fatalf("UserLeft addressed to %v", announced[0].sessionIDs)
	}
	if announced[0].update.Data != "guest" {
		t.Fatalf("UserLeft named %v", announced[0].update.Data)
	}
}

func TestJoiningASecondGroupLeavesTheFirst(t *testing.T) {
	f := newFixture(t)
	owner := f.signIn(t, "owner")
	guest := f.signIn(t, "guest")

	first := f.create(t, owner, t.Name()+"-first")
	second := f.create(t, guest, t.Name()+"-second")

	if _, err := f.server.SyncPlayJoinGroup(owner, api.SyncPlayJoinGroupRequestObject{
		JSONBody: &api.JoinGroupRequestDto{GroupId: second.GroupId},
	}); err != nil {
		t.Fatalf("failed to join the second group: %v", err)
	}

	participants := apiutil.Deref(f.group(t, owner, *second.GroupId).Participants)
	slices.Sort(participants)
	if !slices.Equal(participants, []string{"guest", "owner"}) {
		t.Fatalf("participants are %v", participants)
	}

	response, err := f.server.SyncPlayGetGroup(owner, api.SyncPlayGetGroupRequestObject{Id: *first.GroupId})
	if err != nil {
		t.Fatalf("failed to read the first group: %v", err)
	}
	if _, ok := response.(api.SyncPlayGetGroup404JSONResponse); !ok {
		t.Fatalf("SyncPlayGetGroup returned %T", response)
	}
}

func TestTheLastMemberOutDisbandsTheGroup(t *testing.T) {
	f := newFixture(t)
	owner := f.signIn(t, "owner")
	guest := f.signIn(t, "guest")

	created := f.create(t, owner, t.Name())
	if _, err := f.server.SyncPlayJoinGroup(guest, api.SyncPlayJoinGroupRequestObject{
		JSONBody: &api.JoinGroupRequestDto{GroupId: created.GroupId},
	}); err != nil {
		t.Fatalf("failed to join the group: %v", err)
	}

	for _, member := range []context.Context{owner, guest} {
		response, err := f.server.SyncPlayLeaveGroup(member, api.SyncPlayLeaveGroupRequestObject{})
		if err != nil {
			t.Fatalf("failed to leave the group: %v", err)
		}
		if _, ok := response.(api.SyncPlayLeaveGroup204Response); !ok {
			t.Fatalf("SyncPlayLeaveGroup returned %T", response)
		}
	}

	response, err := f.server.SyncPlayGetGroup(owner, api.SyncPlayGetGroupRequestObject{Id: *created.GroupId})
	if err != nil {
		t.Fatalf("failed to read the group: %v", err)
	}
	if _, ok := response.(api.SyncPlayGetGroup404JSONResponse); !ok {
		t.Fatalf("SyncPlayGetGroup returned %T", response)
	}
}

func TestGetGroupsListsTheGroup(t *testing.T) {
	f := newFixture(t)
	owner := f.signIn(t, "owner")

	created := f.create(t, owner, t.Name())

	response, err := f.server.SyncPlayGetGroups(owner, api.SyncPlayGetGroupsRequestObject{})
	if err != nil {
		t.Fatalf("failed to list the groups: %v", err)
	}

	listed, ok := response.(api.SyncPlayGetGroups200JSONResponse)
	if !ok {
		t.Fatalf("SyncPlayGetGroups returned %T", response)
	}

	if !slices.ContainsFunc(listed, func(group api.GroupInfoDto) bool {
		return apiutil.Deref(group.GroupId) == apiutil.Deref(created.GroupId)
	}) {
		t.Fatalf("group %s is missing from %d listed groups", apiutil.Deref(created.GroupId), len(listed))
	}
}

func TestDeletingASessionDropsItsMembership(t *testing.T) {
	f := newFixture(t)
	owner := f.signIn(t, "owner")
	guest := f.signIn(t, "guest")

	created := f.create(t, owner, t.Name())
	if _, err := f.server.SyncPlayJoinGroup(guest, api.SyncPlayJoinGroupRequestObject{
		JSONBody: &api.JoinGroupRequestDto{GroupId: created.GroupId},
	}); err != nil {
		t.Fatalf("failed to join the group: %v", err)
	}

	if err := f.sessions.DeleteByToken(context.Background(), auth.SessionFrom(guest).AccessToken); err != nil {
		t.Fatalf("failed to delete the guest session: %v", err)
	}

	if participants := apiutil.Deref(f.group(t, owner, *created.GroupId).Participants); !slices.Equal(participants, []string{"owner"}) {
		t.Fatalf("participants are %v", participants)
	}
}
