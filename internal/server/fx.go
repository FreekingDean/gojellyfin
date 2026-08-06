package server

import (
	"go.uber.org/fx"

	"github.com/FreekingDean/gojellyfin/internal/auth"
	"github.com/FreekingDean/gojellyfin/internal/config"
	"github.com/FreekingDean/gojellyfin/internal/http/middleware"
	"github.com/FreekingDean/gojellyfin/internal/items"
	"github.com/FreekingDean/gojellyfin/internal/libraries"
	"github.com/FreekingDean/gojellyfin/internal/scanner"
	"github.com/FreekingDean/gojellyfin/internal/server/activitylog"
	"github.com/FreekingDean/gojellyfin/internal/server/branding"
	"github.com/FreekingDean/gojellyfin/internal/server/configuration"
	"github.com/FreekingDean/gojellyfin/internal/server/displaypreferences"
	serveritems "github.com/FreekingDean/gojellyfin/internal/server/items"
	"github.com/FreekingDean/gojellyfin/internal/server/library"
	"github.com/FreekingDean/gojellyfin/internal/server/librarystructure"
	"github.com/FreekingDean/gojellyfin/internal/server/localization"
	"github.com/FreekingDean/gojellyfin/internal/server/mediainfo"
	"github.com/FreekingDean/gojellyfin/internal/server/playlists"
	"github.com/FreekingDean/gojellyfin/internal/server/playstate"
	"github.com/FreekingDean/gojellyfin/internal/server/quickconnect"
	"github.com/FreekingDean/gojellyfin/internal/server/session"
	"github.com/FreekingDean/gojellyfin/internal/server/syncplay"
	"github.com/FreekingDean/gojellyfin/internal/server/system"
	"github.com/FreekingDean/gojellyfin/internal/server/user"
	"github.com/FreekingDean/gojellyfin/internal/server/userlibrary"
	"github.com/FreekingDean/gojellyfin/internal/server/userviews"
	"github.com/FreekingDean/gojellyfin/internal/users"
)

var Module = fx.Module(
	"server",
	fx.Provide(
		// domains
		auth.New,
		users.New,
		items.New,
		libraries.New,
		config.New,

		// one handler service per spec tag
		activitylog.New,
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

		sessions,
		New,
	),
	fx.Invoke(
		useScanner,
	),
)

func sessions(session *session.Server) middleware.Sessions {
	return session
}

func useScanner(library *library.Server, scanner *scanner.Scanner) {
	library.UseScanner(scanner)
}
