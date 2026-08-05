package system

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

func New() Service {
	return serviceImpl{
		localAddress:    "http://localhost:8080",
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
