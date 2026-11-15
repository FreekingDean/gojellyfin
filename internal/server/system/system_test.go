package system

import (
	"context"
	"testing"

	"github.com/FreekingDean/gojellyfin/internal/auth"
	"github.com/FreekingDean/gojellyfin/internal/env"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	systemsvc "github.com/FreekingDean/gojellyfin/internal/system"
)

func testServer() *Server {
	return New(nil, systemsvc.New(env.Config{}))
}

func TestServer_GetPingSystem(t *testing.T) {
	response, err := testServer().GetPingSystem(context.Background(), api.GetPingSystemRequestObject{})
	if err != nil {
		t.Fatalf("failed to ping: %v", err)
	}

	pong, ok := response.(api.GetPingSystem200JSONResponse)
	if !ok {
		t.Fatalf("response = %T, want api.GetPingSystem200JSONResponse", response)
	}
	if string(pong) != "Jellyfin Server" {
		t.Errorf("ping = %q, want %q", string(pong), "Jellyfin Server")
	}
}

func TestServer_PostPingSystem(t *testing.T) {
	response, err := testServer().PostPingSystem(context.Background(), api.PostPingSystemRequestObject{})
	if err != nil {
		t.Fatalf("failed to ping: %v", err)
	}

	pong, ok := response.(api.PostPingSystem200JSONResponse)
	if !ok {
		t.Fatalf("response = %T, want api.PostPingSystem200JSONResponse", response)
	}
	if string(pong) != "Jellyfin Server" {
		t.Errorf("ping = %q, want %q", string(pong), "Jellyfin Server")
	}
}

func TestServer_GetEndpointInfo(t *testing.T) {
	for _, tc := range []struct {
		name        string
		remoteAddr  string
		localAddr   string
		isLocal     bool
		isInNetwork bool
	}{
		{name: "loopback", remoteAddr: "127.0.0.1:52341", isLocal: true, isInNetwork: true},
		{name: "ipv6 loopback", remoteAddr: "[::1]:52341", isLocal: true, isInNetwork: true},
		{name: "private", remoteAddr: "192.168.1.24:52341", isInNetwork: true},
		{name: "unique local ipv6", remoteAddr: "[fd00::1]:52341", isInNetwork: true},
		{name: "public", remoteAddr: "8.8.8.8:52341"},
		{name: "the server's own address", remoteAddr: "192.168.1.5:52341", localAddr: "192.168.1.5:8081", isLocal: true, isInNetwork: true},
		{name: "another host on the server's network", remoteAddr: "192.168.1.6:52341", localAddr: "192.168.1.5:8081", isInNetwork: true},
		{name: "unknown", remoteAddr: ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := auth.ContextWithAuthorization(context.Background(), auth.Authorization{RemoteAddr: tc.remoteAddr, LocalAddr: tc.localAddr})

			response, err := testServer().GetEndpointInfo(ctx, api.GetEndpointInfoRequestObject{})
			if err != nil {
				t.Fatalf("failed to get the endpoint info: %v", err)
			}

			info, ok := response.(api.GetEndpointInfo200JSONResponse)
			if !ok {
				t.Fatalf("response = %T, want api.GetEndpointInfo200JSONResponse", response)
			}
			if *info.IsLocal != tc.isLocal {
				t.Errorf("IsLocal = %v, want %v", *info.IsLocal, tc.isLocal)
			}
			if *info.IsInNetwork != tc.isInNetwork {
				t.Errorf("IsInNetwork = %v, want %v", *info.IsInNetwork, tc.isInNetwork)
			}
		})
	}
}

func TestServer_GetServerLogs(t *testing.T) {
	response, err := testServer().GetServerLogs(context.Background(), api.GetServerLogsRequestObject{})
	if err != nil {
		t.Fatalf("failed to list the logs: %v", err)
	}

	logs, ok := response.(api.GetServerLogs200JSONResponse)
	if !ok {
		t.Fatalf("response = %T, want api.GetServerLogs200JSONResponse", response)
	}
	if logs == nil || len(logs) != 0 {
		t.Errorf("logs = %v, want an empty list", logs)
	}
}
