package migration

import (
	"fmt"
	"sort"
	"strings"
)

type UserState string

const (
	UserCreated     UserState = "created"
	UserAlreadyHere UserState = "already here"
	UserNameTaken   UserState = "name taken"
)

type UserOutcome struct {
	Username    string
	State       UserState
	SignInUnset bool
	Restricted  bool
}

type Unmatched struct {
	Kind  TitleKind
	Name  string
	Path  string
	Users int
}

type Report struct {
	DryRun   bool
	ServerID string
	Users    []UserOutcome

	SessionsCarried     int
	SessionsAlreadyHere int
	SessionsSkipped     int

	IntegrationsCarried     int
	IntegrationsAlreadyHere int

	WatchedCarried     int
	WatchedMerged      int
	WatchedAlreadyHere int
	WatchedSkipped     int
	Orphaned           int

	missing map[*Title]int
}

func (r *Report) unmatched(title *Title) {
	if r.missing == nil {
		r.missing = map[*Title]int{}
	}
	r.missing[title]++
}

func (r *Report) Unmatched() []Unmatched {
	found := make([]Unmatched, 0, len(r.missing))
	for title, users := range r.missing {
		found = append(found, Unmatched{Kind: title.Kind, Name: describe(title), Path: title.Path, Users: users})
	}
	sort.Slice(found, func(a, b int) bool {
		if found[a].Kind != found[b].Kind {
			return found[a].Kind < found[b].Kind
		}
		if found[a].Name != found[b].Name {
			return found[a].Name < found[b].Name
		}

		return found[a].Path < found[b].Path
	})

	return found
}

func describe(title *Title) string {
	if title.Series == nil || title.Kind == KindMovie || title.Kind == KindSeries {
		return title.Name
	}
	if title.Kind == KindSeason && title.Index != nil {
		return fmt.Sprintf("%s, season %d", title.Series.Name, *title.Index)
	}
	if title.ParentIndex != nil && title.Index != nil {
		return fmt.Sprintf("%s S%02dE%02d %s", title.Series.Name, *title.ParentIndex, *title.Index, title.Name)
	}

	return title.Series.Name + " " + title.Name
}

func (r *Report) String() string {
	out := &strings.Builder{}
	verb, tense := "carried", "left"
	if r.DryRun {
		verb, tense = "would carry", "would leave"
		fmt.Fprintln(out, "Dry run: nothing was written.")
		fmt.Fprintln(out)
	}

	fmt.Fprintln(out, "Users")
	for _, user := range r.Users {
		fmt.Fprintf(out, "  %-24s %s\n", user.Username, userLine(user))
	}

	fmt.Fprintln(out)
	fmt.Fprintf(out, "Sessions     %s %d, %d already here, %s %d behind\n", verb, r.SessionsCarried, r.SessionsAlreadyHere, tense, r.SessionsSkipped)
	fmt.Fprintf(out, "API keys     %s %d, %d already here\n", verb, r.IntegrationsCarried, r.IntegrationsAlreadyHere)
	fmt.Fprintf(out, "Watch state  %s %d, merged %d, %d already here, %s %d behind with their users\n",
		verb, r.WatchedCarried, r.WatchedMerged, r.WatchedAlreadyHere, tense, r.WatchedSkipped)
	if r.Orphaned > 0 {
		fmt.Fprintf(out, "             %d rows belong to things gojellyfin does not hold: collections, folders, music, or titles Jellyfin has since removed\n", r.Orphaned)
	}

	if missing := r.Unmatched(); len(missing) > 0 {
		fmt.Fprintln(out)
		fmt.Fprintf(out, "No title here matches these %d, so their watch state stayed behind. Run again once a library scan has them:\n", len(missing))
		for _, title := range missing {
			fmt.Fprintf(out, "  %-8s %s (%d users) %s\n", title.Kind, title.Name, title.Users, title.Path)
		}
	}

	fmt.Fprintln(out)
	if r.ServerID == "" {
		fmt.Fprintln(out, "No device.txt beside the database, so the server id is unknown. Without SERVER_ID set to it every client signs in again.")

		return out.String()
	}
	fmt.Fprintf(out, "Start gojellyfin with SERVER_ID=%s (serverId in the chart), or every client signs in again.\n", r.ServerID)

	return out.String()
}

func userLine(user UserOutcome) string {
	line := map[UserState]string{
		UserCreated:     "created with their Jellyfin id",
		UserAlreadyHere: "already here from an earlier run",
		UserNameTaken:   "SKIPPED: another account here holds the name. Rename or delete it and run again",
	}[user.State]

	if user.SignInUnset {
		line += "; no password in Jellyfin, so none here until `gojellyfin resetpassword`"
	}
	if user.Restricted && user.State != UserNameTaken {
		line += "; limited to chosen libraries in Jellyfin, so sees none here until theirs are ticked in the dashboard"
	}

	return line
}
