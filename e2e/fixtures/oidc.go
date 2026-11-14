package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/FreekingDean/gojellyfin/internal/sso/ssotest"
)

func oidc() error {
	redirect := os.Getenv("OIDC_REDIRECT_URI")
	username := os.Getenv("OIDC_USERNAME")
	if redirect == "" || username == "" {
		return fmt.Errorf("OIDC_REDIRECT_URI and OIDC_USERNAME are both required")
	}

	provider, err := ssotest.Start()
	if err != nil {
		return err
	}
	defer provider.Close()

	provider.Register(redirect)
	provider.SignIn(ssotest.Claims{"sub": "e2e-" + username, "preferred_username": username})

	fmt.Println(provider.URL)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, os.Interrupt)
	<-stop

	return nil
}
