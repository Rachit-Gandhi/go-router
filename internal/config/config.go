package config

type Config struct {
	Host string
	Port int
}

type ControlConfig struct {
	Config
}

type RouterConfig struct {
	Config
}
