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
	"github.com/zatunohito/tarikihonganncalendar/internal/service/analysis"
	"github.com/zatunohito/tarikihonganncalendar/internal/service/print"
	"github.com/zatunohito/tarikihonganncalendar/internal/service/task"
	"github.com/zatunohito/tarikihonganncalendar/internal/service/upload"
	"github.com/zatunohito/tarikihonganncalendar/internal/storage"
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

	taskRepo := postgres.NewTaskRepository(pool)
	taskSvc := task.NewService(taskRepo)
	taskH := handler.NewTaskHandler(taskSvc)

	storageClient, err := storage.NewMinioClient(storage.MinioConfig{
		Endpoint:  cfg.ObjectStorageEndpoint,
		AccessKey: cfg.ObjectStorageAccessKey,
		SecretKey: cfg.ObjectStorageSecretKey,
		Bucket:    cfg.ObjectStorageBucket,
		Region:    cfg.ObjectStorageRegion,
	})
	if err != nil {
		slog.Error("failed to create storage client", "error", err)
		os.Exit(1)
	}

	printRepo := postgres.NewPrintRepository(pool)
	uploadSvc := upload.NewService(printRepo, storageClient, upload.Config{
		PresignedURLTTL: cfg.PresignedURLTTL,
		MaxUploadBytes:  cfg.MaxUploadBytes,
	})
	uploadH := handler.NewUploadHandler(uploadSvc)

	printSvc := print.NewService(printRepo, storageClient)
	printH := handler.NewPrintHandler(printSvc)

	analysisJobRepo := postgres.NewAnalysisJobRepository(pool)
	analysisResultRepo := postgres.NewAnalysisResultRepository(pool)
	analysisSvc := analysis.NewService(analysisJobRepo, analysisResultRepo, printRepo)
	analysisH := handler.NewAnalysisHandler(analysisSvc)

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
			r.With(middleware.RequireAuth(sessionRepo)).Get("/me", authH.Me)
		})

		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireAuth(sessionRepo))
			r.Get("/tasks", taskH.List)
			r.Post("/tasks", taskH.Create)
			r.Get("/tasks/{taskId}", taskH.Get)
			r.Patch("/tasks/{taskId}", taskH.Update)
			r.Delete("/tasks/{taskId}", taskH.Delete)
			r.Post("/uploads", uploadH.Create)
			r.Get("/prints", printH.List)
			r.Get("/prints/{printId}", printH.Get)
			r.Delete("/prints/{printId}", printH.Delete)
			r.Post("/analysis-jobs", analysisH.Start)
			r.Get("/analysis-jobs/{jobId}", analysisH.Get)
			r.Post("/analysis-jobs/{jobId}/retry", analysisH.Retry)
			r.Post("/analysis-jobs/{jobId}/commit", analysisH.Commit)
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
