package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/animus-labs/animus-go/closed/internal/platform/env"
	_ "github.com/jackc/pgx/v5/stdlib"
)

type Config struct {
	URL             string
	PingTimeout     time.Duration
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

func ConfigFromEnv() (Config, error) {
	pingTimeout, err := env.Duration("DATABASE_PING_TIMEOUT", 2*time.Second)
	if err != nil {
		return Config{}, err
	}

	maxOpenConns, err := env.Int("DATABASE_MAX_OPEN_CONNS", 10)
	if err != nil {
		return Config{}, err
	}
	maxIdleConns, err := env.Int("DATABASE_MAX_IDLE_CONNS", 5)
	if err != nil {
		return Config{}, err
	}
	connMaxLifetime, err := env.Duration("DATABASE_CONN_MAX_LIFETIME", 30*time.Minute)
	if err != nil {
		return Config{}, err
	}
	connMaxIdleTime, err := env.Duration("DATABASE_CONN_MAX_IDLE_TIME", 5*time.Minute)
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		URL:             env.String("DATABASE_URL", "postgres://animus:animus@localhost:5432/animus?sslmode=disable"),
		PingTimeout:     pingTimeout,
		MaxOpenConns:    maxOpenConns,
		MaxIdleConns:    maxIdleConns,
		ConnMaxLifetime: connMaxLifetime,
		ConnMaxIdleTime: connMaxIdleTime,
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) Validate() error {
	if c.URL == "" {
		return errors.New("DATABASE_URL is required")
	}
	if c.PingTimeout <= 0 {
		return errors.New("DATABASE_PING_TIMEOUT must be positive")
	}
	if c.MaxOpenConns < 1 {
		return errors.New("DATABASE_MAX_OPEN_CONNS must be >= 1")
	}
	if c.MaxIdleConns < 0 {
		return errors.New("DATABASE_MAX_IDLE_CONNS must be >= 0")
	}
	if c.MaxIdleConns > c.MaxOpenConns {
		return errors.New("DATABASE_MAX_IDLE_CONNS must be <= DATABASE_MAX_OPEN_CONNS")
	}
	if c.ConnMaxLifetime < 0 {
		return errors.New("DATABASE_CONN_MAX_LIFETIME must be >= 0")
	}
	if c.ConnMaxIdleTime < 0 {
		return errors.New("DATABASE_CONN_MAX_IDLE_TIME must be >= 0")
	}
	return nil
}

func Open(ctx context.Context, cfg Config) (*sql.DB, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	db, err := sql.Open("pgx", cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("open: %w", err)
	}

	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	db.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)

	pingCtx, cancel := context.WithTimeout(ctx, cfg.PingTimeout)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}

	return db, nil
}
