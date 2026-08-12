package repository

import (
	"sort"
	"sync"
	"time"
)

// Task and Project live here alongside events and timetable entries, following
// the same shape: owned by a user, versioned for optimistic locking.
//
// A task may belong to a project, or to none — ProjectID is empty for the
// unfiled ones. Deleting a project unfiles its tasks rather than taking them
// with it; losing work because a grouping went away would be a bad trade.

const (
	TaskStatusOpen = "OPEN"
	TaskStatusDone = "DONE"
)

type Task struct {
	ID          string `json:"id"`
	UserID      string `json:"userId"`
	Title       string `json:"title"`
	Description string `json:"description"`
	// DueAt is a calendar day (YYYY-MM-DD), not an instant: a task is due on a
	// day, and pinning it to a time zone only creates off-by-one bugs.
	DueAt     string    `json:"dueAt,omitempty"`
	Status    string    `json:"status"`
	ProjectID string    `json:"projectId,omitempty"`
	Version   int       `json:"version"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type Project struct {
	ID          string    `json:"id"`
	UserID      string    `json:"userId"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Version     int       `json:"version"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type TaskRepository interface {
	CreateTask(task Task) (Task, error)
	ListTasks(userID string) ([]Task, error)
	GetTask(userID, taskID string) (Task, error)
	UpdateTask(task Task) (Task, error)
	DeleteTask(userID, taskID string) error
}

type ProjectRepository interface {
	CreateProject(project Project) (Project, error)
	ListProjects(userID string) ([]Project, error)
	GetProject(userID, projectID string) (Project, error)
	UpdateProject(project Project) (Project, error)
	DeleteProject(userID, projectID string) error
}

// taskStore holds the task and project maps. MemoryRepository embeds it so the
// existing constructor keeps working without listing every map twice.
type taskStore struct {
	taskMu   sync.RWMutex
	tasks    map[string]Task
	projects map[string]Project
}

func newTaskStore() taskStore {
	return taskStore{
		tasks:    make(map[string]Task),
		projects: make(map[string]Project),
	}
}

func (r *MemoryRepository) CreateTask(task Task) (Task, error) {
	r.taskMu.Lock()
	defer r.taskMu.Unlock()
	if task.ProjectID != "" {
		if _, ok := r.projects[task.ProjectID]; !ok {
			return Task{}, ValidationError("projectId does not exist")
		}
	}
	if task.ID == "" {
		task.ID = newID()
	}
	if task.Status == "" {
		task.Status = TaskStatusOpen
	}
	if task.CreatedAt.IsZero() {
		task.CreatedAt = time.Now().UTC()
	}
	task.UpdatedAt = task.CreatedAt
	if task.Version == 0 {
		task.Version = 1
	}
	r.tasks[task.ID] = task
	return task, nil
}

func (r *MemoryRepository) ListTasks(userID string) ([]Task, error) {
	r.taskMu.RLock()
	defer r.taskMu.RUnlock()
	items := make([]Task, 0)
	for _, task := range r.tasks {
		if task.UserID == userID {
			items = append(items, task)
		}
	}
	// Soonest deadline first; undated tasks trail behind.
	sort.Slice(items, func(i, j int) bool {
		if (items[i].DueAt == "") != (items[j].DueAt == "") {
			return items[j].DueAt == ""
		}
		if items[i].DueAt != items[j].DueAt {
			return items[i].DueAt < items[j].DueAt
		}
		return items[i].ID < items[j].ID
	})
	return items, nil
}

func (r *MemoryRepository) GetTask(userID, taskID string) (Task, error) {
	r.taskMu.RLock()
	defer r.taskMu.RUnlock()
	task, ok := r.tasks[taskID]
	if !ok {
		return Task{}, ErrNotFound
	}
	if task.UserID != userID {
		return Task{}, ErrForbidden
	}
	return task, nil
}

func (r *MemoryRepository) UpdateTask(task Task) (Task, error) {
	r.taskMu.Lock()
	defer r.taskMu.Unlock()
	existing, ok := r.tasks[task.ID]
	if !ok {
		return Task{}, ErrNotFound
	}
	if existing.Version != task.Version {
		return Task{}, ErrConflict
	}
	if task.ProjectID != "" {
		if _, ok := r.projects[task.ProjectID]; !ok {
			return Task{}, ValidationError("projectId does not exist")
		}
	}
	task.Version = existing.Version + 1
	task.CreatedAt = existing.CreatedAt
	task.UpdatedAt = time.Now().UTC()
	r.tasks[task.ID] = task
	return task, nil
}

func (r *MemoryRepository) DeleteTask(userID, taskID string) error {
	r.taskMu.Lock()
	defer r.taskMu.Unlock()
	task, ok := r.tasks[taskID]
	if !ok {
		return ErrNotFound
	}
	if task.UserID != userID {
		return ErrForbidden
	}
	delete(r.tasks, taskID)
	return nil
}

func (r *MemoryRepository) CreateProject(project Project) (Project, error) {
	r.taskMu.Lock()
	defer r.taskMu.Unlock()
	if project.ID == "" {
		project.ID = newID()
	}
	if project.CreatedAt.IsZero() {
		project.CreatedAt = time.Now().UTC()
	}
	project.UpdatedAt = project.CreatedAt
	if project.Version == 0 {
		project.Version = 1
	}
	r.projects[project.ID] = project
	return project, nil
}

func (r *MemoryRepository) ListProjects(userID string) ([]Project, error) {
	r.taskMu.RLock()
	defer r.taskMu.RUnlock()
	items := make([]Project, 0)
	for _, project := range r.projects {
		if project.UserID == userID {
			items = append(items, project)
		}
	}
	sort.Slice(items, func(i, j int) bool {
		if !items[i].CreatedAt.Equal(items[j].CreatedAt) {
			return items[i].CreatedAt.Before(items[j].CreatedAt)
		}
		return items[i].ID < items[j].ID
	})
	return items, nil
}

func (r *MemoryRepository) GetProject(userID, projectID string) (Project, error) {
	r.taskMu.RLock()
	defer r.taskMu.RUnlock()
	project, ok := r.projects[projectID]
	if !ok {
		return Project{}, ErrNotFound
	}
	if project.UserID != userID {
		return Project{}, ErrForbidden
	}
	return project, nil
}

func (r *MemoryRepository) UpdateProject(project Project) (Project, error) {
	r.taskMu.Lock()
	defer r.taskMu.Unlock()
	existing, ok := r.projects[project.ID]
	if !ok {
		return Project{}, ErrNotFound
	}
	if existing.Version != project.Version {
		return Project{}, ErrConflict
	}
	project.Version = existing.Version + 1
	project.CreatedAt = existing.CreatedAt
	project.UpdatedAt = time.Now().UTC()
	r.projects[project.ID] = project
	return project, nil
}

// DeleteProject unfiles the project's tasks instead of deleting them. Removing
// a grouping should not destroy the work filed under it.
func (r *MemoryRepository) DeleteProject(userID, projectID string) error {
	r.taskMu.Lock()
	defer r.taskMu.Unlock()
	project, ok := r.projects[projectID]
	if !ok {
		return ErrNotFound
	}
	if project.UserID != userID {
		return ErrForbidden
	}
	delete(r.projects, projectID)
	for id, task := range r.tasks {
		if task.ProjectID == projectID {
			task.ProjectID = ""
			task.UpdatedAt = time.Now().UTC()
			r.tasks[id] = task
		}
	}
	return nil
}
