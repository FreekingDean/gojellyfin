package sessions

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/FreekingDean/gojellyfin/internal/store"
	"github.com/FreekingDean/gojellyfin/internal/store/entities"
)

func at(day, hour, minute int) time.Time {
	return time.Date(2026, time.November, day, hour, minute, 0, 0, time.Local)
}

func TestService_ByToken_AccessSchedule(t *testing.T) {
	const (
		monday    = 9
		tuesday   = 10
		wednesday = 11
		friday    = 13
		saturday  = 14
		sunday    = 15
	)

	saturdayEvening := []entities.AccessSchedule{{DayOfWeek: "Saturday", StartHour: 16, EndHour: 20}}
	lateNights := []entities.AccessSchedule{{DayOfWeek: "Everyday", StartHour: 22, EndHour: 2}}

	tests := []serviceByTokenAccessScheduleCase{
		{"no schedule admits any time", nil, false, at(monday, 3, 0), true},
		{"an empty schedule admits any time", []entities.AccessSchedule{}, false, at(monday, 3, 0), true},
		{"inside a window", saturdayEvening, false, at(saturday, 17, 30), true},
		{"at the start of a window", saturdayEvening, false, at(saturday, 16, 0), true},
		{"at the end of a window", saturdayEvening, false, at(saturday, 20, 0), false},
		{"after a window", saturdayEvening, false, at(saturday, 21, 0), false},
		{"the right hours on the wrong day", saturdayEvening, false, at(sunday, 17, 0), false},
		{"Weekday on a weekday", []entities.AccessSchedule{{DayOfWeek: "Weekday", StartHour: 9, EndHour: 17}}, false, at(monday, 10, 0), true},
		{"Weekday on a weekend", []entities.AccessSchedule{{DayOfWeek: "Weekday", StartHour: 9, EndHour: 17}}, false, at(saturday, 10, 0), false},
		{"Weekend on a weekend", []entities.AccessSchedule{{DayOfWeek: "Weekend", StartHour: 9, EndHour: 17}}, false, at(sunday, 10, 0), true},
		{"a fractional hour", []entities.AccessSchedule{{DayOfWeek: "Everyday", StartHour: 9.5, EndHour: 10}}, false, at(monday, 9, 15), false},
		{"before midnight in a window that crosses it", lateNights, false, at(tuesday, 23, 0), true},
		{"after midnight in a window that crosses it", lateNights, false, at(wednesday, 1, 0), true},
		{"after a window that crossed midnight", lateNights, false, at(wednesday, 3, 0), false},
		{"before a window that crosses midnight", lateNights, false, at(tuesday, 21, 0), false},
		{"the morning after the night the window opened", []entities.AccessSchedule{{DayOfWeek: "Friday", StartHour: 22, EndHour: 2}}, false, at(saturday, 1, 0), true},
		{"the morning of the night the window opens", []entities.AccessSchedule{{DayOfWeek: "Friday", StartHour: 22, EndHour: 2}}, false, at(friday, 1, 0), false},
		{"any one window is enough", append([]entities.AccessSchedule{{DayOfWeek: "Monday", StartHour: 1, EndHour: 2}}, saturdayEvening...), false, at(saturday, 17, 0), true},
		{"an administrator is never scheduled out", saturdayEvening, true, at(monday, 3, 0), true},
	}

	for _, test := range tests {
		t.Run(test.name, testService_ByToken_AccessScheduleCase(test))
	}
}

func testService_ByToken_AccessScheduleCase(test serviceByTokenAccessScheduleCase) func(*testing.T) {
	return func(t *testing.T) {
		fixture := newFixture(t)
		ctx := context.Background()

		policy := fixture.client.UserPolicy.Create().SetUserID(fixture.user.ID).SetIsAdministrator(test.administrator)
		if test.schedules != nil {
			policy.SetAccessSchedules(test.schedules)
		}
		if err := policy.Exec(ctx); err != nil {
			t.Fatalf("failed to create the policy: %v", err)
		}

		token := uuid.NewString()
		if err := fixture.service.insert(ctx, fixture.user.ID, token, fixture.device, nil); err != nil {
			t.Fatalf("failed to create the session: %v", err)
		}

		fixture.service.now = func() time.Time { return test.at }
		_, err := fixture.service.ByToken(ctx, token)
		if test.admitted && err != nil {
			t.Errorf("err = %v, want the session admitted", err)
		}
		if !test.admitted && !store.IsNotFound(err) {
			t.Errorf("err = %v, want not found outside the schedule", err)
		}

		_, err = fixture.service.Create(ctx, fixture.user.ID, uuid.NewString(), fixture.device)
		if test.admitted && err != nil {
			t.Errorf("sign in err = %v, want it admitted", err)
		}
		if !test.admitted && !errors.Is(err, ErrOutsideSchedule) {
			t.Errorf("sign in err = %v, want ErrOutsideSchedule", err)
		}
	}
}

type serviceByTokenAccessScheduleCase struct {
	name          string
	schedules     []entities.AccessSchedule
	administrator bool
	at            time.Time
	admitted      bool
}
