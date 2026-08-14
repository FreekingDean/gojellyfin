package user

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/auth"
	"github.com/FreekingDean/gojellyfin/internal/env"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
	"github.com/FreekingDean/gojellyfin/internal/sessions"
	"github.com/FreekingDean/gojellyfin/internal/store"
	"github.com/FreekingDean/gojellyfin/internal/users"
)

type accounts struct {
	server *Server
	admin  *users.User
	member *users.User
}

func newAccounts(t *testing.T) *accounts {
	t.Helper()

	connection, err := store.NewStore(env.MustLoad())
	if err != nil {
		t.Fatalf("failed to open the database: %v", err)
	}
	if err := connection.Start(); err != nil {
		t.Fatalf("failed to reach the database, set DATABASE_URL: %v", err)
	}
	t.Cleanup(func() {
		if err := connection.Stop(); err != nil {
			t.Errorf("failed to close the database: %v", err)
		}
	})

	service := users.New(connection.Client())
	ctx := context.Background()

	hash, err := auth.Hash("current-password")
	if err != nil {
		t.Fatalf("failed to hash: %v", err)
	}

	admin, err := service.CreateUser(ctx, "admin-"+uuid.NewString(), hash, true)
	if err != nil {
		t.Fatalf("failed to create the administrator: %v", err)
	}
	member, err := service.CreateUser(ctx, "member-"+uuid.NewString(), hash, false)
	if err != nil {
		t.Fatalf("failed to create the member: %v", err)
	}

	t.Cleanup(func() {
		for _, user := range []*users.User{admin, member} {
			if err := service.DeleteUser(ctx, user.ID); err != nil {
				t.Errorf("failed to delete %s: %v", user.Name, err)
			}
		}
	})

	return &accounts{server: New(service, nil), admin: admin, member: member}
}

func as(user *users.User) context.Context {
	session := &sessions.Session{}
	session.Edges.User = &store.User{ID: user.ID}

	return auth.ContextWithSession(context.Background(), session)
}

func changePassword(t *testing.T, server *Server, ctx context.Context, target uuid.UUID, reset bool) api.UpdateUserPasswordResponseObject {
	t.Helper()

	response, err := server.UpdateUserPassword(ctx, api.UpdateUserPasswordRequestObject{
		Params: api.UpdateUserPasswordParams{UserId: &target},
		JSONBody: &api.UpdateUserPasswordJSONRequestBody{
			CurrentPw:     apiutil.Ptr("current-password"),
			NewPw:         apiutil.Ptr("new-password"),
			ResetPassword: apiutil.Ptr(reset),
		},
	})
	if err != nil {
		t.Fatalf("UpdateUserPassword returned %v", err)
	}

	return response
}

// The takeover: name the administrator, ask for a reset, and the current
// password is never checked.
func TestAMemberCannotResetAnotherAccountsPassword(t *testing.T) {
	accounts := newAccounts(t)

	response := changePassword(t, accounts.server, as(accounts.member), accounts.admin.ID, true)
	if _, refused := response.(api.UpdateUserPassword403JSONResponse); !refused {
		t.Fatalf("response = %T, want 403: a member reset the administrator's password", response)
	}

	after, err := accounts.server.users.User(context.Background(), accounts.admin.ID)
	if err != nil {
		t.Fatalf("failed to reload the administrator: %v", err)
	}
	if matches, _ := auth.Verify("new-password", after.PasswordHash); matches {
		t.Error("the administrator's password was changed")
	}
}

func TestAMemberCannotSkipItsOwnCurrentPassword(t *testing.T) {
	accounts := newAccounts(t)

	response, err := accounts.server.UpdateUserPassword(as(accounts.member), api.UpdateUserPasswordRequestObject{
		Params: api.UpdateUserPasswordParams{UserId: &accounts.member.ID},
		JSONBody: &api.UpdateUserPasswordJSONRequestBody{
			CurrentPw:     apiutil.Ptr("not-the-password"),
			NewPw:         apiutil.Ptr("new-password"),
			ResetPassword: apiutil.Ptr(true),
		},
	})
	if err != nil {
		t.Fatalf("UpdateUserPassword returned %v", err)
	}
	if _, refused := response.(api.UpdateUserPassword403JSONResponse); !refused {
		t.Errorf("response = %T, want 403", response)
	}
}

func TestAMemberChangesItsOwnPassword(t *testing.T) {
	accounts := newAccounts(t)

	response := changePassword(t, accounts.server, as(accounts.member), accounts.member.ID, false)
	if _, ok := response.(api.UpdateUserPassword204Response); !ok {
		t.Fatalf("response = %T, want 204", response)
	}
}

func TestAnAdministratorResetsWithoutTheCurrentPassword(t *testing.T) {
	accounts := newAccounts(t)

	response := changePassword(t, accounts.server, as(accounts.admin), accounts.member.ID, true)
	if _, ok := response.(api.UpdateUserPassword204Response); !ok {
		t.Fatalf("response = %T, want 204", response)
	}
}

// The second door onto elevation: UpdateUser is DefaultAuthorization, so it
// must not carry a policy at all.
func TestUpdateUserIgnoresPolicy(t *testing.T) {
	accounts := newAccounts(t)
	ctx := as(accounts.member)

	_, err := accounts.server.UpdateUser(ctx, api.UpdateUserRequestObject{
		Params: api.UpdateUserParams{UserId: &accounts.member.ID},
		JSONBody: &api.UpdateUserJSONRequestBody{
			Policy: &api.UserPolicy{IsAdministrator: apiutil.Ptr(true)},
		},
	})
	if err != nil {
		t.Fatalf("UpdateUser returned %v", err)
	}

	administrator, err := accounts.server.users.IsAdministrator(context.Background(), accounts.member.ID)
	if err != nil {
		t.Fatalf("failed to read the policy: %v", err)
	}
	if administrator {
		t.Error("a member made itself an administrator through UpdateUser")
	}
}

func TestPublicUsersCarryNoPolicy(t *testing.T) {
	accounts := newAccounts(t)

	response, err := accounts.server.GetPublicUsers(context.Background(), api.GetPublicUsersRequestObject{})
	if err != nil {
		t.Fatalf("GetPublicUsers returned %v", err)
	}

	listed, ok := response.(api.GetPublicUsers200JSONResponse)
	if !ok {
		t.Fatalf("response = %T, want 200", response)
	}
	if len(listed) == 0 {
		t.Fatal("no users were listed")
	}

	for _, dto := range listed {
		if dto.Policy != nil {
			t.Errorf("%v carries a policy, which names the administrator to an anonymous caller", dto.Name)
		}
		if dto.Configuration != nil {
			t.Errorf("%v carries a configuration", dto.Name)
		}
	}
}
