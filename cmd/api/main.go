package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/zatunohito/tarikihonganncalendar/internal/config"
	"github.com/zatunohito/tarikihonganncalendar/internal/httpapi/handler"
	"github.com/zatunohito/tarikihonganncalendar/internal/httpapi/middleware"
	"github.com/zatunohito/tarikihonganncalendar/internal/httpapi/response"
	"github.com/zatunohito/tarikihonganncalendar/internal/repository/postgres"
	"github.com/zatunohito/tarikihonganncalendar/internal/service/auth"
)

func main() {
	cfg := config.Load()

	pool, err := pgxpool.New(context.Background(), cfg.DatabaseURL)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	userRepo := postgres.NewUserRepository(pool)
	sessionRepo := postgres.NewSessionRepository(pool)

	authSvc := auth.NewService(userRepo, sessionRepo)
	authH := handler.NewAuthHandler(authSvc)

	r := chi.NewRouter()

	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(middleware.Logger)
	r.Use(middleware.CORS(cfg.FrontendOrigin))

	r.Get("/healthz", healthz)
	r.Get("/readyz", readyz)

	r.Route("/v1", func(r chi.Router) {
		r.Route("/auth", func(r chi.Router) {
			r.Post("/register", authH.Register)
			r.Post("/login", authH.Login)
			r.Post("/logout", authH.Logout)
		})
	})

	srv := &http.Server{
		Addr:         ":" + cfg.HTTPPort,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		slog.Info("starting server", "port", cfg.HTTPPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down server")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("shutdown error", "error", err)
		os.Exit(1)
	}
	slog.Info("server stopped")
}

func healthz(w http.ResponseWriter, r *http.Request) {
	response.OK(w, middleware.GetRequestID(r.Context()), map[string]string{"status": "ok"})
}

func readyz(w http.ResponseWriter, r *http.Request) {
	response.OK(w, middleware.GetRequestID(r.Context()), map[string]string{"status": "ready"})
}
