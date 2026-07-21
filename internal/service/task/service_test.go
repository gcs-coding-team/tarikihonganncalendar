package task

import (
	"context"
	"testing"
	"time"

	"github.com/zatunohito/tarikihonganncalendar/internal/domain"
	"github.com/zatunohito/tarikihonganncalendar/internal/repository"
)

type mockTaskRepo struct {
	tasks map[string]*domain.Task
}

func (m *mockTaskRepo) Create(_ context.Context, t *domain.Task) error {
	m.tasks[t.ID] = t
	return nil
}

func (m *mockTaskRepo) FindByID(_ context.Context, id string) (*domain.Task, error) {
	return m.tasks[id], nil
}

func (m *mockTaskRepo) FindByUserID(_ context.Context, userID string, filter repository.ListTasksFilter) ([]*domain.Task, string, error) {
	var result []*domain.Task
	for _, t := range m.tasks {
		if t.UserID == userID {
			result = append(result, t)
		}
	}
	return result, "", nil
}

func (m *mockTaskRepo) Update(_ context.Context, t *domain.Task) error {
	m.tasks[t.ID] = t
	return nil
}

func (m *mockTaskRepo) Delete(_ context.Context, id string) error {
	delete(m.tasks, id)
	return nil
}

func newMockTaskService() *Service {
	return NewService(&mockTaskRepo{tasks: map[string]*domain.Task{}})
}

func TestCreateTask(t *testing.T) {
	svc := newMockTaskService()
	dueAt := time.Date(2026, 7, 30, 10, 0, 0, 0, time.UTC)

	task, err := svc.Create(context.Background(), CreateInput{
		UserID:      "user-1",
		Title:       "Test Task",
		Description: "Description",
		DueAt:       &dueAt,
	})
	if err != nil {
		t.Fatal(err)
	}
	if task.ID == "" {
		t.Fatal("expected non-empty ID")
	}
	if task.UserID != "user-1" {
		t.Fatal("user ID mismatch")
	}
	if task.Status != domain.TaskStatusOpen {
		t.Fatal("expected OPEN status")
	}
	if task.Version != 1 {
		t.Fatal("expected version 1")
	}
	if task.Title != "Test Task" {
		t.Fatal("title mismatch")
	}
	if task.Description != "Description" {
		t.Fatal("description mismatch")
	}
	if task.DueAt == nil || !task.DueAt.Equal(dueAt) {
		t.Fatal("dueAt mismatch")
	}
}

func TestCreateTask_EmptyDueAt(t *testing.T) {
	svc := newMockTaskService()
	task, err := svc.Create(context.Background(), CreateInput{
		UserID:      "user-1",
		Title:       "No Due",
		Description: "",
		DueAt:       nil,
	})
	if err != nil {
		t.Fatal(err)
	}
	if task.DueAt != nil {
		t.Fatal("expected nil dueAt")
	}
}

func TestGetByID_Success(t *testing.T) {
	svc := newMockTaskService()
	created, _ := svc.Create(context.Background(), CreateInput{
		UserID: "user-1", Title: "Get Test", Description: "",
	})

	found, err := svc.GetByID(context.Background(), created.ID, "user-1")
	if err != nil {
		t.Fatal(err)
	}
	if found.ID != created.ID {
		t.Fatal("ID mismatch")
	}
}

func TestGetByID_NotFound(t *testing.T) {
	svc := newMockTaskService()
	_, err := svc.GetByID(context.Background(), "non-existent", "user-1")
	if err != ErrNotFound {
		t.Fatal("expected ErrNotFound")
	}
}

func TestGetByID_WrongOwner(t *testing.T) {
	svc := newMockTaskService()
	created, _ := svc.Create(context.Background(), CreateInput{
		UserID: "user-1", Title: "Owned", Description: "",
	})

	_, err := svc.GetByID(context.Background(), created.ID, "user-2")
	if err != ErrNotFound {
		t.Fatal("expected ErrNotFound for wrong owner")
	}
}

func TestList(t *testing.T) {
	svc := newMockTaskService()
	svc.Create(context.Background(), CreateInput{UserID: "user-1", Title: "T1"})
	svc.Create(context.Background(), CreateInput{UserID: "user-1", Title: "T2"})
	svc.Create(context.Background(), CreateInput{UserID: "user-2", Title: "T3"})

	tasks, _, err := svc.List(context.Background(), "user-1", repository.ListTasksFilter{Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(tasks))
	}
}

func TestUpdate_Success(t *testing.T) {
	svc := newMockTaskService()
	created, _ := svc.Create(context.Background(), CreateInput{
		UserID: "user-1", Title: "Original", Description: "original desc",
	})

	updated, err := svc.Update(context.Background(), UpdateInput{
		ID:          created.ID,
		UserID:      "user-1",
		Title:       "Updated",
		Description: "updated desc",
		Status:      domain.TaskStatusDone,
		Version:     1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Title != "Updated" {
		t.Fatal("title not updated")
	}
	if updated.Status != domain.TaskStatusDone {
		t.Fatal("status not updated")
	}
}

func TestUpdate_VersionConflict(t *testing.T) {
	svc := newMockTaskService()
	created, _ := svc.Create(context.Background(), CreateInput{
		UserID: "user-1", Title: "Conflict", Description: "",
	})

	_, err := svc.Update(context.Background(), UpdateInput{
		ID:      created.ID,
		UserID:  "user-1",
		Title:   "New",
		Status:  domain.TaskStatusOpen,
		Version: 999,
	})
	if err != ErrConflict {
		t.Fatal("expected ErrConflict for version mismatch")
	}
}

func TestUpdate_WrongOwner(t *testing.T) {
	svc := newMockTaskService()
	created, _ := svc.Create(context.Background(), CreateInput{
		UserID: "user-1", Title: "Owned", Description: "",
	})

	_, err := svc.Update(context.Background(), UpdateInput{
		ID:      created.ID,
		UserID:  "user-2",
		Title:   "Hacked",
		Status:  domain.TaskStatusOpen,
		Version: 1,
	})
	if err != ErrNotFound {
		t.Fatal("expected ErrNotFound for wrong owner")
	}
}

func TestDelete_Success(t *testing.T) {
	svc := newMockTaskService()
	created, _ := svc.Create(context.Background(), CreateInput{
		UserID: "user-1", Title: "To Delete", Description: "",
	})

	err := svc.Delete(context.Background(), created.ID, "user-1")
	if err != nil {
		t.Fatal(err)
	}

	_, err = svc.GetByID(context.Background(), created.ID, "user-1")
	if err != ErrNotFound {
		t.Fatal("expected ErrNotFound after delete")
	}
}

func TestDelete_WrongOwner(t *testing.T) {
	svc := newMockTaskService()
	created, _ := svc.Create(context.Background(), CreateInput{
		UserID: "user-1", Title: "Owned", Description: "",
	})

	err := svc.Delete(context.Background(), created.ID, "user-2")
	if err != ErrNotFound {
		t.Fatal("expected ErrNotFound for wrong owner")
	}
}
