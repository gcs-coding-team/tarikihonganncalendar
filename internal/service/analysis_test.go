package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/gcs-coding-team/tarikihonganncalendar/internal/repository"
	"github.com/gcs-coding-team/tarikihonganncalendar/internal/service/vision"
)

type stubAnalyser struct {
	calls   int
	failFor int // fail this many times, then succeed
	err     error
}

func (s *stubAnalyser) Analyse(ctx context.Context, image []byte) ([]repository.Candidate, error) {
	s.calls++
	if s.calls <= s.failFor {
		return nil, s.err
	}
	return []repository.Candidate{{Type: "task", Title: "数学プリント", Date: "2026-08-20"}}, nil
}

func (s *stubAnalyser) Available(ctx context.Context) bool { return true }

func newJob(t *testing.T, repo repository.Repository) repository.AnalysisJob {
	t.Helper()
	job, err := repo.CreateAnalysisJob(repository.AnalysisJob{UserID: "u1", Status: JobQueued})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}
	return job
}

func withFastBackoff(svc *AnalysisService) *AnalysisService {
	svc.backoff = time.Millisecond
	return svc
}

// A cold model that is still loading answers on the second or third try; giving
// up on the first would make the first upload after a boot fail for no reason.
func TestAnalysisRetriesAnUnreachableModel(t *testing.T) {
	repo := repository.NewMemoryRepository()
	stub := &stubAnalyser{failFor: 2, err: vision.ErrUnavailable}
	svc := withFastBackoff(NewAnalysisService(repo, stub, nil))
	job := newJob(t, repo)

	out, err := svc.Run(context.Background(), "u1", job.ID, []byte{1, 2, 3})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if stub.calls != 3 {
		t.Fatalf("expected 3 attempts, got %d", stub.calls)
	}
	if out.Status != JobReview {
		t.Fatalf("expected the job to succeed, got %q (%s)", out.Status, out.ResultSummary)
	}
	cands, _ := svc.Candidates("u1", job.ID)
	if len(cands) != 1 {
		t.Fatalf("candidates not saved: %+v", cands)
	}
}

func TestAnalysisGivesUpAfterMaxAttempts(t *testing.T) {
	repo := repository.NewMemoryRepository()
	stub := &stubAnalyser{failFor: 99, err: vision.ErrUnavailable}
	svc := withFastBackoff(NewAnalysisService(repo, stub, nil))
	job := newJob(t, repo)

	out, err := svc.Run(context.Background(), "u1", job.ID, []byte{1})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if stub.calls != svc.MaxAttempts {
		t.Fatalf("expected %d attempts, got %d", svc.MaxAttempts, stub.calls)
	}
	if out.Status != JobFailed {
		t.Fatalf("expected failed, got %q", out.Status)
	}
	// Nothing invented on the way out.
	if cands, _ := svc.Candidates("u1", job.ID); len(cands) != 0 {
		t.Fatalf("a failed job produced candidates: %+v", cands)
	}
}

// Retrying an answer the parser could not read just wastes the model's time —
// it will say the same thing again.
func TestAnalysisDoesNotRetryAnUnreadableAnswer(t *testing.T) {
	repo := repository.NewMemoryRepository()
	stub := &stubAnalyser{failFor: 99, err: errors.New("no JSON in model output")}
	svc := withFastBackoff(NewAnalysisService(repo, stub, nil))
	job := newJob(t, repo)

	out, _ := svc.Run(context.Background(), "u1", job.ID, []byte{1})
	if stub.calls != 1 {
		t.Fatalf("expected a single attempt, got %d", stub.calls)
	}
	if out.Status != JobFailed {
		t.Fatalf("expected failed, got %q", out.Status)
	}
}

func TestAnalysisWithoutAnyModel(t *testing.T) {
	repo := repository.NewMemoryRepository()
	svc := NewAnalysisService(repo, nil, nil)
	job := newJob(t, repo)

	out, err := svc.Run(context.Background(), "u1", job.ID, []byte{1})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if out.Status != JobFailed {
		t.Fatalf("expected failed, got %q", out.Status)
	}
	if svc.Ready(context.Background()) {
		t.Fatal("Ready should be false with no analyser")
	}
}

func TestAnalysisRejectsAnotherUsersJob(t *testing.T) {
	repo := repository.NewMemoryRepository()
	svc := NewAnalysisService(repo, &stubAnalyser{}, nil)
	job := newJob(t, repo)

	if _, err := svc.Run(context.Background(), "someone-else", job.ID, []byte{1}); !errors.Is(err, repository.ErrForbidden) {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}
}
