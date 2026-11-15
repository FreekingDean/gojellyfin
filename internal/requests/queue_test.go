package requests

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/discover"
	itemmodel "github.com/FreekingDean/gojellyfin/internal/store/item"
)

func quota(titles int) *int { return &titles }

func (f *fixture) allow(t *testing.T, userID uuid.UUID, allowance Allowance) {
	t.Helper()

	if err := f.service.Allow(context.Background(), userID, allowance); err != nil {
		t.Fatalf("failed to save the allowance: %v", err)
	}
}

func (f *fixture) asked(t *testing.T, key string) Asked {
	t.Helper()

	queue, err := f.service.Queue(context.Background())
	if err != nil {
		t.Fatalf("failed to read the queue: %v", err)
	}
	for _, asked := range queue {
		if asked.Key == key {
			return asked
		}
	}
	t.Fatalf("%s is not in the queue", key)

	return Asked{}
}

func TestService_Allowance(t *testing.T) {
	ctx := context.Background()

	t.Run("a user who may not ask is refused and nothing is recorded", func(t *testing.T) {
		fixture := newFixture(t)
		fixture.allow(t, fixture.users[0], Allowance{CanRequest: false})
		title := wanted(itemmodel.KindMovie, nil)

		if err := fixture.service.Request(ctx, fixture.users[0], title); !errors.Is(err, ErrNotAllowed) {
			t.Errorf("err = %v, want ErrNotAllowed", err)
		}
		if got := fixture.mark(t, fixture.users[0], title.Key); got.Mine || len(fixture.downloaders.submitted) != 0 {
			t.Errorf("mark = %+v, submitted = %v", got, fixture.downloaders.submitted)
		}
	})

	t.Run("the quota counts titles, and asking again for one is free", func(t *testing.T) {
		fixture := newFixture(t)
		fixture.allow(t, fixture.users[0], Allowance{CanRequest: true, Quota: quota(1)})
		first, second := wanted(itemmodel.KindMovie, nil), wanted(itemmodel.KindMovie, nil)

		if err := fixture.service.Request(ctx, fixture.users[0], first); err != nil {
			t.Fatalf("failed to request: %v", err)
		}
		if err := fixture.service.Request(ctx, fixture.users[0], first); err != nil {
			t.Errorf("asking again for the same title = %v", err)
		}
		if err := fixture.service.Request(ctx, fixture.users[0], second); !errors.Is(err, ErrQuota) {
			t.Errorf("err = %v, want ErrQuota", err)
		}
		if err := fixture.service.Request(ctx, fixture.users[1], second); err != nil {
			t.Errorf("another user's request = %v", err)
		}

		askers, err := fixture.service.Askers(ctx)
		if err != nil {
			t.Fatalf("failed to list the askers: %v", err)
		}
		for _, asker := range askers {
			if asker.ID == fixture.users[0] && (asker.Used != 1 || asker.Quota == nil || *asker.Quota != 1) {
				t.Errorf("asker = %+v", asker)
			}
		}
	})

	t.Run("a request that needs approval waits, and approving it asks the downloader", func(t *testing.T) {
		fixture := newFixture(t)
		fixture.allow(t, fixture.users[0], Allowance{CanRequest: true, NeedsApproval: true})
		title := wanted(itemmodel.KindMovie, nil)

		if err := fixture.service.Request(ctx, fixture.users[0], title); err != nil {
			t.Fatalf("failed to request: %v", err)
		}
		if got := fixture.mark(t, fixture.users[0], title.Key); got.Requested || !got.Pending || !got.Mine {
			t.Errorf("mark while waiting = %+v", got)
		}
		if len(fixture.downloaders.submitted) != 0 {
			t.Fatalf("submitted %v before approval", fixture.downloaders.submitted)
		}

		waiting := fixture.asked(t, title.Key)
		if len(waiting.Requests) != 1 || waiting.Requests[0].Status != StatusPending || waiting.Requests[0].User == nil {
			t.Fatalf("queue entry = %+v", waiting)
		}

		if err := fixture.service.Approve(ctx, waiting.Requests[0].ID); err != nil {
			t.Fatalf("failed to approve: %v", err)
		}
		if got := fixture.mark(t, fixture.users[0], title.Key); !got.Requested || got.Pending {
			t.Errorf("mark once approved = %+v", got)
		}
		approved := fixture.asked(t, title.Key)
		if len(approved.Submissions) != 1 || approved.Submissions[0].Source == nil || approved.Submissions[0].AcceptedAt == nil {
			t.Errorf("submissions = %+v", approved.Submissions)
		}
	})

	t.Run("a declined request marks nothing and asks nobody", func(t *testing.T) {
		fixture := newFixture(t)
		fixture.allow(t, fixture.users[0], Allowance{CanRequest: true, NeedsApproval: true})
		title := wanted(itemmodel.KindMovie, nil)

		if err := fixture.service.Request(ctx, fixture.users[0], title); err != nil {
			t.Fatalf("failed to request: %v", err)
		}
		if err := fixture.service.Decline(ctx, fixture.asked(t, title.Key).Requests[0].ID); err != nil {
			t.Fatalf("failed to decline: %v", err)
		}
		if got := fixture.mark(t, fixture.users[0], title.Key); got != (Mark{}) {
			t.Errorf("mark = %+v, want none", got)
		}
		if len(fixture.downloaders.submitted) != 0 {
			t.Errorf("submitted %v", fixture.downloaders.submitted)
		}
	})

	t.Run("a request nobody made cannot be decided", func(t *testing.T) {
		fixture := newFixture(t)

		if err := fixture.service.Approve(ctx, uuid.New()); !errors.Is(err, ErrNotFound) {
			t.Errorf("approve = %v, want ErrNotFound", err)
		}
		if err := fixture.service.Decline(ctx, uuid.New()); !errors.Is(err, ErrNotFound) {
			t.Errorf("decline = %v, want ErrNotFound", err)
		}
		if err := fixture.service.Allow(ctx, uuid.New(), Allowance{}); !errors.Is(err, ErrNotFound) {
			t.Errorf("allow = %v, want ErrNotFound", err)
		}
	})
}

func TestService_Mine(t *testing.T) {
	ctx := context.Background()
	fixture := newFixture(t)
	fixture.allow(t, fixture.users[0], Allowance{CanRequest: true})
	first, second, theirs := wanted(itemmodel.KindMovie, nil), wanted(itemmodel.KindMovie, nil), wanted(itemmodel.KindMovie, nil)

	for _, asking := range []struct {
		userID uuid.UUID
		title  discover.Title
	}{{fixture.users[0], first}, {fixture.users[0], second}, {fixture.users[1], theirs}} {
		if err := fixture.service.Request(ctx, asking.userID, asking.title); err != nil {
			t.Fatalf("failed to request %s: %v", asking.title.Key, err)
		}
	}

	mine, err := fixture.service.Mine(ctx, fixture.users[0])
	if err != nil {
		t.Fatalf("failed to list the user's requests: %v", err)
	}
	keys := []string{}
	for _, record := range mine {
		keys = append(keys, record.Key)
	}
	if len(keys) != 2 || keys[0] != second.Key || keys[1] != first.Key {
		t.Errorf("mine = %v, want %s then %s, newest first and only this user's", keys, second.Key, first.Key)
	}
}
