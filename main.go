package main

import (
	"log"
	"net/http"
	"os"

	"github.com/gcs-coding-team/tarikihonganncalendar/internal/httpapi/handler"
	"github.com/gcs-coding-team/tarikihonganncalendar/internal/repository"
)

func main() {
	repo := repository.NewMemoryRepository()
	mux := handler.NewHandler(repo)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("listening on :%s", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatal(err)
	}
}
