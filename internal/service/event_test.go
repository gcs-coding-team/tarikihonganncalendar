package service

import (
	"testing"
	"time"

	"github.com/gcs-coding-team/tarikihonganncalendar/internal/repository"
)

func TestCreateAndListEvents(t *testing.T) {
	repo := repository.NewMemoryRepository()
	svc := NewEventService(repo)

	created, err := svc.Create("user-1", CreateEventInput{
		Title:       "学校行事",
		Description: "体育館集合",
		StartAt:     time.Date(2026, 7, 25, 9, 0, 0, 0, time.UTC),
		EndAt:       time.Date(2026, 7, 25, 12, 0, 0, 0, time.UTC),
		AllDay:      false,
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if created.Title != "学校行事" {
		t.Fatalf("unexpected title: %s", created.Title)
	}

	items, err := svc.List("user-1", "", 20)
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 event, got %d", len(items))
	}
}
