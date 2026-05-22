package config

import "os"

type Config struct {
	Addr          string
	DatabaseURL   string
	StaticDir     string
	RunMigrations bool
	AuthRequired  bool
	SessionSecret string
}

func Load() Config {
	port := getenv("PORT", "8080")
	addr := getenv("APP_ADDR", ":"+port)
	dbURL := getenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/agro_cost?sslmode=disable")
	staticDir := getenv("STATIC_DIR", "web")
	return Config{
		Addr:          addr,
		DatabaseURL:   dbURL,
		StaticDir:     staticDir,
		RunMigrations: getenv("RUN_MIGRATIONS", "false") == "true",
		AuthRequired:  getenv("AUTH_REQUIRED", "false") == "true",
		SessionSecret: getenv("SESSION_SECRET", "dev-secret-change-me"),
	}
}

func getenv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
