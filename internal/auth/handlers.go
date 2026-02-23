package auth

import (
	"net/http"

	"github.com/Rachit-Gandhi/go-router/internal/config"
)

// ControlServer wraps ControlConfig so auth can define handlers that have access to config and Db.
type ControlServer struct {
	*config.ControlConfig
}

func (s *ControlServer) SignupHandler(w http.ResponseWriter, r *http.Request) {
	// s.Db, s.Host, s.Port available via embedded ControlConfig
}
