package service

import (
	"testing"

	"github.com/gcs-coding-team/tarikihonganncalendar/internal/repository"
)

func TestAuthServiceCreatesSessionAndResolvesUserID(t *testing.T) {
	repo := repository.NewMemoryRepository()
	svc := NewAuthService(repo)

	session, err := svc.CreateSession("user-1", "Alice")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	if session.UserID != "user-1" {
		t.Fatalf("expected user id to be preserved, got %s", session.UserID)
	}
	if session.Token == "" {
		t.Fatal("expected a non-empty token")
	}

	resolved := svc.ResolveUserID("", "Bearer "+session.Token)
	if resolved != "user-1" {
		t.Fatalf("expected resolved user id to be user-1, got %s", resolved)
	}
}
