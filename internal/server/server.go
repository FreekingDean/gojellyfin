package server

import (
	"github.com/FreekingDean/gojellyfin/internal/server/activitylog"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/branding"
	"github.com/FreekingDean/gojellyfin/internal/server/configuration"
	"github.com/FreekingDean/gojellyfin/internal/server/displaypreferences"
	"github.com/FreekingDean/gojellyfin/internal/server/filter"
	"github.com/FreekingDean/gojellyfin/internal/server/genres"
	"github.com/FreekingDean/gojellyfin/internal/server/items"
	"github.com/FreekingDean/gojellyfin/internal/server/library"
	"github.com/FreekingDean/gojellyfin/internal/server/librarystructure"
	"github.com/FreekingDean/gojellyfin/internal/server/localization"
	"github.com/FreekingDean/gojellyfin/internal/server/mediainfo"
	"github.com/FreekingDean/gojellyfin/internal/server/musicgenres"
	"github.com/FreekingDean/gojellyfin/internal/server/persons"
	"github.com/FreekingDean/gojellyfin/internal/server/playlists"
	"github.com/FreekingDean/gojellyfin/internal/server/playstate"
	"github.com/FreekingDean/gojellyfin/internal/server/quickconnect"
	"github.com/FreekingDean/gojellyfin/internal/server/search"
	"github.com/FreekingDean/gojellyfin/internal/server/session"
	"github.com/FreekingDean/gojellyfin/internal/server/studios"
	"github.com/FreekingDean/gojellyfin/internal/server/syncplay"
	"github.com/FreekingDean/gojellyfin/internal/server/system"
	"github.com/FreekingDean/gojellyfin/internal/server/user"
	"github.com/FreekingDean/gojellyfin/internal/server/userlibrary"
	"github.com/FreekingDean/gojellyfin/internal/server/userviews"
	"github.com/FreekingDean/gojellyfin/internal/server/years"
)

// Tag services sit one level shallower than the fallback, so a registered
// service wins the selector and everything else falls through to Unimplemented.
type nestedUnimplemented struct {
	api.Unimplemented
}

// Embedded field names are type names, so every tag service comes in under an
// alias to keep them distinct.
type (
	FilterServer             = filter.Server
	YearsServer              = years.Server
	SearchServer             = search.Server
	StudiosServer            = studios.Server
	GenresServer             = genres.Server
	MusicGenresServer        = musicgenres.Server
	PersonsServer            = persons.Server
	ActivityLogServer        = activitylog.Server
	BrandingServer           = branding.Server
	ConfigurationServer      = configuration.Server
	DisplayPreferencesServer = displaypreferences.Server
	ItemsServer              = items.Server
	LibraryServer            = library.Server
	LibraryStructureServer   = librarystructure.Server
	LocalizationServer       = localization.Server
	MediaInfoServer          = mediainfo.Server
	PlaylistsServer          = playlists.Server
	PlaystateServer          = playstate.Server
	QuickConnectServer       = quickconnect.Server
	SessionServer            = session.Server
	SyncPlayServer           = syncplay.Server
	SystemServer             = system.Server
	UserServer               = user.Server
	UserLibraryServer        = userlibrary.Server
	UserViewsServer          = userviews.Server
)

type Server struct {
	*FilterServer
	*YearsServer
	*SearchServer
	*StudiosServer
	*GenresServer
	*MusicGenresServer
	*PersonsServer
	*ActivityLogServer
	*BrandingServer
	*ConfigurationServer
	*DisplayPreferencesServer
	*ItemsServer
	*LibraryServer
	*LibraryStructureServer
	*LocalizationServer
	*MediaInfoServer
	*PlaylistsServer
	*PlaystateServer
	*QuickConnectServer
	*SessionServer
	*SyncPlayServer
	*SystemServer
	*UserServer
	*UserLibraryServer
	*UserViewsServer

	nestedUnimplemented
}

func New(
	filter *filter.Server,
	years *years.Server,
	search *search.Server,
	studios *studios.Server,
	genres *genres.Server,
	musicgenres *musicgenres.Server,
	persons *persons.Server,
	activityLog *activitylog.Server,
	branding *branding.Server,
	configuration *configuration.Server,
	displayPreferences *displaypreferences.Server,
	items *items.Server,
	library *library.Server,
	libraryStructure *librarystructure.Server,
	localization *localization.Server,
	mediaInfo *mediainfo.Server,
	playlists *playlists.Server,
	playstate *playstate.Server,
	quickConnect *quickconnect.Server,
	session *session.Server,
	syncPlay *syncplay.Server,
	system *system.Server,
	user *user.Server,
	userLibrary *userlibrary.Server,
	userViews *userviews.Server,
) *Server {
	return &Server{
		FilterServer:             filter,
		YearsServer:              years,
		SearchServer:             search,
		StudiosServer:            studios,
		GenresServer:             genres,
		MusicGenresServer:        musicgenres,
		PersonsServer:            persons,
		ActivityLogServer:        activityLog,
		BrandingServer:           branding,
		ConfigurationServer:      configuration,
		DisplayPreferencesServer: displayPreferences,
		ItemsServer:              items,
		LibraryServer:            library,
		LibraryStructureServer:   libraryStructure,
		LocalizationServer:       localization,
		MediaInfoServer:          mediaInfo,
		PlaylistsServer:          playlists,
		PlaystateServer:          playstate,
		QuickConnectServer:       quickConnect,
		SessionServer:            session,
		SyncPlayServer:           syncPlay,
		SystemServer:             system,
		UserServer:               user,
		UserLibraryServer:        userLibrary,
		UserViewsServer:          userViews,
	}
}
