package service

import (
	"context"
	"errors"
	"log"

	"github.com/gcs-coding-team/tarikihonganncalendar/internal/repository"
	"github.com/gcs-coding-team/tarikihonganncalendar/internal/service/vision"
)

// Analyser is what actually reads an image. It is an interface so the service
// can be exercised without a model running.
type Analyser interface {
	Analyse(ctx context.Context, image []byte) ([]repository.Candidate, error)
	Available(ctx context.Context) bool
}

// A job moves queued → processing → review_required, or → failed. Nothing is
// written to the calendar along the way: the candidates wait for a person to
// confirm them, because a handout misread into the wrong deadline is worse than
// one that was never read.
const (
	JobQueued     = "queued"
	JobProcessing = "processing"
	JobReview     = "review_required"
	JobFailed     = "failed"
)

type AnalysisService struct {
	repo     repository.Repository
	analyser Analyser
}

func NewAnalysisService(repo repository.Repository, analyser Analyser) *AnalysisService {
	return &AnalysisService{repo: repo, analyser: analyser}
}

func (s *AnalysisService) Create(userID, contentType, filename string) (repository.AnalysisJob, error) {
	if userID == "" {
		return repository.AnalysisJob{}, repository.ErrForbidden
	}
	return s.repo.CreateAnalysisJob(repository.AnalysisJob{
		UserID: userID, ContentType: contentType, Filename: filename, Status: JobQueued,
	})
}

func (s *AnalysisService) List(userID string) ([]repository.AnalysisJob, error) {
	return s.repo.ListAnalysisJobs(userID)
}

func (s *AnalysisService) Get(userID, jobID string) (repository.AnalysisJob, error) {
	job, err := s.repo.GetAnalysisJob(jobID)
	if err != nil {
		return repository.AnalysisJob{}, err
	}
	if job.UserID != userID {
		return repository.AnalysisJob{}, repository.ErrForbidden
	}
	return job, nil
}

func (s *AnalysisService) Candidates(userID, jobID string) ([]repository.Candidate, error) {
	if _, err := s.Get(userID, jobID); err != nil {
		return nil, err
	}
	return s.repo.ListCandidates(jobID)
}

// Run reads the image and records what it found. It runs inline rather than in a
// worker: one image takes seconds, and a queue that can lose jobs is a worse
// trade than a request that waits.
func (s *AnalysisService) Run(ctx context.Context, userID, jobID string, image []byte) (repository.AnalysisJob, error) {
	job, err := s.Get(userID, jobID)
	if err != nil {
		return repository.AnalysisJob{}, err
	}
	if len(image) == 0 {
		return repository.AnalysisJob{}, repository.ValidationError("image is required")
	}
	if s.analyser == nil {
		return s.fail(job, "解析モデルが設定されていません")
	}

	job.Status = JobProcessing
	if job, err = s.repo.UpdateAnalysisJob(job); err != nil {
		return repository.AnalysisJob{}, err
	}

	cands, err := s.analyser.Analyse(ctx, image)
	if err != nil {
		log.Printf("analysis %s failed: %v", jobID, err)
		if errors.Is(err, vision.ErrUnavailable) {
			return s.fail(job, "解析モデルに接続できませんでした")
		}
		return s.fail(job, "プリントを読み取れませんでした")
	}
	if err := s.repo.SaveCandidates(jobID, cands); err != nil {
		return repository.AnalysisJob{}, err
	}
	job.Status = JobReview
	job.ResultSummary = summarize(cands)
	return s.repo.UpdateAnalysisJob(job)
}

func (s *AnalysisService) fail(job repository.AnalysisJob, reason string) (repository.AnalysisJob, error) {
	job.Status = JobFailed
	job.ResultSummary = reason
	return s.repo.UpdateAnalysisJob(job)
}

// Ready reports whether a model is reachable, so the app can say so before
// someone bothers to take a photo.
func (s *AnalysisService) Ready(ctx context.Context) bool {
	return s.analyser != nil && s.analyser.Available(ctx)
}

func summarize(cands []repository.Candidate) string {
	if len(cands) == 0 {
		return "日付は見つかりませんでした"
	}
	tasks, events := 0, 0
	for _, c := range cands {
		if c.Type == "event" {
			events++
		} else {
			tasks++
		}
	}
	switch {
	case events == 0:
		return itoa(tasks) + "件のタスク"
	case tasks == 0:
		return itoa(events) + "件の予定"
	default:
		return itoa(tasks) + "件のタスクと" + itoa(events) + "件の予定"
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
