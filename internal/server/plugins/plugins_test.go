package plugins

import (
	"context"
	"fmt"
	"testing"

	"github.com/FreekingDean/gojellyfin/internal/server/api"
)

func TestGetPluginsIsEmpty(t *testing.T) {
	response, err := New().GetPlugins(context.Background(), api.GetPluginsRequestObject{})
	if err != nil {
		t.Fatal(err)
	}

	if installed := response.(api.GetPlugins200JSONResponse); len(installed) != 0 {
		t.Errorf("got %d plugins, want 0", len(installed))
	}
}

func TestEveryOtherOperationIsRefused(t *testing.T) {
	server := New()
	ctx := context.Background()

	for _, tc := range []struct {
		name string
		call func() (any, error)
		want any
	}{
		{
			name: "GetPluginConfiguration",
			call: func() (any, error) {
				return server.GetPluginConfiguration(ctx, api.GetPluginConfigurationRequestObject{})
			},
			want: api.GetPluginConfiguration404JSONResponse{},
		},
		{
			name: "GetPluginManifest",
			call: func() (any, error) {
				return server.GetPluginManifest(ctx, api.GetPluginManifestRequestObject{})
			},
			want: api.GetPluginManifest404JSONResponse{},
		},
		{
			name: "GetPluginImage",
			call: func() (any, error) {
				return server.GetPluginImage(ctx, api.GetPluginImageRequestObject{})
			},
			want: api.GetPluginImage404JSONResponse{},
		},
		{
			name: "UpdatePluginConfiguration",
			call: func() (any, error) {
				return server.UpdatePluginConfiguration(ctx, api.UpdatePluginConfigurationRequestObject{})
			},
			want: api.UpdatePluginConfiguration404JSONResponse{},
		},
		{
			name: "UninstallPlugin",
			call: func() (any, error) {
				return server.UninstallPlugin(ctx, api.UninstallPluginRequestObject{})
			},
			want: api.UninstallPlugin404JSONResponse{},
		},
		{
			name: "UninstallPluginByVersion",
			call: func() (any, error) {
				return server.UninstallPluginByVersion(ctx, api.UninstallPluginByVersionRequestObject{})
			},
			want: api.UninstallPluginByVersion404JSONResponse{},
		},
		{
			name: "EnablePlugin",
			call: func() (any, error) {
				return server.EnablePlugin(ctx, api.EnablePluginRequestObject{})
			},
			want: api.EnablePlugin404JSONResponse{},
		},
		{
			name: "DisablePlugin",
			call: func() (any, error) {
				return server.DisablePlugin(ctx, api.DisablePluginRequestObject{})
			},
			want: api.DisablePlugin404JSONResponse{},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			response, err := tc.call()
			if err != nil {
				t.Fatal(err)
			}

			if got, want := fmt.Sprintf("%T", response), fmt.Sprintf("%T", tc.want); got != want {
				t.Errorf("got %s, want %s", got, want)
			}
		})
	}
}
