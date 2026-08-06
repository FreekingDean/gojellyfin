package libraries

import (
	"context"

	"gorm.io/gorm"
)

// Declared here rather than imported so the scanner can depend on this package
// for its models without a cycle.
type Scanner interface {
	Scan(ctx context.Context) error
}

type Server struct {
	db      *gorm.DB
	scanner Scanner
}

func New(db *gorm.DB) *Server {
	return &Server{db: db}
}

// Set after construction: the scanner reads libraries, so taking it as a
// constructor argument would make the object graph cyclic.
func (s *Server) UseScanner(scanner Scanner) {
	s.scanner = scanner
}
