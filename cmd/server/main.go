package main

import (
	"go.uber.org/fx"

	"github.com/FreekingDean/gojellyfin/internal/http"
	"github.com/FreekingDean/gojellyfin/internal/observability"
	"github.com/FreekingDean/gojellyfin/internal/scanner"
	"github.com/FreekingDean/gojellyfin/internal/server"
	"github.com/FreekingDean/gojellyfin/internal/store"
	"github.com/FreekingDean/gojellyfin/internal/system"
)

func main() {
	fx.New(
		observability.Module,
		store.Module,
		system.Module,
		scanner.Module,
		server.Module,
		http.Module,
	).Run()
}
