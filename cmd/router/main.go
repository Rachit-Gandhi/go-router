package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/Rachit-Gandhi/go-router/internal/config"
	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	//dbURL := os.Getenv("DB_CONNECTION_URL")
	/*db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatalf("error connecting to db: %v", err)
	}*/
	//dbConnection := database.New(db)
	port, err := strconv.Atoi(os.Getenv("ROUTER_PORT"))
	if err != nil {
		log.Fatal("Error converting ROUTER_PORT to int")
	}
	cfg := config.RouterConfig{
		Config: config.Config{Host: os.Getenv("ROUTER_HOST"), Port: port},
	}
	routermux := http.NewServeMux()
	router := &http.Server{
		Addr:    fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Handler: routermux,
	}
	fmt.Printf("Server starting on %s:%d\n", cfg.Host, cfg.Port)
	log.Fatal(router.ListenAndServe())
}
