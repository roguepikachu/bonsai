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
	// RedisPort is the port on which the Redis server runs.
	RedisPort string `env:"REDIS_PORT"`
	// PostgresURL is the full DSN for connecting to Postgres. If provided, it will be used as-is.
	PostgresURL string `env:"POSTGRES_URL"`
	// PostgresHost is the hostname for Postgres (used if PostgresURL is empty).
	PostgresHost string `env:"POSTGRES_HOST"`
	// PostgresPort is the port for Postgres (used if PostgresURL is empty).
	PostgresPort string `env:"POSTGRES_PORT"`
	// PostgresUser is the username for Postgres (used if PostgresURL is empty).
	PostgresUser string `env:"POSTGRES_USER"`
	// PostgresPassword is the password for Postgres (used if PostgresURL is empty).
	PostgresPassword string `env:"POSTGRES_PASSWORD"`
	// PostgresDB is the database name for Postgres (used if PostgresURL is empty).
	PostgresDB string `env:"POSTGRES_DB"`
	// PostgresSSLMode controls the sslmode parameter when building a DSN (disable, require, verify-ca, verify-full).
	PostgresSSLMode string `env:"POSTGRES_SSLMODE"`
	// AutoMigrate, if true, will run light schema migrations on startup.
	AutoMigrate bool `env:"AUTO_MIGRATE"`
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
