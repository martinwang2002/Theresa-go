package config

import (
	"fmt"

	_ "github.com/joho/godotenv/autoload"
	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	// DevMode to indicate development mode. When true, the program would spin up utilities for debugging and
	// provide a more contextual message when encountered a panic. See internal/server/httpserver/http.go for the
	// actual implementation details.
	DevMode bool `split_words:"true"`
}

func Parse() (*Config, error) {
	var config Config
	err := envconfig.Process("theresa_go", &config)
	if err != nil {
		_ = envconfig.Usage("theresa_go", &config)
		return nil, fmt.Errorf("failed to parse configuration: %w", err)
	}

	return &config, nil
}
