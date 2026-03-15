package main

import (
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Rachit-Gandhi/go-router/internal/config"
	"github.com/Rachit-Gandhi/go-router/internal/control/httpapi"
	"github.com/Rachit-Gandhi/go-router/internal/server"
)

func main() {
	addr := config.EnvOrDefault("CONTROL_ADDR", ":8080")
	log.Printf("control listening on %s", addr)

	srv := &http.Server{
		Addr:    addr,
		Handler: httpapi.NewHandler(),
	}

	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signalCh)

	if err := server.Run(srv, signalCh, 30*time.Second); err != nil {
		log.Fatal(err)
	}
}
