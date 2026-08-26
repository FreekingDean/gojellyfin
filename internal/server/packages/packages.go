package packages

import (
	"context"

	"github.com/FreekingDean/gojellyfin/internal/server/api"
)

type Server struct{}

func New() *Server {
	return &Server{}
}

func (s *Server) GetPackages(ctx context.Context, request api.GetPackagesRequestObject) (api.GetPackagesResponseObject, error) {
	return api.GetPackages200JSONResponse([]api.PackageInfo{}), nil
}

func (s *Server) GetPackageInfo(ctx context.Context, request api.GetPackageInfoRequestObject) (api.GetPackageInfoResponseObject, error) {
	return api.GetPackageInfo200JSONResponse(api.PackageInfo{}), nil
}

func (s *Server) GetRepositories(ctx context.Context, request api.GetRepositoriesRequestObject) (api.GetRepositoriesResponseObject, error) {
	return api.GetRepositories200JSONResponse([]api.RepositoryInfo{}), nil
}

func (s *Server) InstallPackage(ctx context.Context, request api.InstallPackageRequestObject) (api.InstallPackageResponseObject, error) {
	return api.InstallPackage404JSONResponse{}, nil
}

func (s *Server) CancelPackageInstallation(ctx context.Context, request api.CancelPackageInstallationRequestObject) (api.CancelPackageInstallationResponseObject, error) {
	return api.CancelPackageInstallation403Response{}, nil
}

func (s *Server) SetRepositories(ctx context.Context, request api.SetRepositoriesRequestObject) (api.SetRepositoriesResponseObject, error) {
	return api.SetRepositories403Response{}, nil
}
