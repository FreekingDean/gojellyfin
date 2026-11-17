package plugins

import (
	"context"
	"fmt"
	"testing"

	"github.com/FreekingDean/gojellyfin/internal/server/api"
)

func TestServer_GetPlugins(t *testing.T) {
	response, err := New().GetPlugins(context.Background(), api.GetPluginsRequestObject{})
	if err != nil {
		t.Fatal(err)
	}

	if installed := response.(api.GetPlugins200JSONResponse); len(installed) != 0 {
		t.Errorf("got %d plugins, want 0", len(installed))
	}
}

func TestServer(t *testing.T) {
	for _, tc := range serverCases {
		t.Run(tc.name, testServerCase(tc))
	}
}

var serverCases = []serverCase{
	{
		name: "GetPluginConfiguration",
		call: func(server *Server, ctx context.Context) (any, error) {
			return server.GetPluginConfiguration(ctx, api.GetPluginConfigurationRequestObject{})
		},
		want: api.GetPluginConfiguration404JSONResponse{},
	},
	{
		name: "GetPluginManifest",
		call: func(server *Server, ctx context.Context) (any, error) {
			return server.GetPluginManifest(ctx, api.GetPluginManifestRequestObject{})
		},
		want: api.GetPluginManifest404JSONResponse{},
	},
	{
		name: "GetPluginImage",
		call: func(server *Server, ctx context.Context) (any, error) {
			return server.GetPluginImage(ctx, api.GetPluginImageRequestObject{})
		},
		want: api.GetPluginImage404JSONResponse{},
	},
	{
		name: "UpdatePluginConfiguration",
		call: func(server *Server, ctx context.Context) (any, error) {
			return server.UpdatePluginConfiguration(ctx, api.UpdatePluginConfigurationRequestObject{})
		},
		want: api.UpdatePluginConfiguration404JSONResponse{},
	},
	{
		name: "UninstallPlugin",
		call: func(server *Server, ctx context.Context) (any, error) {
			return server.UninstallPlugin(ctx, api.UninstallPluginRequestObject{})
		},
		want: api.UninstallPlugin404JSONResponse{},
	},
	{
		name: "UninstallPluginByVersion",
		call: func(server *Server, ctx context.Context) (any, error) {
			return server.UninstallPluginByVersion(ctx, api.UninstallPluginByVersionRequestObject{})
		},
		want: api.UninstallPluginByVersion404JSONResponse{},
	},
	{
		name: "EnablePlugin",
		call: func(server *Server, ctx context.Context) (any, error) {
			return server.EnablePlugin(ctx, api.EnablePluginRequestObject{})
		},
		want: api.EnablePlugin404JSONResponse{},
	},
	{
		name: "DisablePlugin",
		call: func(server *Server, ctx context.Context) (any, error) {
			return server.DisablePlugin(ctx, api.DisablePluginRequestObject{})
		},
		want: api.DisablePlugin404JSONResponse{},
	},
}

func testServerCase(tc serverCase) func(*testing.T) {
	return func(t *testing.T) {
		response, err := tc.call(New(), context.Background())
		if err != nil {
			t.Fatal(err)
		}

		if got, want := fmt.Sprintf("%T", response), fmt.Sprintf("%T", tc.want); got != want {
			t.Errorf("got %s, want %s", got, want)
		}
	}
}

type serverCase struct {
	name string
	call func(*Server, context.Context) (any, error)
	want any
}
