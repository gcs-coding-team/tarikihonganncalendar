package service

import (
	"log"
	"time"

	"github.com/gcs-coding-team/tarikihonganncalendar/internal/repository"
)

// jst is used to decide which calendar day "tomorrow" is, independent of the
// server's system timezone. The container sets TZ=Asia/Tokyo, but the reminder
// job should not depend on that holding true everywhere it might run.
var jst = mustLoadJST()

func mustLoadJST() *time.Location {
	loc, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		// No tzdata available. UTC+9 has no DST, so a fixed offset is exact, not
		// an approximation.
		return time.FixedZone("JST", 9*60*60)
	}
	return loc
}

// ReminderService emails whoever owns a task the day before it is due. It runs
// as a periodic sweep rather than scheduling per-task, since a task's due date
// can change and a timer set for the old one would fire at the wrong time — or
// not get cancelled at all.
type ReminderService struct {
	repo    repository.Repository
	deliver Delivery
}

func NewReminderService(repo repository.Repository, deliver Delivery) *ReminderService {
	if deliver == nil {
		deliver = LogDelivery{}
	}
	return &ReminderService{repo: repo, deliver: deliver}
}

// Run sends one round of reminders for tasks due tomorrow (JST) and returns
// how many were sent. A failure on one task — a deleted user, a mailer
// hiccup — is logged and skipped rather than aborting the sweep; the rest of
// the day's reminders still deserve to go out.
func (s *ReminderService) Run() int {
	tomorrow := time.Now().In(jst).AddDate(0, 0, 1).Format("2006-01-02")
	tasks, err := s.repo.ListTasksDueUnreminded(tomorrow)
	if err != nil {
		log.Printf("reminder sweep: list tasks: %v", err)
		return 0
	}
	sent := 0
	for _, task := range tasks {
		user, err := s.repo.GetUserByID(task.UserID)
		if err != nil {
			log.Printf("reminder sweep: task %s: look up user: %v", task.ID, err)
			continue
		}
		if err := s.deliver.SendTaskReminder(user.Email, task.Title, task.DueAt); err != nil {
			log.Printf("reminder sweep: task %s: send: %v", task.ID, err)
			continue
		}
		if err := s.repo.MarkTaskReminded(task.ID, tomorrow); err != nil {
			log.Printf("reminder sweep: task %s: mark reminded: %v", task.ID, err)
			continue
		}
		sent++
	}
	return sent
}

// RunEvery starts Run on a fixed interval and blocks — call it in its own
// goroutine. It runs once immediately so a restart does not wait a full
// interval before the first sweep.
func (s *ReminderService) RunEvery(interval time.Duration, stop <-chan struct{}) {
	s.Run()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			s.Run()
		case <-stop:
			return
		}
	}
}
