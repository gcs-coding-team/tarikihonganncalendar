package service

import (
	"github.com/gcs-coding-team/tarikihonganncalendar/internal/repository"
)

type AnalysisJobService struct {
	repo repository.AnalysisJobRepository
}

type CreateJobInput struct {
	ContentType string
	Filename    string
}

func NewAnalysisJobService() *AnalysisJobService {
	return &AnalysisJobService{}
}

func (s *AnalysisJobService) CreateJob(userID, contentType, filename string) (repository.AnalysisJob, error) {
	job := repository.AnalysisJob{UserID: userID, ContentType: contentType, Filename: filename, Status: "queued"}
	if s.repo != nil {
		return s.repo.CreateAnalysisJob(job)
	}
	return job, nil
}

func (s *AnalysisJobService) ListJobs(userID string) ([]repository.AnalysisJob, error) {
	if s.repo == nil {
		return nil, nil
	}
	return s.repo.ListAnalysisJobs(userID)
}

func (s *AnalysisJobService) GetJob(jobID string) (repository.AnalysisJob, error) {
	if s.repo == nil {
		return repository.AnalysisJob{}, repository.ErrNotFound
	}
	return s.repo.GetAnalysisJob(jobID)
}

func (s *AnalysisJobService) UpdateJob(job repository.AnalysisJob) (repository.AnalysisJob, error) {
	if s.repo == nil {
		return job, nil
	}
	return s.repo.UpdateAnalysisJob(job)
}
