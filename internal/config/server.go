package config

import (
	"gorm.io/gorm"

	"github.com/FreekingDean/gojellyfin/internal/system"
)

// Server identity, shared by every DTO that carries a ServerId.
const (
	ServerID     = "e10a32fca79342d7b8b9d96e255ce1bc"
	RootFolderID = "e9d5075a555c1cbc394eec4cef295274"
)

type Server struct {
	db     *gorm.DB
	system system.Service
}

func New(db *gorm.DB, system system.Service) *Server {
	return &Server{db: db, system: system}
}
