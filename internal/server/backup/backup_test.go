package backup

import (
	"context"
	"testing"

	"github.com/FreekingDean/gojellyfin/internal/server/api"
)

func TestServer_ListBackups(t *testing.T) {
	response, err := New().ListBackups(context.Background(), api.ListBackupsRequestObject{})
	if err != nil {
		t.Fatalf("failed to list the backups: %v", err)
	}

	backups, ok := response.(api.ListBackups200JSONResponse)
	if !ok {
		t.Fatalf("response = %T, want api.ListBackups200JSONResponse", response)
	}
	if backups == nil || len(backups) != 0 {
		t.Errorf("backups = %v, want an empty list", backups)
	}
}
