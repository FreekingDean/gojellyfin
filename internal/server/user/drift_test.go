package user

import (
	"testing"

	"github.com/FreekingDean/gojellyfin/internal/server/apitest"
)

func TestPolicyTranslation(t *testing.T) {
	apitest.Reads(t, mergePolicy, map[string]string{})
	apitest.Writes(t, policyDto, map[string]string{})
}

func TestConfigurationTranslation(t *testing.T) {
	apitest.Reads(t, mergeConfiguration, map[string]string{})
	apitest.Writes(t, configurationDto, map[string]string{})
}
