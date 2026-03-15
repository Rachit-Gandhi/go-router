package httpapi

import (
	"net/http"

	"github.com/Rachit-Gandhi/go-router/internal/httputil"
)

// NewHandler builds the router-plane HTTP router.
func NewHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/router/healthz", httputil.HealthHandler())

	return mux
}
