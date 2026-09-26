package apikey

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/FreekingDean/gojellyfin/internal/apikeys"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
)

func TestAuthenticationInfo(t *testing.T) {
	revoked := time.Now().Add(-time.Hour)
	pending := time.Now().Add(time.Hour)

	tests := []struct {
		name        string
		revokedAt   *time.Time
		active      bool
		dateRevoked *time.Time
	}{
		{"an active key", nil, true, nil},
		{"a revoked key", &revoked, false, &revoked},
		{"a key revoked in the future", &pending, true, nil},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			info := authenticationInfo(&apikeys.ApiKey{RevokedAt: test.revokedAt})
			assert.Equal(t, apiutil.Ptr(test.active), info.IsActive)
			assert.Equal(t, test.dateRevoked, info.DateRevoked)
		})
	}
}
