package main

import (
	"context"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/schererja/yggdrasil/internal/auth"
	"github.com/schererja/yggdrasil/internal/shared"
)

func main() {
	cfg := shared.LoadConfig()

	db, err := shared.ConnectDB(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("connect db: %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Printf("close db: %v", err)
		}
	}()

	authSvc := auth.NewService(db, cfg.JWTSecret, cfg.JWTExpiry, cfg.RefreshTokenExpiry)

	if err := authSvc.SyncPermissions(context.Background()); err != nil {
		log.Fatalf("sync permissions: %v", err)
	}

	authHandler := auth.NewHandler(authSvc)
	requireAuth := auth.RequireAuthMiddleware(authSvc.JWTManager())

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	r.Route("/api/v1/auth", func(r chi.Router) {
		r.Post("/login", authHandler.Login)
		r.Post("/logout", authHandler.Logout)
		r.Post("/refresh", authHandler.Refresh)

		r.Group(func(r chi.Router) {
			r.Use(requireAuth)
			r.Get("/me", authHandler.Me)
			r.Get("/roles", authHandler.ListRoles)
			r.With(auth.RequirePermission(authSvc, "role:manage")).Post("/roles", authHandler.CreateRole)
		})
	})

	log.Printf("starting on %s", cfg.APIHost)
	if err := http.ListenAndServe(cfg.APIHost, r); err != nil {
		log.Fatalf("server: %v", err)
	}
}
