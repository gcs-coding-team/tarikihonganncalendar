package main

import (
	"log"
	"net/http"
	"os"

	"github.com/gcs-coding-team/tarikihonganncalendar/internal/httpapi/handler"
	"github.com/gcs-coding-team/tarikihonganncalendar/internal/httpapi/middleware"
	"github.com/gcs-coding-team/tarikihonganncalendar/internal/repository"
)

func main() {
	repo := repository.NewMemoryRepository()

	// FRONTEND_ORIGIN names the single origin allowed to call this API from a
	// browser, e.g. http://localhost:3000. Leave it unset when the frontend is
	// served from this same origin; then nothing cross-origin is permitted.
	var h http.Handler = handler.NewHandler(repo)
	h = middleware.CORS(os.Getenv("FRONTEND_ORIGIN"))(h)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("listening on :%s", port)
	if err := http.ListenAndServe(":"+port, h); err != nil {
		log.Fatal(err)
	}
}
