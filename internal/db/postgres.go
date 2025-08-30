package db

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func NewPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse dsn: %w", err)
	}
	cfg.MinConns = 0
	cfg.MaxConns = 10
	cfg.HealthCheckPeriod = 30 * time.Second
	cfg.MaxConnLifetime = 30 * time.Minute
	cfg.MaxConnIdleTime = 5 * time.Minute

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return pgxpool.NewWithConfig(ctx, cfg)
}

func DSNFromEnv() string {
	if v := os.Getenv("DATABASE_URL"); v != "" {
		return v
	}
	host := getenv("PGHOST", "127.0.0.1")
	port := getenv("PGPORT", "5432")
	user := getenv("PGUSER", "postgres")
	pass := os.Getenv("PGPASSWORD")
	db := getenv("PGDATABASE", "postgres")
	sslmode := getenv("PGSSLMODE", "disable")

	u := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(user, pass),
		Host:   fmt.Sprintf("%s:%s", host, port),
		Path:   "/" + db,
	}
	q := u.Query()
	q.Set("sslmode", sslmode)
	u.RawQuery = q.Encode()
	return u.String()
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
