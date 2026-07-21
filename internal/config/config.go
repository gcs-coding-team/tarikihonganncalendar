package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	AppEnv     string
	HTTPPort   string
	DatabaseURL string

	SessionCookieName string
	SessionTTL        time.Duration

	FrontendOrigin string

	ObjectStorageEndpoint string
	ObjectStorageAccessKey string
	ObjectStorageSecretKey string
	ObjectStorageBucket   string
	ObjectStorageRegion   string

	PresignedURLTTL time.Duration

	MaxUploadBytes int64

	OllamaBaseURL       string
	OllamaModel         string
	OllamaTimeout       time.Duration

	WorkerConcurrency int
	WorkerMaxAttempts int
}

func Load() *Config {
	return &Config{
		AppEnv:     getEnv("APP_ENV", "development"),
		HTTPPort:   getEnv("HTTP_PORT", "8080"),
		DatabaseURL: getEnv("DATABASE_URL", "postgres://user:password@localhost:5432/tarikihongann?sslmode=disable"),

		SessionCookieName: getEnv("SESSION_COOKIE_NAME", "session"),
		SessionTTL:        getDuration("SESSION_TTL_HOURS", 720*time.Hour),

		FrontendOrigin: getEnv("FRONTEND_ORIGIN", ""),

		ObjectStorageEndpoint: getEnv("OBJECT_STORAGE_ENDPOINT", ""),
		ObjectStorageAccessKey: getEnv("OBJECT_STORAGE_ACCESS_KEY", ""),
		ObjectStorageSecretKey: getEnv("OBJECT_STORAGE_SECRET_KEY", ""),
		ObjectStorageBucket:   getEnv("OBJECT_STORAGE_BUCKET", ""),
		ObjectStorageRegion:   getEnv("OBJECT_STORAGE_REGION", ""),

		PresignedURLTTL: getDuration("PRESIGNED_URL_TTL_SECONDS", 5*time.Minute),

		MaxUploadBytes: getInt64("MAX_UPLOAD_BYTES", 10*1024*1024),

		OllamaBaseURL: getEnv("OLLAMA_BASE_URL", "http://ollama:11434"),
		OllamaModel:   getEnv("OLLAMA_MODEL", "gemma3:4b"),
		OllamaTimeout: getDuration("OLLAMA_TIMEOUT_SECONDS", 180*time.Second),

		WorkerConcurrency: getInt("WORKER_CONCURRENCY", 1),
		WorkerMaxAttempts: getInt("WORKER_MAX_ATTEMPTS", 3),
	}
}

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func getDuration(key string, defaultVal time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := strconv.Atoi(v); err == nil {
			return time.Duration(d) * time.Second
		}
	}
	return defaultVal
}

func getInt(key string, defaultVal int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return defaultVal
}

func getInt64(key string, defaultVal int64) int64 {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.ParseInt(v, 10, 64); err == nil {
			return i
		}
	}
	return defaultVal
}
