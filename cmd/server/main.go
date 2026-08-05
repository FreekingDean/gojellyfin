package main

import (
	"go.uber.org/fx"

	"github.com/FreekingDean/gojellyfin/internal/http"
	"github.com/FreekingDean/gojellyfin/internal/server"
	"github.com/FreekingDean/gojellyfin/internal/store"
	"github.com/FreekingDean/gojellyfin/internal/system"
)

func main() {
	fx.New(
		store.Module,
		system.Module,
		server.Module,
		http.Module,
	).Run()
}
