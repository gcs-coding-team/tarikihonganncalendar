package service

import (
	"time"

	"github.com/gcs-coding-team/tarikihonganncalendar/internal/repository"
)

type EventService struct {
	repo repository.EventRepository
}

type CreateEventInput struct {
	Title       string
	Description string
	StartAt     time.Time
	EndAt       time.Time
	AllDay      bool
	Repeat      *repository.Repeat
	ExDates     []string
}

type UpdateEventInput struct {
	Title       *string
	Description *string
	StartAt     *time.Time
	EndAt       *time.Time
	AllDay      *bool
	// Repeat is a double pointer so the three cases stay distinct: leave the
	// rule alone, replace it, or clear it by sending null.
	Repeat  **repository.Repeat
	ExDates *[]string
	Version int
}

func NewEventService(repo repository.EventRepository) *EventService {
	return &EventService{repo: repo}
}

func (s *EventService) Create(userID string, input CreateEventInput) (repository.Event, error) {
	if userID == "" {
		return repository.Event{}, repository.ErrForbidden
	}
	if input.Title == "" {
		return repository.Event{}, repository.ValidationError("title is required")
	}
	if err := validateRepeat(input.Repeat); err != nil {
		return repository.Event{}, err
	}
	event := repository.Event{
		UserID:      userID,
		Title:       input.Title,
		Description: input.Description,
		StartAt:     input.StartAt,
		EndAt:       input.EndAt,
		AllDay:      input.AllDay,
		Repeat:      input.Repeat,
		ExDates:     input.ExDates,
	}
	return s.repo.CreateEvent(event)
}

func (s *EventService) List(userID, cursor string, limit int) ([]repository.Event, error) {
	return s.repo.ListEvents(userID, cursor, limit)
}

func (s *EventService) Get(userID, eventID string) (repository.Event, error) {
	return s.repo.GetEvent(userID, eventID)
}

func (s *EventService) Update(userID, eventID string, input UpdateEventInput) (repository.Event, error) {
	existing, err := s.repo.GetEvent(userID, eventID)
	if err != nil {
		return repository.Event{}, err
	}
	if input.Version != 0 && input.Version != existing.Version {
		return repository.Event{}, repository.ErrConflict
	}
	if input.Title != nil {
		existing.Title = *input.Title
	}
	if input.Description != nil {
		existing.Description = *input.Description
	}
	if input.StartAt != nil {
		existing.StartAt = *input.StartAt
	}
	if input.EndAt != nil {
		existing.EndAt = *input.EndAt
	}
	if input.AllDay != nil {
		existing.AllDay = *input.AllDay
	}
	if input.Repeat != nil {
		if err := validateRepeat(*input.Repeat); err != nil {
			return repository.Event{}, err
		}
		existing.Repeat = *input.Repeat
	}
	if input.ExDates != nil {
		existing.ExDates = *input.ExDates
	}
	return s.repo.UpdateEvent(existing)
}

func (s *EventService) Delete(userID, eventID string) error {
	return s.repo.DeleteEvent(userID, eventID)
}

// validateRepeat rejects rules the expander would not know what to do with.
func validateRepeat(r *repository.Repeat) error {
	if r == nil {
		return nil
	}
	switch r.Freq {
	case "daily", "weekly", "monthly":
	default:
		return repository.ValidationError("repeat.freq must be daily, weekly or monthly")
	}
	if r.Until != "" {
		if _, err := time.Parse("2006-01-02", r.Until); err != nil {
			return repository.ValidationError("repeat.until must be YYYY-MM-DD")
		}
	}
	return nil
}
