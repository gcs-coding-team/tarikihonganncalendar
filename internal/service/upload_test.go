package service

import (
	"testing"
)

func TestAnalysisJobServiceCreatesJob(t *testing.T) {
	svc := NewAnalysisJobService()
	job, err := svc.CreateJob("user-1", "image/png", "sample.png")
	if err != nil {
		t.Fatalf("create job: %v", err)
	}
	if job.Status != "queued" {
		t.Fatalf("expected queued status, got %s", job.Status)
	}
	if job.UserID != "user-1" {
		t.Fatalf("expected user id to be preserved, got %s", job.UserID)
	}
}
