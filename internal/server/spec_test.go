package server

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestOwnSpecDeclaresSecurity(t *testing.T) {
	path := filepath.Join("..", "..", "spec", "gojellyfin.json")

	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read the spec: %v", err)
	}

	var document struct {
		Paths map[string]map[string]struct {
			OperationID string            `json:"operationId"`
			Security    *[]map[string]any `json:"security"`
		} `json:"paths"`
	}
	if err := json.Unmarshal(body, &document); err != nil {
		t.Fatalf("failed to parse the spec: %v", err)
	}

	operations := 0
	for route, methods := range document.Paths {
		for method, operation := range methods {
			if operation.OperationID == "" {
				continue
			}
			operations++

			if operation.Security == nil || len(*operation.Security) == 0 {
				t.Errorf(
					"%s %s (%s) declares no security, which makes it public: "+
						"the generator reads a missing security block as PublicOperations",
					method, route, operation.OperationID,
				)
			}
		}
	}

	if operations == 0 {
		t.Fatal("the spec declared no operations, so this guard checked nothing")
	}
}
