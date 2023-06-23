package config

import (
	"fmt"

	_ "github.com/joho/godotenv/autoload"
	"github.com/kelseyhightower/envconfig"
)

// Note that all configs should have prefix THERESA_GO_
// e.g. THERESA_GO_DEV_MODE
type Config struct {
	// DevMode to indicate development mode. When true, the program would spin up utilities for debugging and
	// provide a more contextual message when encountered a panic. See internal/server/httpserver/http.go for the
	// actual implementation details.
	DevMode bool `split_words:"true"`

	// redis connection url
	RedisDsn string `required:"true" split_words:"true" default:"redis://127.0.0.1:6379/1"`

	// use gamedata from github repo
	UseGithubGamedata  bool   `split_words:"true"`
	GithubToken        string `split_words:"true"`
	GithubGamedataRepo string `split_words:"true"`

	// ak ab fs remote name
	AkAbFsRemoteName string `split_words:"true" default:"remote:"`
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
