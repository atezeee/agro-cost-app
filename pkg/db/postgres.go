package db

import (
	"database/sql"
	"fmt"
	"net/url"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func Connect(databaseURL string) (*sql.DB, error) {
	dsn := forceSimpleProtocol(databaseURL)
	database, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	database.SetMaxOpenConns(10)
	database.SetMaxIdleConns(5)
	database.SetConnMaxLifetime(30 * time.Minute)

	if err := database.Ping(); err != nil {
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return database, nil
}

func forceSimpleProtocol(databaseURL string) string {
	parsed, err := url.Parse(databaseURL)
	if err != nil || parsed.Scheme == "" {
		return databaseURL
	}
	q := parsed.Query()
	if q.Get("default_query_exec_mode") == "" {
		q.Set("default_query_exec_mode", "simple_protocol")
	}
	parsed.RawQuery = q.Encode()
	return parsed.String()
}
