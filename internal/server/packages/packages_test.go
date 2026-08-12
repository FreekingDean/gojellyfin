package packages

import (
	"context"
	"fmt"
	"testing"

	"github.com/FreekingDean/gojellyfin/internal/server/api"
)

func TestReadsAreEmpty(t *testing.T) {
	server := New()
	ctx := context.Background()

	t.Run("GetPackages", func(t *testing.T) {
		response, err := server.GetPackages(ctx, api.GetPackagesRequestObject{})
		if err != nil {
			t.Fatal(err)
		}
		if catalogue := response.(api.GetPackages200JSONResponse); len(catalogue) != 0 {
			t.Errorf("got %d packages, want 0", len(catalogue))
		}
	})

	t.Run("GetRepositories", func(t *testing.T) {
		response, err := server.GetRepositories(ctx, api.GetRepositoriesRequestObject{})
		if err != nil {
			t.Fatal(err)
		}
		if repositories := response.(api.GetRepositories200JSONResponse); len(repositories) != 0 {
			t.Errorf("got %d repositories, want 0", len(repositories))
		}
	})

	t.Run("GetPackageInfo", func(t *testing.T) {
		response, err := server.GetPackageInfo(ctx, api.GetPackageInfoRequestObject{Name: "Anime"})
		if err != nil {
			t.Fatal(err)
		}
		if info := response.(api.GetPackageInfo200JSONResponse); info.Name != nil {
			t.Errorf("got package %q, want none", *info.Name)
		}
	})
}

func TestMutationsAreRefused(t *testing.T) {
	server := New()
	ctx := context.Background()

	for _, tc := range []struct {
		name string
		call func() (any, error)
		want any
	}{
		{
			name: "InstallPackage",
			call: func() (any, error) {
				return server.InstallPackage(ctx, api.InstallPackageRequestObject{Name: "Anime"})
			},
			want: api.InstallPackage404JSONResponse{},
		},
		{
			name: "CancelPackageInstallation",
			call: func() (any, error) {
				return server.CancelPackageInstallation(ctx, api.CancelPackageInstallationRequestObject{})
			},
			want: api.CancelPackageInstallation403Response{},
		},
		{
			name: "SetRepositories",
			call: func() (any, error) {
				return server.SetRepositories(ctx, api.SetRepositoriesRequestObject{})
			},
			want: api.SetRepositories403Response{},
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
