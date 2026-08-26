package main

import (
	"testing"

	"go.uber.org/fx"
)

func TestServerModules(t *testing.T) {
	if err := fx.ValidateApp(serverModules); err != nil {
		t.Fatalf("server modules do not compose: %v", err)
	}
}

func TestWorkerModules(t *testing.T) {
	if err := fx.ValidateApp(workerModules); err != nil {
		t.Fatalf("worker modules do not compose: %v", err)
	}
}
