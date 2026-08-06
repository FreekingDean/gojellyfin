package items

import (
	"gorm.io/gorm"

	"github.com/FreekingDean/gojellyfin/internal/server/libraries"
)

type Server struct {
	db        *gorm.DB
	libraries *libraries.Server
}

func New(db *gorm.DB, libraries *libraries.Server) *Server {
	return &Server{db: db, libraries: libraries}
}
