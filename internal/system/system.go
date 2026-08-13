package system

import "os"

type Service interface {
	LocalAddress() string
	OperatingSystem() string
	ProductName() string
	Version() string
}

type serviceImpl struct {
	localAddress    string
	operatingSystem string
	productName     string
	version         string
}

var (
	VERSION = "10.10.0"
)

// Clients switch to LocalAddress when they believe they are on the same
// network, so an address this server cannot confirm is worse than none: they
// stop talking to the address that reached them and never come back.
func New() Service {
	return serviceImpl{
		localAddress:    os.Getenv("PUBLISHED_SERVER_URL"),
		version:         VERSION,
		operatingSystem: "linux",
		productName:     "Jellyfin Server",
	}
}

func (s serviceImpl) LocalAddress() string {
	return s.localAddress
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
