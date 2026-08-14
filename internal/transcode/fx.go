package transcode

import (
	"os"
	"strconv"
	"time"

	"go.uber.org/fx"
)

var Module = fx.Module(
	"transcode",
	fx.Provide(New),
)

func New() *Encoder {
	return NewEncoder(jobs(), stall())
}

func jobs() int {
	value, _ := strconv.Atoi(os.Getenv("TRANSCODER_JOBS"))

	return value
}

func stall() time.Duration {
	value, _ := time.ParseDuration(os.Getenv("TRANSCODER_STALL_TIMEOUT"))

	return value
}
