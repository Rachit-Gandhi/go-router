package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"

	_ "github.com/lib/pq"

	"github.com/Rachit-Gandhi/go-router/internal/apikeys"
	"github.com/Rachit-Gandhi/go-router/internal/auth"
	"github.com/Rachit-Gandhi/go-router/internal/config"
	"github.com/Rachit-Gandhi/go-router/internal/credits"
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
		Config: config.Config{Host: os.Getenv("CONTROL_HOST"), Port: port},
	}
	controlmux := http.NewServeMux()
	authHandle := auth.AuthHandler{Db: dbConnection}
	apiKeysHandle := apikeys.ApiKeysHandler{Db: dbConnection}
	creditsHandle := credits.CreditsHandler{Db: dbConnection, SQL: db}
	control := &http.Server{
		Addr:    fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Handler: controlmux,
	}
	controlmux.HandleFunc("POST /users", authHandle.SignupHandler)
	controlmux.HandleFunc("POST /login", authHandle.LoginHandler)

	controlmux.Handle("GET /api-keys", authHandle.AuthMiddleware(http.HandlerFunc(apiKeysHandle.GetApiKeysHandler)))
	controlmux.Handle("POST /api-keys", authHandle.AuthMiddleware(http.HandlerFunc(apiKeysHandle.CreateApiKeyHandler)))
	controlmux.Handle("PATCH /api-keys/revoke", authHandle.AuthMiddleware(http.HandlerFunc(apiKeysHandle.RevokeApiKeyHandler)))
	controlmux.Handle("DELETE /api-keys", authHandle.AuthMiddleware(http.HandlerFunc(apiKeysHandle.DeleteApiKeyHandler)))
	controlmux.Handle("GET /credits", authHandle.AuthMiddleware(http.HandlerFunc(creditsHandle.GetBalanceHandler)))
	controlmux.Handle("POST /credits/topup", authHandle.AuthMiddleware(http.HandlerFunc(creditsHandle.MockTopupHandler)))
	fmt.Printf("Server starting on %s:%d\n", cfg.Host, cfg.Port)
	log.Fatal(control.ListenAndServe())
}
