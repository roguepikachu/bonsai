// Package config provides configuration loading and management for the Bonsai application.
package config

import (
	"context"
	"os"
	"strings"

	"github.com/caarlos0/env"
	"github.com/joho/godotenv"
	"github.com/roguepikachu/bonsai/pkg/logger"
)

// Config holds environment configuration for the Bonsai application.
type Config struct {
	// BonsaiPort is the port on which the Bonsai server runs.
	BonsaiPort string `env:"BONSAI_PORT"`
}

// Conf holds the global configuration for the Bonsai application.
var Conf Config

func loadDotEnv() {
	// Load .env file at the root of the project into environment if present.
	// Does not override existing environ variable
	path := os.Getenv("DOTENV_PATHS")
	if path != "" {
		err := godotenv.Load(strings.Split(path, ",")...)
		if err != nil {
			logger.Fatal(context.Background(), err.Error())
		}
	}
}

// InitConf initializes the global configuration by loading environment variables and .env files.
func InitConf() {
	loadDotEnv()

	if err := env.Parse(&Conf); err != nil {
		logger.Fatal(context.Background(), err.Error())
	}
}
