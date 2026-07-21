package service

import (
	"testing"

	"github.com/gcs-coding-team/tarikihonganncalendar/internal/repository"
)

func TestCreateSharedItemRejectsDuplicate(t *testing.T) {
	repo := repository.NewMemoryRepository()
	svc := NewColonyService(repo)

	colony, err := svc.Create("user-1", CreateColonyInput{Name: "3年1組", Description: "クラス共有"})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	_, err = svc.CreateSharedItem("user-1", colony.ID, CreateSharedItemInput{SourceType: "TASK", SourceID: "task-1"})
	if err != nil {
		t.Fatalf("first shared-item create returned error: %v", err)
	}

	_, err = svc.CreateSharedItem("user-1", colony.ID, CreateSharedItemInput{SourceType: "TASK", SourceID: "task-1"})
	if err == nil {
		t.Fatal("expected duplicate shared item error")
	}
}
