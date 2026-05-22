package handler

import (
	"context"
	"log"
	"net/http"
	"os"
	"sync"

	"agro-cost-app/pkg/config"
	"agro-cost-app/pkg/db"
	"agro-cost-app/pkg/handlers"
	"agro-cost-app/pkg/migrations"
	"agro-cost-app/pkg/repository"
	"agro-cost-app/pkg/services"
)

var (
	once    sync.Once
	mux     *http.ServeMux
	initErr error
)

func initApp() {
	cfg := config.Load()

	database, err := db.Connect(cfg.DatabaseURL)
	if err != nil {
		initErr = err
		return
	}

	if cfg.RunMigrations {
		if err := migrations.Run(database, ""); err != nil {
			log.Printf("database migration failed: %v", err)
			if os.Getenv("MIGRATIONS_FATAL") == "true" {
				initErr = err
				return
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

	m := http.NewServeMux()
	h.Register(m)
	mux = m
}

func Handler(w http.ResponseWriter, r *http.Request) {
	once.Do(initApp)
	if initErr != nil {
		log.Printf("application init failed: %v", initErr)
		http.Error(w, "server initialization failed: "+initErr.Error(), http.StatusInternalServerError)
		return
	}
	mux.ServeHTTP(w, r)
}
