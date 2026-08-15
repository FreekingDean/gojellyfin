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

func TestWorkerModulesResolve(t *testing.T) {
	if err := fx.ValidateApp(workerModules); err != nil {
		t.Fatalf("worker modules do not compose: %v", err)
	}
}
