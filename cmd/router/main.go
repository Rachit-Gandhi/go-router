package main

import (
	"log"
	"net/http"
	"os"

	"github.com/Rachit-Gandhi/go-router/internal/router/httpapi"
)

func main() {
	addr := envOrDefault("ROUTER_ADDR", ":8081")
	log.Printf("router listening on %s", addr)

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
