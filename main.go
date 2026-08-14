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
	"github.com/gcs-coding-team/tarikihonganncalendar/internal/service"
	"github.com/gcs-coding-team/tarikihonganncalendar/internal/service/vision"
	"github.com/gcs-coding-team/tarikihonganncalendar/internal/storage"
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

	// ALLOW_DEV_AUTH re-opens the shortcuts that predate passwords: a trusted
	// X-User-ID header and passwordless session creation. Convenient against a
	// local server, and a way straight past every password anywhere else.
	devAuth := os.Getenv("ALLOW_DEV_AUTH") == "true"
	if devAuth {
		log.Print("ALLOW_DEV_AUTH is on — X-User-ID is trusted and sessions can be created without a password")
	}

	delivery := openDelivery()

	var h http.Handler = handler.NewHandler(repo, handler.Options{
		Analyser:    analyser,
		Delivery:    delivery,
		Blobs:       openBlobs(),
		DevAuth:     devAuth,
		MaxAttempts: getenvInt("WORKER_MAX_ATTEMPTS", 3),
	})

	// Sweeps for tasks due tomorrow once an hour. Hourly rather than daily so a
	// task created or rescheduled onto tomorrow any time during the day still
	// gets its reminder — ListTasksDueUnreminded is idempotent per calendar day,
	// so the extra sweeps just find nothing new to send.
	reminders := service.NewReminderService(repo, delivery)
	go reminders.RunEvery(time.Hour, nil)

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

// openDelivery decides how a password reset or task reminder reaches its
// owner. Without SMTP configured, both go to the log instead of an inbox.
func openDelivery() service.Delivery {
	host := os.Getenv("SMTP_HOST")
	if host == "" {
		log.Print("SMTP_HOST is unset — password reset tokens and task reminders go to this log, not to anyone's inbox")
		return service.LogDelivery{}
	}
	from := getenv("SMTP_FROM", os.Getenv("SMTP_USERNAME"))
	if from == "" {
		log.Fatal("SMTP_HOST is set but SMTP_FROM is not — mail needs a sender")
	}
	log.Printf("sending password resets and task reminders through %s as %s", host, from)
	return service.SMTPDelivery{
		Host:     host,
		Port:     getenvInt("SMTP_PORT", 587),
		Username: os.Getenv("SMTP_USERNAME"),
		Password: os.Getenv("SMTP_PASSWORD"),
		From:     from,
		AppURL:   os.Getenv("APP_URL"),
	}
}

// openBlobs decides where photographed handouts are kept. BLOB_DIR is a
// directory on this machine — enough for a single server, and the difference
// between an app that can show you the handout it read and one that cannot.
// Set it empty to throw the images away after reading.
func openBlobs() storage.Blobs {
	dir, set := os.LookupEnv("BLOB_DIR")
	if !set {
		dir = "./data/prints"
	}
	if dir == "" {
		log.Print("BLOB_DIR is empty — handouts are read and then discarded")
		return storage.NewDiscardBlobs()
	}
	blobs, err := storage.NewFileBlobs(dir)
	if err != nil {
		log.Fatalf("blob storage: %v", err)
	}
	log.Printf("keeping handouts in %s", dir)
	return blobs
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
