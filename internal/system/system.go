package system

import "github.com/FreekingDean/gojellyfin/internal/env"

type Service interface {
	LocalAddress() string
	OperatingSystem() string
	PackageName() string
	ProductName() string
	Version() string
}

type serviceImpl struct {
	localAddress    string
	operatingSystem string
	packageName     string
	productName     string
	version         string
}

func New(config env.Config) Service {
	return serviceImpl{
		localAddress:    config.PublishedServerURL,
		version:         JellyfinVersion,
		operatingSystem: "linux",
		packageName:     Build(),
		productName:     "Jellyfin Server",
	}
}

func (s serviceImpl) LocalAddress() string {
	return s.localAddress
}

func (s serviceImpl) PackageName() string {
	return s.packageName
}

func (s serviceImpl) ProductName() string {
	return s.productName
}

func (s serviceImpl) Version() string {
	return s.version
}

func (s serviceImpl) OperatingSystem() string {
	return s.operatingSystem
}
