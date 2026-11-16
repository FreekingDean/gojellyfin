package dto

import (
	"github.com/FreekingDean/gojellyfin/internal/config"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
)

func CastReceiverApplications() *[]api.CastReceiverApplication {
	applications := make([]api.CastReceiverApplication, 0, len(config.CastReceivers))
	for _, receiver := range config.CastReceivers {
		applications = append(applications, api.CastReceiverApplication{Id: receiver.ID, Name: receiver.Name})
	}

	return &applications
}
