package main

import (
	"log"
	"net/http"
	"os"

	"github.com/Rachit-Gandhi/go-router/internal/control/httpapi"
)

func main() {
	addr := envOrDefault("CONTROL_ADDR", ":8080")
	log.Printf("control listening on %s", addr)

	if err := http.ListenAndServe(addr, httpapi.NewHandler()); err != nil {
		log.Fatal(err)
	}
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
