package config

import "github.com/Rachit-Gandhi/go-router/internal/database"

type Config struct {
	Host string
	Port int
	Db   *database.Queries
}

type ControlConfig struct {
	Config
}

type RouterConfig struct {
	Config
}
