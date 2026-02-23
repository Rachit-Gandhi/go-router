package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/Rachit-Gandhi/go-router/internal/auth"
	"github.com/Rachit-Gandhi/go-router/internal/config"
	"github.com/Rachit-Gandhi/go-router/internal/database"
	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	dbURL := os.Getenv("DB_CONNECTION_URL")
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatalf("error connecting to db: %v", err)
	}
	dbConnection := database.New(db)
	port, err := strconv.Atoi(os.Getenv("CONTROL_PORT"))
	if err != nil {
		log.Fatal("Error converting CONTROL_PORT to int")
	}
	cfg := config.ControlConfig{
		Config: config.Config{Host: os.Getenv("CONTROL_HOST"), Port: port, Db: dbConnection},
	}
	server := &auth.ControlServer{ControlConfig: &cfg}
	controlmux := http.NewServeMux()
	control := &http.Server{
		Addr:    fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Handler: controlmux,
	}
	controlmux.HandleFunc("POST /users", server.SignupHandler)
	fmt.Printf("Server starting on %s:%d\n", cfg.Host, cfg.Port)
	log.Fatal(control.ListenAndServe())
}
