package migrations

import (
	"database/sql"
	_ "embed"
	"fmt"
)

//go:embed 001_init.sql
var initSQL string

func Run(db *sql.DB, _ string) error {
	if _, err := db.Exec(initSQL); err != nil {
		return fmt.Errorf("execute embedded migration: %w", err)
	}
	return nil
}
