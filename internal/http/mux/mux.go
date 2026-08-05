package mux

import (
	"log"
	"net/http"
	"regexp"
	"strings"
)

type Mux struct {
	routesByMethod map[string][]route
	errs           []error
}

type route struct {
	pattern     regexp.Regexp
	handler     http.Handler
	routeString string
}

var (
	HTTP_METHODS = []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS", "HEAD"}
)

func New() *Mux {
	routes := make(map[string][]route)
	for _, method := range HTTP_METHODS {
		routes[method] = make([]route, 0)
	}
	return &Mux{
		routesByMethod: routes,

		errs: make([]error, 0),
	}
}

func (m *Mux) HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request)) {
	method := ""
	if i := strings.IndexByte(pattern, ' '); i >= 0 {
		method, pattern = pattern[:i], pattern[i+1:]
	}

	reg, err := compile(pattern)
	if err != nil {
		m.errs = append(m.errs, err)
		return
	}

	m.routesByMethod[method] = append(m.routesByMethod[method], route{
		pattern:     *reg,
		handler:     http.HandlerFunc(handler),
		routeString: pattern,
	})
}

func (m *Mux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	for _, route := range m.routesByMethod[r.Method] {
		matches := route.pattern.FindStringSubmatch(r.URL.Path)
		names := route.pattern.SubexpNames()
		if len(matches) != 0 && len(matches) == len(names) {
			for i, name := range names {
				r.SetPathValue(name, matches[i])
			}
			route.handler.ServeHTTP(w, r)
			return
		}
	}

	w.WriteHeader(http.StatusNotFound)
	log.Printf("No route found for %s %s", r.Method, r.URL.Path)
}

func compile(pattern string) (*regexp.Regexp, error) {
	var b strings.Builder
	b.WriteString("(?i)^")
	for _, s := range strings.Split(strings.Trim(pattern, "/"), "/") {
		b.WriteString("/")
		switch {
		case s == "*": // catch-all, e.g. /web/*  — swallows remaining slashes
			b.WriteString("(?P<rest>.*)")
		case strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}"):
			b.WriteString("(?P<" + s[1:len(s)-1] + ">[^/]+)")
		default:
			b.WriteString(regexp.QuoteMeta(s)) // escape . in things like stream.mp4
		}
	}
	b.WriteString("/?$") // tolerate a trailing slash for free
	reg, err := regexp.Compile(b.String())
	return reg, err
}
