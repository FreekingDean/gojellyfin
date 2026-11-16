package sessions

import (
	"errors"
	"fmt"
	"time"

	"entgo.io/ent/dialect/sql"

	"github.com/FreekingDean/gojellyfin/internal/store/predicate"
	usermodel "github.com/FreekingDean/gojellyfin/internal/store/user"
	policymodel "github.com/FreekingDean/gojellyfin/internal/store/userpolicy"
)

var ErrOutsideSchedule = errors.New("the account's access schedule does not allow it now")

func admittedNow(at time.Time) predicate.User {
	return usermodel.Or(usermodel.Not(usermodel.HasPolicy()), usermodel.HasPolicyWith(scheduled(at)))
}

func scheduled(at time.Time) predicate.UserPolicy {
	local := at.In(time.Local)
	hour := float64(local.Hour()) + float64(local.Minute())/60 + float64(local.Second())/3600
	today, yesterday := days(local.Weekday()), days(local.AddDate(0, 0, -1).Weekday())

	return func(selector *sql.Selector) {
		schedules := selector.C(policymodel.FieldAccessSchedules)
		selector.Where(sql.Or(
			sql.EQ(selector.C(policymodel.FieldIsAdministrator), true),
			sql.IsNull(schedules),
			sql.ExprP(fmt.Sprintf("jsonb_array_length(%s) = 0", schedules)),
			sql.P(func(b *sql.Builder) {
				b.WriteString(fmt.Sprintf("EXISTS (SELECT 1 FROM jsonb_array_elements(%s) AS w(e) WHERE ", schedules))
				b.WriteString("((e->>'day_of_week') IN (").Args(today[0], today[1], today[2])
				b.WriteString(") AND (e->>'start_hour')::float8 <= ").Arg(hour)
				b.WriteString(" AND ((e->>'end_hour')::float8 < (e->>'start_hour')::float8 OR ").Arg(hour)
				b.WriteString(" < (e->>'end_hour')::float8)) OR ((e->>'day_of_week') IN (").Args(yesterday[0], yesterday[1], yesterday[2])
				b.WriteString(") AND (e->>'end_hour')::float8 < (e->>'start_hour')::float8 AND ").Arg(hour)
				b.WriteString(" < (e->>'end_hour')::float8))")
			}),
		))
	}
}

func days(day time.Weekday) [3]string {
	group := "Weekday"
	if day == time.Saturday || day == time.Sunday {
		group = "Weekend"
	}

	return [3]string{day.String(), "Everyday", group}
}
