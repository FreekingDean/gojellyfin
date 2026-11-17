package librarystructure

import (
	"testing"

	"github.com/FreekingDean/gojellyfin/internal/server/apitest"
)

func TestOptionsTranslation(t *testing.T) {
	apitest.Reads(t, mergeOptions, map[string]string{
		"PathInfos": "a library's folders are what its downloaders map, so the round trip jellyfin-web posts back is dropped",
	})
	apitest.Writes(t, optionsDto, map[string]string{})
}
