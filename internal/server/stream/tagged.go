package stream

import (
	"net/http"
	"strings"

	"github.com/FreekingDean/gojellyfin/internal/auth"
	"github.com/FreekingDean/gojellyfin/internal/http/middleware"
	"github.com/google/uuid"
)

const tokenParameter = "api_key"

func (h *Handler) Tagged(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if middleware.TokenFrom(r) == "" {
			if token, ok := h.tagToken(r); ok {
				r = r.Clone(r.Context())
				query := r.URL.Query()
				query.Set(tokenParameter, token)
				r.URL.RawQuery = query.Encode()
			}
		}

		next(w, r)
	}
}

func (h *Handler) tagToken(r *http.Request) (string, bool) {
	query := r.URL.Query()
	tag := queryValue(query, "tag")
	source, err := uuid.Parse(queryValue(query, "mediaSourceId"))
	if tag == "" || err != nil {
		return "", false
	}

	id, ok := auth.StreamTagSession(tag)
	if !ok {
		return "", false
	}
	session, err := h.sessions.ByID(r.Context(), id)
	if err != nil || !auth.StreamTagValid(tag, session.AccessToken, source) {
		return "", false
	}
	if _, err := h.sessions.ByToken(r.Context(), session.AccessToken); err != nil {
		return "", false
	}

	return session.AccessToken, true
}

func queryValue(query map[string][]string, name string) string {
	for key, values := range query {
		if strings.EqualFold(key, name) && len(values) > 0 {
			return values[0]
		}
	}

	return ""
}
