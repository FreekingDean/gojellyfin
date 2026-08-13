package pprof

import (
	"net/http"
	"testing"

	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
)

func TestTheModuleListens(t *testing.T) {
	t.Setenv("PPROF_ADDR", "127.0.0.1:6199")

	app := fxtest.New(t, Module)
	app.RequireStart()
	defer app.RequireStop()

	response, err := http.Get("http://127.0.0.1:6199/debug/pprof/goroutine?debug=1")
	if err != nil {
		t.Fatalf("the module started but nothing is listening: %v", err)
	}
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", response.StatusCode)
	}
}

func TestTheModuleStaysOffWhenUnset(t *testing.T) {
	t.Setenv("PPROF_ADDR", "")

	app := fxtest.New(t, Module, fx.NopLogger)
	app.RequireStart()
	app.RequireStop()

	if _, err := http.Get("http://127.0.0.1:6199/debug/pprof/goroutine"); err == nil {
		t.Error("something answered on the pprof port with PPROF_ADDR unset")
	}
}
