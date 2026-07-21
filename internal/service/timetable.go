package service

import (
	"github.com/gcs-coding-team/tarikihonganncalendar/internal/repository"
)

type TimetableService struct {
	repo repository.TimetableRepository
}

type CreateTimetableEntryInput struct {
	DayOfWeek int
	Period    int
	Subject   string
	Room      string
	Teacher   string
}

type UpdateTimetableEntryInput struct {
	DayOfWeek *int
	Period    *int
	Subject   *string
	Room      *string
	Teacher   *string
	Version   int
}

func NewTimetableService(repo repository.TimetableRepository) *TimetableService {
	return &TimetableService{repo: repo}
}

func (s *TimetableService) Create(userID string, input CreateTimetableEntryInput) (repository.TimetableEntry, error) {
	entry := repository.TimetableEntry{
		UserID:    userID,
		DayOfWeek: input.DayOfWeek,
		Period:    input.Period,
		Subject:   input.Subject,
		Room:      input.Room,
		Teacher:   input.Teacher,
	}
	return s.repo.CreateTimetableEntry(entry)
}

func (s *TimetableService) List(userID string) ([]repository.TimetableEntry, error) {
	return s.repo.ListTimetableEntries(userID)
}

func (s *TimetableService) Get(userID, entryID string) (repository.TimetableEntry, error) {
	return s.repo.GetTimetableEntry(userID, entryID)
}

func (s *TimetableService) Update(userID, entryID string, input UpdateTimetableEntryInput) (repository.TimetableEntry, error) {
	existing, err := s.repo.GetTimetableEntry(userID, entryID)
	if err != nil {
		return repository.TimetableEntry{}, err
	}
	if input.Version != 0 && input.Version != existing.Version {
		return repository.TimetableEntry{}, repository.ErrConflict
	}
	if input.DayOfWeek != nil {
		existing.DayOfWeek = *input.DayOfWeek
	}
	if input.Period != nil {
		existing.Period = *input.Period
	}
	if input.Subject != nil {
		existing.Subject = *input.Subject
	}
	if input.Room != nil {
		existing.Room = *input.Room
	}
	if input.Teacher != nil {
		existing.Teacher = *input.Teacher
	}
	return s.repo.UpdateTimetableEntry(existing)
}

func (s *TimetableService) Delete(userID, entryID string) error {
	return s.repo.DeleteTimetableEntry(userID, entryID)
}
