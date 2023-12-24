package app

import "final-project/internal/httpadapter"

type Config struct {
	HTTP httpadapter.Config `yaml:"http"`
}

func NewConfig() (*Config, error) {
	return &Config{
		HTTP: httpadapter.Config{
			ServeAddress: ":8080",
			BasePath:     "/",
		},
	}, nil
}
