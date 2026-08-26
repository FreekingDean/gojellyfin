package system

import "fmt"

var (
	buildVersion = "dev"
	buildCommit  = "unknown"
	buildDate    = "unknown"
)

func Build() string {
	return fmt.Sprintf("gojellyfin %s (%s, %s)", buildVersion, buildCommit, buildDate)
}
