package library

import (
	"context"
	"log"

	"github.com/FreekingDean/gojellyfin/internal/server/api"
)

type Scanner interface {
	Scan(ctx context.Context) error
}

type Server struct {
	scanner Scanner
}

func New() *Server {
	return &Server{}
}

// Set after construction: the scanner reads libraries, so taking it as a
// constructor argument would make the object graph cyclic.
func (s *Server) UseScanner(scanner Scanner) {
	s.scanner = scanner
}

func (s *Server) RefreshLibrary(ctx context.Context, request api.RefreshLibraryRequestObject) (api.RefreshLibraryResponseObject, error) {
	go func() {
		if err := s.scanner.Scan(context.Background()); err != nil {
			log.Printf("refresh library: %v", err)
		}
	}()

	return api.RefreshLibrary204Response{}, nil
}
