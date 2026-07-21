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
}

type UpdateEventInput struct {
	Title       *string
	Description *string
	StartAt     *time.Time
	EndAt       *time.Time
	AllDay      *bool
	Version     int
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
	event := repository.Event{
		UserID:      userID,
		Title:       input.Title,
		Description: input.Description,
		StartAt:     input.StartAt,
		EndAt:       input.EndAt,
		AllDay:      input.AllDay,
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
	return s.repo.UpdateEvent(existing)
}

func (s *EventService) Delete(userID, eventID string) error {
	return s.repo.DeleteEvent(userID, eventID)
}
