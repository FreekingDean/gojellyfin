package server

import (
	"go.uber.org/fx"

	"github.com/FreekingDean/gojellyfin/internal/apikeys"
	"github.com/FreekingDean/gojellyfin/internal/auth"
	"github.com/FreekingDean/gojellyfin/internal/config"
	"github.com/FreekingDean/gojellyfin/internal/filesystem"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/libraries"
	"github.com/FreekingDean/gojellyfin/internal/scanner"
	"github.com/FreekingDean/gojellyfin/internal/server/activitylog"
	"github.com/FreekingDean/gojellyfin/internal/server/apikey"
	"github.com/FreekingDean/gojellyfin/internal/server/branding"
	"github.com/FreekingDean/gojellyfin/internal/server/channels"
	"github.com/FreekingDean/gojellyfin/internal/server/configuration"
	"github.com/FreekingDean/gojellyfin/internal/server/dashboard"
	"github.com/FreekingDean/gojellyfin/internal/server/devices"
	"github.com/FreekingDean/gojellyfin/internal/server/displaypreferences"
	"github.com/FreekingDean/gojellyfin/internal/server/environment"
	"github.com/FreekingDean/gojellyfin/internal/server/filter"
	"github.com/FreekingDean/gojellyfin/internal/server/genres"
	serveritems "github.com/FreekingDean/gojellyfin/internal/server/items"
	"github.com/FreekingDean/gojellyfin/internal/server/library"
	"github.com/FreekingDean/gojellyfin/internal/server/librarystructure"
	"github.com/FreekingDean/gojellyfin/internal/server/livetv"
	"github.com/FreekingDean/gojellyfin/internal/server/localization"
	"github.com/FreekingDean/gojellyfin/internal/server/mediainfo"
	"github.com/FreekingDean/gojellyfin/internal/server/musicgenres"
	"github.com/FreekingDean/gojellyfin/internal/server/persons"
	"github.com/FreekingDean/gojellyfin/internal/server/playlists"
	"github.com/FreekingDean/gojellyfin/internal/server/playstate"
	"github.com/FreekingDean/gojellyfin/internal/server/quickconnect"
	"github.com/FreekingDean/gojellyfin/internal/server/scheduledtasks"
	"github.com/FreekingDean/gojellyfin/internal/server/search"
	"github.com/FreekingDean/gojellyfin/internal/server/session"
	"github.com/FreekingDean/gojellyfin/internal/server/studios"
	"github.com/FreekingDean/gojellyfin/internal/server/syncplay"
	"github.com/FreekingDean/gojellyfin/internal/server/system"
	"github.com/FreekingDean/gojellyfin/internal/server/tvshows"
	"github.com/FreekingDean/gojellyfin/internal/server/user"
	"github.com/FreekingDean/gojellyfin/internal/server/userlibrary"
	"github.com/FreekingDean/gojellyfin/internal/server/userviews"
	"github.com/FreekingDean/gojellyfin/internal/server/years"
	"github.com/FreekingDean/gojellyfin/internal/sessions"
	"github.com/FreekingDean/gojellyfin/internal/users"
)

var Module = fx.Module(
	"server",
	fx.Provide(
		// domains
		apikeys.New,
		auth.New,
		sessions.New,
		users.New,
		items.New,
		libraries.New,
		config.New,
		filesystem.New,

		// one handler service per spec tag
		apikey.New,
		filter.New,
		years.New,
		search.New,
		studios.New,
		genres.New,
		musicgenres.New,
		persons.New,
		activitylog.New,
		environment.New,
		channels.New,
		dashboard.New,
		scheduledtasks.New,
		devices.New,
		tvshows.New,
		livetv.New,
		branding.New,
		configuration.New,
		displaypreferences.New,
		serveritems.New,
		library.New,
		librarystructure.New,
		localization.New,
		mediainfo.New,
		playlists.New,
		playstate.New,
		quickconnect.New,
		session.New,
		syncplay.New,
		system.New,
		user.New,
		userlibrary.New,
		userviews.New,

		New,
	),
	fx.Invoke(
		useScanner,
	),
)

func useScanner(library *library.Server, scanner *scanner.Scanner) {
	library.UseScanner(scanner)
}
