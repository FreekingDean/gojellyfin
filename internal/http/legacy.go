package http

import (
	"net/http"
	"strings"

	"github.com/FreekingDean/gojellyfin/internal/http/mux"
)

// jellyfin-web still calls pre-10.9 paths that were dropped from the 10.10
// spec, so the generated router has no route for them and they 404. Each is
// rewritten onto its modern equivalent and re-dispatched; the user id moves to
// the query string, where the replacement operations declare it.
// Ordered, not a map: the literal paths must be registered before the
// {itemId} pattern, which would otherwise swallow /Items/Latest and friends.
var legacyRoutes = []struct{ from, to string }{
	{"/Users/{userId}/Items/Latest", "/Items/Latest"},
	{"/Users/{userId}/Items/Resume", "/UserItems/Resume"},
	{"/Users/{userId}/Items/Root", "/Items/Root"},
	{"/Users/{userId}/Items/Suggestions", "/Items/Suggestions"},
	{"/Users/{userId}/Items/{itemId}", "/Items/{itemId}"},
	{"/Users/{userId}/Items", "/Items"},
	{"/Users/{userId}/Views", "/UserViews"},
	{"/Users/{userId}/FavoriteItems/{itemId}", "/UserFavoriteItems/{itemId}"},
	{"/Users/{userId}/PlayedItems/{itemId}", "/UserPlayedItems/{itemId}"},
	{"/Users/{userId}/PlayingItems/{itemId}", "/PlayingItems/{itemId}"},
}

var legacyMethods = []string{"GET", "POST", "DELETE"}

func registerLegacyRoutes(m *mux.Mux) {
	for _, route := range legacyRoutes {
		for _, method := range legacyMethods {
			m.HandleFunc(method+" "+route.from, legacyHandler(m, route.to))
		}
	}
}

func legacyHandler(m *mux.Mux, target string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := target
		if strings.Contains(path, "{itemId}") {
			path = strings.ReplaceAll(path, "{itemId}", r.PathValue("itemId"))
		}

		rewritten := r.Clone(r.Context())
		rewritten.URL.Path = path

		if userID := r.PathValue("userId"); userID != "" && rewritten.URL.Query().Get("userId") == "" {
			query := rewritten.URL.Query()
			query.Set("userId", userID)
			rewritten.URL.RawQuery = query.Encode()
		}

		m.ServeHTTP(w, rewritten)
	}
}
