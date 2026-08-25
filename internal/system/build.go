package system

import "fmt"

// Set with -ldflags -X by the Makefile and the Dockerfile; a plain `go build`
// leaves the defaults.
var (
	buildVersion = "dev"
	buildCommit  = "unknown"
	buildDate    = "unknown"
)

func Build() string {
	return fmt.Sprintf("gojellyfin %s (%s, %s)", buildVersion, buildCommit, buildDate)
}
