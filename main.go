package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strings"

	"agro-cost-app/pkg/config"
	"agro-cost-app/pkg/db"
	"agro-cost-app/pkg/handlers"
	"agro-cost-app/pkg/migrations"
	"agro-cost-app/pkg/repository"
	"agro-cost-app/pkg/services"
)

func main() {
	cfg := config.Load()

	database, err := db.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("connect database: %v", err)
	}

	if cfg.RunMigrations {
		if err := migrations.Run(database, ""); err != nil {
			log.Printf("database migration failed: %v", err)
			if os.Getenv("MIGRATIONS_FATAL") == "true" {
				log.Fatalf("run migrations: %v", err)
			}
		} else {
			log.Println("database migration completed")
		}
	}

	repo := repository.New(database)
	if err := repo.EnsureRuntimeSchema(context.Background()); err != nil {
		log.Printf("runtime schema check failed: %v", err)
	}
	calc := services.NewCalculationService(repo)
	parser := services.NewParserService(repo)
	auth := services.NewAuthService(repo, cfg.SessionSecret)
	h := handlers.New(repo, calc, parser, auth, cfg.AuthRequired)

	mux := http.NewServeMux()
	h.Register(mux)

	staticDir := strings.TrimSpace(os.Getenv("STATIC_DIR"))
	if staticDir == "" {
		staticDir = "public"
	}
	fileServer := http.FileServer(http.Dir(staticDir))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == "/healthz" {
			http.NotFound(w, r)
			return
		}
		if r.URL.Path == "/" {
			http.ServeFile(w, r, staticDir+"/index.html")
			return
		}
		fileServer.ServeHTTP(w, r)
	})

	port := strings.TrimSpace(os.Getenv("PORT"))
	if port == "" {
		port = "8080"
	}

	addr := ":" + port
	log.Printf("server started on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}
