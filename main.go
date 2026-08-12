// Command tarikihonganncalendar serves the whole API.
//
// This is the single entry point. It used to be one of two — cmd/api ran a
// separate chi + Postgres server with its own half of the endpoints — and the
// two never agreed on which one was the real thing. Now there is one server and
// one set of routes, with the storage chosen at boot:
//
//	DATABASE_URL set   → Postgres, data survives a restart
//	DATABASE_URL unset → in-memory, everything is lost on exit
//
// The in-memory mode exists so the app can be run and demoed without a database.
// It is not a place to keep anything.
package main

import (
	"context"
	_ "embed"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gcs-coding-team/tarikihonganncalendar/internal/httpapi/handler"
	"github.com/gcs-coding-team/tarikihonganncalendar/internal/httpapi/middleware"
	"github.com/gcs-coding-team/tarikihonganncalendar/internal/repository"
	"github.com/gcs-coding-team/tarikihonganncalendar/internal/repository/pgstore"
	"github.com/gcs-coding-team/tarikihonganncalendar/internal/service/vision"
)

//go:embed migrations/0001_init.sql
var schema string

func main() {
	ctx := context.Background()

	repo, closeRepo := openStore(ctx)
	defer closeRepo()

	// Reading prints needs a vision model. Without one reachable, uploads still
	// work and analysis reports that it is unavailable rather than inventing
	// results — a wrong deadline is worse than a missing one.
	analyser := vision.New(vision.Config{
		BaseURL: getenv("OLLAMA_BASE_URL", "http://localhost:11434"),
		Model:   getenv("OLLAMA_MODEL", "gemma3:4b"),
		Timeout: time.Duration(getenvInt("OLLAMA_TIMEOUT_SECONDS", 180)) * time.Second,
	})

	var h http.Handler = handler.NewHandler(repo, handler.Options{Analyser: analyser})

	// FRONTEND_ORIGIN names the single origin allowed to call this API from a
	// browser, e.g. http://localhost:3000. Leave it unset when the frontend is
	// served from this same origin; then nothing cross-origin is permitted.
	h = middleware.CORS(os.Getenv("FRONTEND_ORIGIN"))(h)

	port := getenv("PORT", getenv("HTTP_PORT", "8080"))
	log.Printf("listening on :%s", port)
	if err := http.ListenAndServe(":"+port, h); err != nil {
		log.Fatal(err)
	}
}

func openStore(ctx context.Context) (repository.Repository, func()) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		log.Print("DATABASE_URL is unset — running in memory, nothing is kept across a restart")
		return repository.NewMemoryRepository(), func() {}
	}
	store, err := pgstore.New(ctx, url)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	if err := store.Migrate(ctx, schema); err != nil {
		log.Fatalf("migrate: %v", err)
	}
	log.Print("connected to postgres")
	return store, store.Close
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getenvInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n := 0
	for _, r := range v {
		if r < '0' || r > '9' {
			return fallback
		}
		n = n*10 + int(r-'0')
	}
	if n == 0 {
		return fallback
	}
	return n
}
