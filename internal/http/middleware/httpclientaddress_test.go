package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHttpClientAddress(t *testing.T) {
	tests := []struct {
		name      string
		trusted   []string
		peer      string
		forwarded []string
		want      string
	}{
		{
			name:      "takes the client the gateway names",
			trusted:   []string{"10.244.0.0/16"},
			peer:      "10.244.4.7:51000",
			forwarded: []string{"203.0.113.9"},
			want:      "203.0.113.9:0",
		},
		{
			name:      "skips the proxies it trusts from the right",
			trusted:   []string{"10.244.0.0/16"},
			peer:      "10.244.4.7:51000",
			forwarded: []string{"198.51.100.1, 203.0.113.9, 10.244.9.9"},
			want:      "203.0.113.9:0",
		},
		{
			name:      "reads every header line as one list",
			trusted:   []string{"10.244.0.0/16"},
			peer:      "10.244.4.7:51000",
			forwarded: []string{"198.51.100.1", "203.0.113.9"},
			want:      "203.0.113.9:0",
		},
		{
			name:      "ignores the header from a peer it does not trust",
			trusted:   []string{"10.244.0.0/16"},
			peer:      "203.0.113.50:51000",
			forwarded: []string{"198.51.100.1"},
			want:      "203.0.113.50:51000",
		},
		{
			name:      "keeps the peer when the header is unreadable",
			trusted:   []string{"10.244.0.0/16"},
			peer:      "10.244.4.7:51000",
			forwarded: []string{"not-an-address"},
			want:      "10.244.4.7:51000",
		},
		{
			name:      "keeps the peer when every hop is a proxy",
			trusted:   []string{"10.244.0.0/16"},
			peer:      "10.244.4.7:51000",
			forwarded: []string{"10.244.1.1"},
			want:      "10.244.4.7:51000",
		},
		{
			name:      "trusts nobody unless told to",
			peer:      "10.244.4.7:51000",
			forwarded: []string{"203.0.113.9"},
			want:      "10.244.4.7:51000",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/link", nil)
			request.RemoteAddr = tt.peer
			for _, line := range tt.forwarded {
				request.Header.Add("X-Forwarded-For", line)
			}

			got := ""
			HttpClientAddress(Prefixes(tt.trusted))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				got = r.RemoteAddr
			})).ServeHTTP(httptest.NewRecorder(), request)

			if got != tt.want {
				t.Errorf("RemoteAddr = %q, want %q", got, tt.want)
			}
		})
	}
}
