package main

import (
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/schererja/yggdrasil/internal/shared"
)

func main() {
	cfg := shared.LoadConfig()

	db, err := shared.ConnectDB(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("connect db: %v", err)
	}
	defer db.Close()

	r := chi.NewRouter()
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	log.Printf("starting on %s", cfg.APIHost)
	if err := http.ListenAndServe(cfg.APIHost, r); err != nil {
		log.Fatalf("server: %v", err)
	}
}
