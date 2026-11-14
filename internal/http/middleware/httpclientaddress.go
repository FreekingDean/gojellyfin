package middleware

import (
	"net"
	"net/http"
	"net/netip"
	"strings"
)

func HttpClientAddress(trusted []netip.Prefix) HttpMiddleware {
	return func(next http.Handler) http.Handler {
		if len(trusted) == 0 {
			return next
		}

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if client, ok := forwardedClient(r, trusted); ok {
				r.RemoteAddr = net.JoinHostPort(client.String(), "0")
			}
			next.ServeHTTP(w, r)
		})
	}
}

func forwardedClient(r *http.Request, trusted []netip.Prefix) (netip.Addr, bool) {
	peer, err := netip.ParseAddrPort(r.RemoteAddr)
	if err != nil || !within(peer.Addr().Unmap(), trusted) {
		return netip.Addr{}, false
	}

	hops := strings.Split(strings.Join(r.Header.Values("X-Forwarded-For"), ","), ",")
	for i := len(hops) - 1; i >= 0; i-- {
		hop, err := netip.ParseAddr(strings.TrimSpace(hops[i]))
		if err != nil {
			return netip.Addr{}, false
		}
		if hop = hop.Unmap(); !within(hop, trusted) {
			return hop, true
		}
	}

	return netip.Addr{}, false
}

func within(addr netip.Addr, networks []netip.Prefix) bool {
	for _, network := range networks {
		if network.Contains(addr) {
			return true
		}
	}

	return false
}

func Prefixes(networks []string) []netip.Prefix {
	parsed := make([]netip.Prefix, 0, len(networks))
	for _, network := range networks {
		if prefix, err := netip.ParsePrefix(network); err == nil {
			parsed = append(parsed, prefix.Masked())
		}
	}

	return parsed
}
