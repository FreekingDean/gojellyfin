package server

import (
	"github.com/FreekingDean/gojellyfin/internal/server/activitylog"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
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
	"github.com/FreekingDean/gojellyfin/internal/server/image"
	"github.com/FreekingDean/gojellyfin/internal/server/items"
	"github.com/FreekingDean/gojellyfin/internal/server/itemupdate"
	"github.com/FreekingDean/gojellyfin/internal/server/library"
	"github.com/FreekingDean/gojellyfin/internal/server/librarystructure"
	"github.com/FreekingDean/gojellyfin/internal/server/livetv"
	"github.com/FreekingDean/gojellyfin/internal/server/localization"
	"github.com/FreekingDean/gojellyfin/internal/server/mediainfo"
	"github.com/FreekingDean/gojellyfin/internal/server/packages"
	"github.com/FreekingDean/gojellyfin/internal/server/persons"
	"github.com/FreekingDean/gojellyfin/internal/server/playlists"
	"github.com/FreekingDean/gojellyfin/internal/server/playstate"
	"github.com/FreekingDean/gojellyfin/internal/server/plugins"
	"github.com/FreekingDean/gojellyfin/internal/server/quickconnect"
	"github.com/FreekingDean/gojellyfin/internal/server/scheduledtasks"
	"github.com/FreekingDean/gojellyfin/internal/server/search"
	"github.com/FreekingDean/gojellyfin/internal/server/session"
	"github.com/FreekingDean/gojellyfin/internal/server/studios"
	"github.com/FreekingDean/gojellyfin/internal/server/subtitle"
	"github.com/FreekingDean/gojellyfin/internal/server/suggestions"
	"github.com/FreekingDean/gojellyfin/internal/server/syncplay"
	"github.com/FreekingDean/gojellyfin/internal/server/system"
	"github.com/FreekingDean/gojellyfin/internal/server/timesync"
	"github.com/FreekingDean/gojellyfin/internal/server/trailers"
	"github.com/FreekingDean/gojellyfin/internal/server/tvshows"
	"github.com/FreekingDean/gojellyfin/internal/server/user"
	"github.com/FreekingDean/gojellyfin/internal/server/userlibrary"
	"github.com/FreekingDean/gojellyfin/internal/server/userviews"
	"github.com/FreekingDean/gojellyfin/internal/server/years"
)

type nestedUnimplemented struct {
	api.Unimplemented
}

type (
	ApiKeyServer             = apikey.Server
	FilterServer             = filter.Server
	YearsServer              = years.Server
	SearchServer             = search.Server
	StudiosServer            = studios.Server
	GenresServer             = genres.Server
	ImageServer              = image.Server
	PersonsServer            = persons.Server
	LiveTvServer             = livetv.Server
	TvShowsServer            = tvshows.Server
	DevicesServer            = devices.Server
	ScheduledTasksServer     = scheduledtasks.Server
	DashboardServer          = dashboard.Server
	ChannelsServer           = channels.Server
	EnvironmentServer        = environment.Server
	ActivityLogServer        = activitylog.Server
	BrandingServer           = branding.Server
	ConfigurationServer      = configuration.Server
	DisplayPreferencesServer = displaypreferences.Server
	ItemsServer              = items.Server
	ItemUpdateServer         = itemupdate.Server
	LibraryServer            = library.Server
	LibraryStructureServer   = librarystructure.Server
	LocalizationServer       = localization.Server
	MediaInfoServer          = mediainfo.Server
	PlaylistsServer          = playlists.Server
	PlaystateServer          = playstate.Server
	PluginsServer            = plugins.Server
	PackagesServer           = packages.Server
	QuickConnectServer       = quickconnect.Server
	SessionServer            = session.Server
	SubtitleServer           = subtitle.Server
	SuggestionsServer        = suggestions.Server
	SyncPlayServer           = syncplay.Server
	SystemServer             = system.Server
	TimeSyncServer           = timesync.Server
	TrailersServer           = trailers.Server
	UserServer               = user.Server
	UserLibraryServer        = userlibrary.Server
	UserViewsServer          = userviews.Server
)

type Server struct {
	*ApiKeyServer
	*FilterServer
	*YearsServer
	*SearchServer
	*StudiosServer
	*GenresServer
	*ImageServer
	*PersonsServer
	*LiveTvServer
	*TvShowsServer
	*DevicesServer
	*ScheduledTasksServer
	*DashboardServer
	*ChannelsServer
	*EnvironmentServer
	*ActivityLogServer
	*BrandingServer
	*ConfigurationServer
	*DisplayPreferencesServer
	*ItemsServer
	*ItemUpdateServer
	*LibraryServer
	*LibraryStructureServer
	*LocalizationServer
	*MediaInfoServer
	*PlaylistsServer
	*PlaystateServer
	*PluginsServer
	*PackagesServer
	*QuickConnectServer
	*SessionServer
	*SubtitleServer
	*SuggestionsServer
	*SyncPlayServer
	*SystemServer
	*TimeSyncServer
	*TrailersServer
	*UserServer
	*UserLibraryServer
	*UserViewsServer

	nestedUnimplemented
}

func New(
	apiKey *apikey.Server,
	filter *filter.Server,
	years *years.Server,
	search *search.Server,
	studios *studios.Server,
	genres *genres.Server,
	image *image.Server,
	persons *persons.Server,
	liveTv *livetv.Server,
	tvShows *tvshows.Server,
	devices *devices.Server,
	scheduledtasks *scheduledtasks.Server,
	dashboard *dashboard.Server,
	channels *channels.Server,
	environment *environment.Server,
	activityLog *activitylog.Server,
	branding *branding.Server,
	configuration *configuration.Server,
	displayPreferences *displaypreferences.Server,
	items *items.Server,
	itemUpdate *itemupdate.Server,
	library *library.Server,
	libraryStructure *librarystructure.Server,
	localization *localization.Server,
	mediaInfo *mediainfo.Server,
	playlists *playlists.Server,
	playstate *playstate.Server,
	plugins *plugins.Server,
	packages *packages.Server,
	quickConnect *quickconnect.Server,
	session *session.Server,
	subtitles *subtitle.Server,
	suggestions *suggestions.Server,
	syncPlay *syncplay.Server,
	system *system.Server,
	timeSync *timesync.Server,
	trailers *trailers.Server,
	user *user.Server,
	userLibrary *userlibrary.Server,
	userViews *userviews.Server,
) *Server {
	return &Server{
		ApiKeyServer:             apiKey,
		FilterServer:             filter,
		YearsServer:              years,
		SearchServer:             search,
		StudiosServer:            studios,
		GenresServer:             genres,
		ImageServer:              image,
		PersonsServer:            persons,
		LiveTvServer:             liveTv,
		TvShowsServer:            tvShows,
		DevicesServer:            devices,
		ScheduledTasksServer:     scheduledtasks,
		DashboardServer:          dashboard,
		ChannelsServer:           channels,
		EnvironmentServer:        environment,
		ActivityLogServer:        activityLog,
		BrandingServer:           branding,
		ConfigurationServer:      configuration,
		DisplayPreferencesServer: displayPreferences,
		ItemsServer:              items,
		ItemUpdateServer:         itemUpdate,
		LibraryServer:            library,
		LibraryStructureServer:   libraryStructure,
		LocalizationServer:       localization,
		MediaInfoServer:          mediaInfo,
		PlaylistsServer:          playlists,
		PlaystateServer:          playstate,
		PluginsServer:            plugins,
		PackagesServer:           packages,
		QuickConnectServer:       quickConnect,
		SessionServer:            session,
		SubtitleServer:           subtitles,
		SuggestionsServer:        suggestions,
		SyncPlayServer:           syncPlay,
		SystemServer:             system,
		TimeSyncServer:           timeSync,
		TrailersServer:           trailers,
		UserServer:               user,
		UserLibraryServer:        userLibrary,
		UserViewsServer:          userViews,
	}
}
