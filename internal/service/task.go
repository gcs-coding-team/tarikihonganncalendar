package service

import (
	"time"

	"github.com/gcs-coding-team/tarikihonganncalendar/internal/repository"
)

type TaskService struct {
	repo repository.Repository
}

type CreateTaskInput struct {
	Title       string
	Description string
	DueAt       string
	Status      string
	ProjectID   string
}

type UpdateTaskInput struct {
	Title       *string
	Description *string
	DueAt       *string
	Status      *string
	// ProjectID is a double pointer so clearing it (sending null, meaning
	// "unfile this task") stays distinct from leaving it where it is.
	ProjectID **string
	Version   int
}

func NewTaskService(repo repository.Repository) *TaskService {
	return &TaskService{repo: repo}
}

func (s *TaskService) Create(userID string, input CreateTaskInput) (repository.Task, error) {
	if userID == "" {
		return repository.Task{}, repository.ErrForbidden
	}
	if input.Title == "" {
		return repository.Task{}, repository.ValidationError("title is required")
	}
	if err := validateDueAt(input.DueAt); err != nil {
		return repository.Task{}, err
	}
	status, err := normalizeStatus(input.Status)
	if err != nil {
		return repository.Task{}, err
	}
	if err := s.checkProject(userID, input.ProjectID); err != nil {
		return repository.Task{}, err
	}
	return s.repo.CreateTask(repository.Task{
		UserID:      userID,
		Title:       input.Title,
		Description: input.Description,
		DueAt:       input.DueAt,
		Status:      status,
		ProjectID:   input.ProjectID,
	})
}

func (s *TaskService) List(userID string) ([]repository.Task, error) {
	return s.repo.ListTasks(userID)
}

func (s *TaskService) Get(userID, taskID string) (repository.Task, error) {
	return s.repo.GetTask(userID, taskID)
}

func (s *TaskService) Update(userID, taskID string, input UpdateTaskInput) (repository.Task, error) {
	existing, err := s.repo.GetTask(userID, taskID)
	if err != nil {
		return repository.Task{}, err
	}
	if input.Version != 0 && input.Version != existing.Version {
		return repository.Task{}, repository.ErrConflict
	}
	if input.Title != nil {
		if *input.Title == "" {
			return repository.Task{}, repository.ValidationError("title is required")
		}
		existing.Title = *input.Title
	}
	if input.Description != nil {
		existing.Description = *input.Description
	}
	if input.DueAt != nil {
		if err := validateDueAt(*input.DueAt); err != nil {
			return repository.Task{}, err
		}
		existing.DueAt = *input.DueAt
	}
	if input.Status != nil {
		status, err := normalizeStatus(*input.Status)
		if err != nil {
			return repository.Task{}, err
		}
		existing.Status = status
	}
	if input.ProjectID != nil {
		projectID := ""
		if *input.ProjectID != nil {
			projectID = **input.ProjectID
		}
		if err := s.checkProject(userID, projectID); err != nil {
			return repository.Task{}, err
		}
		existing.ProjectID = projectID
	}
	return s.repo.UpdateTask(existing)
}

func (s *TaskService) Delete(userID, taskID string) error {
	return s.repo.DeleteTask(userID, taskID)
}

// checkProject keeps a task from being filed under someone else's project, or
// one that does not exist.
func (s *TaskService) checkProject(userID, projectID string) error {
	if projectID == "" {
		return nil
	}
	if _, err := s.repo.GetProject(userID, projectID); err != nil {
		return repository.ValidationError("projectId does not exist")
	}
	return nil
}

func normalizeStatus(status string) (string, error) {
	switch status {
	case "":
		return repository.TaskStatusOpen, nil
	case repository.TaskStatusOpen, repository.TaskStatusDone:
		return status, nil
	default:
		return "", repository.ValidationError("status must be OPEN or DONE")
	}
}

func validateDueAt(dueAt string) error {
	if dueAt == "" {
		return nil
	}
	if _, err := time.Parse("2006-01-02", dueAt); err != nil {
		return repository.ValidationError("dueAt must be YYYY-MM-DD")
	}
	return nil
}

type ProjectService struct {
	repo repository.ProjectRepository
}

type CreateProjectInput struct {
	Name        string
	Description string
}

type UpdateProjectInput struct {
	Name        *string
	Description *string
	Version     int
}

func NewProjectService(repo repository.ProjectRepository) *ProjectService {
	return &ProjectService{repo: repo}
}

func (s *ProjectService) Create(userID string, input CreateProjectInput) (repository.Project, error) {
	if userID == "" {
		return repository.Project{}, repository.ErrForbidden
	}
	if input.Name == "" {
		return repository.Project{}, repository.ValidationError("name is required")
	}
	return s.repo.CreateProject(repository.Project{
		UserID:      userID,
		Name:        input.Name,
		Description: input.Description,
	})
}

func (s *ProjectService) List(userID string) ([]repository.Project, error) {
	return s.repo.ListProjects(userID)
}

func (s *ProjectService) Get(userID, projectID string) (repository.Project, error) {
	return s.repo.GetProject(userID, projectID)
}

func (s *ProjectService) Update(userID, projectID string, input UpdateProjectInput) (repository.Project, error) {
	existing, err := s.repo.GetProject(userID, projectID)
	if err != nil {
		return repository.Project{}, err
	}
	if input.Version != 0 && input.Version != existing.Version {
		return repository.Project{}, repository.ErrConflict
	}
	if input.Name != nil {
		if *input.Name == "" {
			return repository.Project{}, repository.ValidationError("name is required")
		}
		existing.Name = *input.Name
	}
	if input.Description != nil {
		existing.Description = *input.Description
	}
	return s.repo.UpdateProject(existing)
}

func (s *ProjectService) Delete(userID, projectID string) error {
	return s.repo.DeleteProject(userID, projectID)
}
