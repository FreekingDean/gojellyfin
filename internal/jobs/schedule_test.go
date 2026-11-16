package jobs

import (
	"errors"
	"testing"
	"time"

	"go.temporal.io/sdk/client"
)

func TestTrigger_Validate(t *testing.T) {
	for name, trigger := range map[string]Trigger{
		"an interval of nothing": {Kind: TriggerInterval},
		"a time past midnight":   {Kind: TriggerDaily, TimeOfDay: 25 * time.Hour},
		"an eighth day":          {Kind: TriggerWeekly, Day: 7},
		"a kind nobody defined":  {Kind: "Hourly"},
		"a negative time limit":  {Kind: TriggerStartup, MaxRuntime: -time.Minute},
	} {
		t.Run(name, func(t *testing.T) {
			if err := trigger.validate(); !errors.Is(err, ErrInvalidTrigger) {
				t.Errorf("validate = %v, want ErrInvalidTrigger", err)
			}
		})
	}
}

func TestSpec(t *testing.T) {
	got := spec([]Trigger{
		{Kind: TriggerInterval, Interval: 6 * time.Hour},
		{Kind: TriggerDaily, TimeOfDay: 3*time.Hour + 15*time.Minute + 30*time.Second},
		{Kind: TriggerWeekly, TimeOfDay: 22 * time.Hour, Day: time.Friday},
		{Kind: TriggerStartup},
	})

	if len(got.Intervals) != 1 || got.Intervals[0].Every != 6*time.Hour {
		t.Errorf("intervals = %+v, want one every six hours", got.Intervals)
	}
	if len(got.Calendars) != 2 {
		t.Fatalf("calendars = %+v, want the daily and the weekly", got.Calendars)
	}

	daily := got.Calendars[0]
	if at(daily.Hour) != 3 || at(daily.Minute) != 15 || at(daily.Second) != 30 || daily.DayOfWeek != nil {
		t.Errorf("daily = %+v, want 03:15:30 every day", daily)
	}

	weekly := got.Calendars[1]
	if at(weekly.Hour) != 22 || at(weekly.DayOfWeek) != int(time.Friday) {
		t.Errorf("weekly = %+v, want 22:00 on Fridays", weekly)
	}
}

func at(ranges []client.ScheduleRange) int {
	if len(ranges) != 1 {
		return -1
	}

	return ranges[0].Start
}

func TestRunLimit(t *testing.T) {
	if got := runLimit(nil); got != runTimeoutMax {
		t.Errorf("no limit = %s, want %s", got, runTimeoutMax)
	}

	got := runLimit([]Trigger{
		{Kind: TriggerDaily, MaxRuntime: 2 * time.Hour},
		{Kind: TriggerInterval, Interval: time.Hour, MaxRuntime: 30 * time.Minute},
		{Kind: TriggerStartup, MaxRuntime: time.Minute},
	})
	if got != 30*time.Minute {
		t.Errorf("limit = %s, want the shortest a timed trigger names", got)
	}
}
