package main

import (
	"testing"

	"go.uber.org/fx"
)

func TestServerModulesResolve(t *testing.T) {
	if err := fx.ValidateApp(serverModules); err != nil {
		t.Fatalf("server modules do not compose: %v", err)
	}
}
