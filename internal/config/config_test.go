package config

import (
	"os"
	"testing"
)

func TestLoad_Defaults(t *testing.T) {
	cfg := Load()

	if cfg.HTTPPort != "8080" {
		t.Fatalf("expected 8080, got %s", cfg.HTTPPort)
	}
	if cfg.AppEnv != "development" {
		t.Fatalf("expected development, got %s", cfg.AppEnv)
	}
	if cfg.WorkerConcurrency != 1 {
		t.Fatalf("expected 1, got %d", cfg.WorkerConcurrency)
	}
	if cfg.WorkerMaxAttempts != 3 {
		t.Fatalf("expected 3, got %d", cfg.WorkerMaxAttempts)
	}
}

func TestLoad_FromEnv(t *testing.T) {
	os.Setenv("HTTP_PORT", "9090")
	os.Setenv("APP_ENV", "production")
	os.Setenv("WORKER_CONCURRENCY", "5")
	os.Setenv("MAX_UPLOAD_BYTES", "2097152")
	defer func() {
		os.Unsetenv("HTTP_PORT")
		os.Unsetenv("APP_ENV")
		os.Unsetenv("WORKER_CONCURRENCY")
		os.Unsetenv("MAX_UPLOAD_BYTES")
	}()

	cfg := Load()

	if cfg.HTTPPort != "9090" {
		t.Fatalf("expected 9090, got %s", cfg.HTTPPort)
	}
	if cfg.AppEnv != "production" {
		t.Fatalf("expected production, got %s", cfg.AppEnv)
	}
	if cfg.WorkerConcurrency != 5 {
		t.Fatalf("expected 5, got %d", cfg.WorkerConcurrency)
	}
	if cfg.MaxUploadBytes != 2097152 {
		t.Fatalf("expected 2097152, got %d", cfg.MaxUploadBytes)
	}
}

func TestLoad_InvalidIntFallback(t *testing.T) {
	os.Setenv("WORKER_CONCURRENCY", "invalid")
	defer os.Unsetenv("WORKER_CONCURRENCY")

	cfg := Load()
	if cfg.WorkerConcurrency != 1 {
		t.Fatalf("expected default 1, got %d", cfg.WorkerConcurrency)
	}
}
