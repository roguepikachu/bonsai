// Package data provides low-level data clients and connection factories.
package data

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/roguepikachu/bonsai/internal/config"
	"github.com/roguepikachu/bonsai/pkg/logger"
)

// NewPostgresPool creates a new pgx connection pool based on environment configuration.
func NewPostgresPool(ctx context.Context) (*pgxpool.Pool, error) {
	if dsn := config.Conf.PostgresURL; dsn != "" {
		logger.Info(ctx, "initializing postgres pool via DSN (host masked)")
		cfg, err := pgxpool.ParseConfig(dsn)
		if err != nil {
			return nil, err
		}
		cfg.MaxConnIdleTime = 30 * time.Second
		cfg.MaxConnLifetime = 30 * time.Minute
		return pgxpool.NewWithConfig(ctx, cfg)
	}
	host := config.Conf.PostgresHost
	if host == "" {
		host = "127.0.0.1"
	}
	port := config.Conf.PostgresPort
	if port == "" {
		port = "5432"
	}
	user := config.Conf.PostgresUser
	if user == "" {
		user = "postgres"
	}
	pass := config.Conf.PostgresPassword
	db := config.Conf.PostgresDB
	if db == "" {
		db = "bonsai"
	}
	sslmode := config.Conf.PostgresSSLMode
	if sslmode == "" {
		sslmode = "disable"
	}
	logger.With(ctx, map[string]any{"host": host, "port": port, "db": db, "sslmode": sslmode}).Info("initializing postgres pool")
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s", user, pass, host, port, db, sslmode)
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}
	cfg.MaxConnIdleTime = 30 * time.Second
	cfg.MaxConnLifetime = 30 * time.Minute
	return pgxpool.NewWithConfig(ctx, cfg)
}
