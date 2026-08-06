package libraries

import (
	"context"

	"github.com/FreekingDean/gojellyfin/internal/libraries"
)

// Declared here rather than imported so the scanner can depend on the domain
// package without a cycle.
type Scanner interface {
	Scan(ctx context.Context) error
}

type Server struct {
	store   *libraries.Store
	scanner Scanner
}

func New(store *libraries.Store) *Server {
	return &Server{store: store}
}

// Set after construction: the scanner reads libraries, so taking it as a
// constructor argument would make the object graph cyclic.
func (s *Server) UseScanner(scanner Scanner) {
	s.scanner = scanner
}
