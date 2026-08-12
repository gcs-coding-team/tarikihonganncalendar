package service

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/gcs-coding-team/tarikihonganncalendar/internal/repository"
	"github.com/gcs-coding-team/tarikihonganncalendar/internal/service/vision"
	"github.com/gcs-coding-team/tarikihonganncalendar/internal/storage"
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
	// blobs keeps the image itself. Without it the app can tell you what it
	// read but never show you what it read it from.
	blobs storage.Blobs
	// MaxAttempts covers a model that is briefly busy or still loading — the
	// common case for a first request against a cold Ollama. It does not paper
	// over a model that is simply not there: those attempts fail fast and the
	// job still ends up failed, just a second or two later.
	MaxAttempts int
	backoff     time.Duration
}

func NewAnalysisService(repo repository.Repository, analyser Analyser, blobs storage.Blobs) *AnalysisService {
	if blobs == nil {
		blobs = storage.NewDiscardBlobs()
	}
	return &AnalysisService{
		repo: repo, analyser: analyser, blobs: blobs,
		MaxAttempts: 3, backoff: 500 * time.Millisecond,
	}
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
	// Keep the image first. Whether it can be read is a separate question from
	// whether it is worth keeping, and a read that fails still leaves the
	// person something to look at. Failing to store it is not worth failing
	// the analysis over either, so the error is logged and the job goes on.
	if err := s.keep(ctx, job, image); err != nil {
		log.Printf("analysis %s: could not keep the image: %v", jobID, err)
	}

	if s.analyser == nil {
		return s.fail(job, "解析モデルが設定されていません")
	}

	job.Status = JobProcessing
	if job, err = s.repo.UpdateAnalysisJob(job); err != nil {
		return repository.AnalysisJob{}, err
	}

	cands, err := s.analyseWithRetries(ctx, jobID, image)
	if err != nil {
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

// analyseWithRetries retries only what retrying can fix. A model that answered
// with something unreadable will answer the same way again, so that is returned
// on the first try; being unable to reach it at all is worth another go.
func (s *AnalysisService) analyseWithRetries(ctx context.Context, jobID string, image []byte) ([]repository.Candidate, error) {
	attempts := s.MaxAttempts
	if attempts < 1 {
		attempts = 1
	}
	var err error
	for attempt := 1; attempt <= attempts; attempt++ {
		var cands []repository.Candidate
		cands, err = s.analyser.Analyse(ctx, image)
		if err == nil {
			return cands, nil
		}
		if !errors.Is(err, vision.ErrUnavailable) {
			log.Printf("analysis %s: unreadable answer: %v", jobID, err)
			return nil, err
		}
		log.Printf("analysis %s: attempt %d/%d could not reach the model: %v", jobID, attempt, attempts, err)
		if attempt == attempts {
			break
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(s.backoff * time.Duration(attempt)):
		}
	}
	return nil, err
}

// keep writes the image and records where it went.
func (s *AnalysisService) keep(ctx context.Context, job repository.AnalysisJob, image []byte) error {
	key := "prints/" + job.UserID + "/" + job.ID
	if err := s.blobs.Put(ctx, key, image, job.ContentType); err != nil {
		return err
	}
	_, err := s.repo.CreatePrint(repository.Print{
		UserID: job.UserID, JobID: job.ID, ObjectKey: key,
		ContentType: job.ContentType, Filename: job.Filename,
	})
	return err
}

// Image returns the stored image for a print the caller owns.
func (s *AnalysisService) Image(ctx context.Context, userID, printID string) (repository.Print, []byte, error) {
	print, err := s.repo.GetPrint(userID, printID)
	if err != nil {
		return repository.Print{}, nil, err
	}
	data, err := s.blobs.Get(ctx, print.ObjectKey)
	if err != nil {
		return repository.Print{}, nil, repository.ErrNotFound
	}
	return print, data, nil
}

func (s *AnalysisService) Prints(userID string) ([]repository.Print, error) {
	return s.repo.ListPrints(userID)
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
