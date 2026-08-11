package consts

import "fmt"

func DefaultProviderFor(domain string, name string) string {
	return fmt.Sprintf("Jellyfin.Server.Implementations.%s.%s", domain, name)
}
