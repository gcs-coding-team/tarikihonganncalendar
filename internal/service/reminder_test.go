package service

import (
	"testing"
	"time"

	"github.com/gcs-coding-team/tarikihonganncalendar/internal/repository"
)

// captureReminderDelivery records every reminder sent, so a test can assert on
// who was mailed and about which task without a real mailer.
type captureReminderDelivery struct {
	sent []struct{ email, title, dueAt string }
}

func (d *captureReminderDelivery) SendPasswordReset(email, token string) error { return nil }

func (d *captureReminderDelivery) SendTaskReminder(email, title, dueAt string) error {
	d.sent = append(d.sent, struct{ email, title, dueAt string }{email, title, dueAt})
	return nil
}

func TestReminderServiceSendsOnlyForTomorrowsOpenTasks(t *testing.T) {
	repo := repository.NewMemoryRepository()
	user, _, err := NewAccountService(repo).Register(Credentials{
		Email: "student@example.jp", Password: "password123", DisplayName: "学生",
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	tomorrow := time.Now().In(jst).AddDate(0, 0, 1).Format("2006-01-02")
	dayAfter := time.Now().In(jst).AddDate(0, 0, 2).Format("2006-01-02")

	due, err := repo.CreateTask(repository.Task{UserID: user.ID, Title: "レポート提出", DueAt: tomorrow})
	if err != nil {
		t.Fatalf("CreateTask (due tomorrow): %v", err)
	}
	if _, err := repo.CreateTask(repository.Task{UserID: user.ID, Title: "まだ先", DueAt: dayAfter}); err != nil {
		t.Fatalf("CreateTask (due later): %v", err)
	}
	doneTask, err := repo.CreateTask(repository.Task{UserID: user.ID, Title: "終わった課題", DueAt: tomorrow})
	if err != nil {
		t.Fatalf("CreateTask (done): %v", err)
	}
	doneTask.Status = repository.TaskStatusDone
	if _, err := repo.UpdateTask(doneTask); err != nil {
		t.Fatalf("UpdateTask (done): %v", err)
	}

	deliver := &captureReminderDelivery{}
	svc := NewReminderService(repo, deliver)

	if sent := svc.Run(); sent != 1 {
		t.Fatalf("expected 1 reminder sent, got %d", sent)
	}
	if len(deliver.sent) != 1 {
		t.Fatalf("expected 1 delivery, got %d", len(deliver.sent))
	}
	got := deliver.sent[0]
	if got.email != "student@example.jp" || got.title != "レポート提出" || got.dueAt != tomorrow {
		t.Fatalf("unexpected reminder: %+v", got)
	}

	reminded, err := repo.GetTask(user.ID, due.ID)
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if reminded.RemindedAt != tomorrow {
		t.Fatalf("expected RemindedAt=%s, got %q", tomorrow, reminded.RemindedAt)
	}

	// A second sweep the same day must not resend.
	if sent := svc.Run(); sent != 0 {
		t.Fatalf("expected 0 reminders on second sweep, got %d", sent)
	}
	if len(deliver.sent) != 1 {
		t.Fatalf("expected still 1 delivery total, got %d", len(deliver.sent))
	}
}
