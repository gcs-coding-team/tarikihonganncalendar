package vision

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// A stand-in for Ollama, so the whole request path is exercised without a model
// on the machine.
func fakeOllama(t *testing.T, reply string, status int) (*httptest.Server, *[]ollamaRequest) {
	t.Helper()
	var seen []ollamaRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/tags" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"models":[]}`))
			return
		}
		var req ollamaRequest
		json.NewDecoder(r.Body).Decode(&req)
		seen = append(seen, req)
		w.WriteHeader(status)
		json.NewEncoder(w).Encode(ollamaResponse{Response: reply})
	}))
	t.Cleanup(srv.Close)
	return srv, &seen
}

func TestAnalyseSendsImageAndReadsCandidates(t *testing.T) {
	srv, seen := fakeOllama(t,
		`{"items":[{"type":"task","title":"数学プリント p.24","date":"2026-08-20"}]}`,
		http.StatusOK)
	a := New(Config{BaseURL: srv.URL, Model: "gemma3:4b", Timeout: 5 * time.Second})

	image := []byte{0x89, 'P', 'N', 'G', 0x00, 0x01}
	out, err := a.Analyse(context.Background(), image)
	if err != nil {
		t.Fatalf("analyse: %v", err)
	}
	if len(out) != 1 || out[0].Title != "数学プリント p.24" {
		t.Fatalf("unexpected candidates: %+v", out)
	}

	if len(*seen) != 1 {
		t.Fatalf("expected one call, got %d", len(*seen))
	}
	req := (*seen)[0]
	if req.Model != "gemma3:4b" {
		t.Errorf("model not passed through: %q", req.Model)
	}
	// The image has to actually reach the model, base64'd as Ollama expects.
	if len(req.Images) != 1 || req.Images[0] != base64.StdEncoding.EncodeToString(image) {
		t.Errorf("image not sent correctly: %+v", req.Images)
	}
	if req.Stream {
		t.Error("streaming would break the single-shot decode")
	}
	// Today's date goes in the prompt so a handout saying 8/20 resolves to a year.
	if !contains(req.Prompt, time.Now().Format("2006-01-02")) {
		t.Error("prompt is missing today's date")
	}
}

func TestAnalyseReportsUnavailableModel(t *testing.T) {
	a := New(Config{BaseURL: "http://127.0.0.1:1", Model: "gemma3:4b", Timeout: time.Second})
	if _, err := a.Analyse(context.Background(), []byte{1}); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("expected ErrUnavailable, got %v", err)
	}
	if a.Available(context.Background()) {
		t.Fatal("Available should be false when nothing is listening")
	}
}

func TestAnalyseReportsServerError(t *testing.T) {
	srv, _ := fakeOllama(t, "", http.StatusInternalServerError)
	a := New(Config{BaseURL: srv.URL, Model: "gemma3:4b", Timeout: 5 * time.Second})
	if _, err := a.Analyse(context.Background(), []byte{1}); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("expected ErrUnavailable for a 500, got %v", err)
	}
}

func TestAvailable(t *testing.T) {
	srv, _ := fakeOllama(t, `{"items":[]}`, http.StatusOK)
	a := New(Config{BaseURL: srv.URL, Model: "gemma3:4b", Timeout: 5 * time.Second})
	if !a.Available(context.Background()) {
		t.Fatal("Available should be true when the server answers")
	}
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (func() bool {
		for i := 0; i+len(needle) <= len(haystack); i++ {
			if haystack[i:i+len(needle)] == needle {
				return true
			}
		}
		return false
	})()
}
