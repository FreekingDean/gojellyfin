package artwork

import (
	"github.com/FreekingDean/gojellyfin/internal/artwork/postgres"
	"github.com/FreekingDean/gojellyfin/internal/store"
)

func New(client *store.Client) Store {
	return postgres.New(client)
}
