package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/gcs-coding-team/tarikihonganncalendar/internal/repository"
	"github.com/gcs-coding-team/tarikihonganncalendar/internal/storage"
)

// Reads back exactly what it was handed, so the round trip can be checked
// without touching a disk.
type memBlobs map[string][]byte

func (m memBlobs) Put(_ context.Context, key string, data []byte, _ string) error {
	m[key] = data
	return nil
}
func (m memBlobs) Get(_ context.Context, key string) ([]byte, error) {
	data, ok := m[key]
	if !ok {
		return nil, storage.ErrNoSuchObject
	}
	return data, nil
}
func (m memBlobs) Delete(_ context.Context, key string) error { delete(m, key); return nil }

func analyseAs(t *testing.T, mux *Handler, actor string, image []byte) string {
	t.Helper()
	rr := call(t, mux, http.MethodPost, "/v1/uploads/jobs", actor,
		`{"contentType":"image/png","filename":"print.png"}`)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create job: got %d body=%s", rr.Code, rr.Body.String())
	}
	var job struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	decodeInto(t, rr.Body.Bytes(), &job)

	req := newRequest(http.MethodPost, "/v1/uploads/jobs/"+job.Data.ID+"/analyse", string(image))
	req.Header.Set("Authorization", "Bearer "+tokenFor(t, mux, actor))
	serve(mux, req)
	return job.Data.ID
}

// The handout has to survive the read, including a read that failed: otherwise
// the app holds dates whose source it cannot show.
func TestPrintIsKeptAndCanBeReadBack(t *testing.T) {
	blobs := memBlobs{}
	mux := NewHandler(repository.NewMemoryRepository(), Options{Blobs: blobs})

	image := []byte("not really a png, but bytes all the same")
	analyseAs(t, mux, "owner", image)

	rr := call(t, mux, http.MethodGet, "/v1/prints", "owner", "")
	if rr.Code != http.StatusOK {
		t.Fatalf("list prints: got %d", rr.Code)
	}
	var prints struct {
		Data []struct {
			ID          string `json:"id"`
			Filename    string `json:"filename"`
			ContentType string `json:"contentType"`
		} `json:"data"`
	}
	decodeInto(t, rr.Body.Bytes(), &prints)
	// The analysis failed — no model is configured — and the image was still kept.
	if len(prints.Data) != 1 {
		t.Fatalf("expected the handout to be kept, got %s", rr.Body.String())
	}
	if prints.Data[0].Filename != "print.png" {
		t.Errorf("filename lost: %+v", prints.Data[0])
	}

	rr = call(t, mux, http.MethodGet, "/v1/prints/"+prints.Data[0].ID+"/image", "owner", "")
	if rr.Code != http.StatusOK {
		t.Fatalf("fetch image: got %d body=%s", rr.Code, rr.Body.String())
	}
	if rr.Body.String() != string(image) {
		t.Fatalf("image came back changed: %q", rr.Body.String())
	}
	if got := rr.Header().Get("Content-Type"); got != "image/png" {
		t.Errorf("content type not preserved: %q", got)
	}
	// Someone else's handout must not sit in a shared cache.
	if got := rr.Header().Get("Cache-Control"); got == "" || got[:7] != "private" {
		t.Errorf("image is not marked private: %q", got)
	}
}

func TestAnotherAccountCannotReadYourPrints(t *testing.T) {
	blobs := memBlobs{}
	mux := NewHandler(repository.NewMemoryRepository(), Options{Blobs: blobs})
	analyseAs(t, mux, "owner", []byte("mine"))

	rr := call(t, mux, http.MethodGet, "/v1/prints", "owner", "")
	var prints struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	decodeInto(t, rr.Body.Bytes(), &prints)

	if rr := call(t, mux, http.MethodGet, "/v1/prints/"+prints.Data[0].ID+"/image", "stranger", ""); rr.Code != http.StatusForbidden {
		t.Fatalf("another account read the image: got %d", rr.Code)
	}
	if rr := call(t, mux, http.MethodGet, "/v1/prints", "stranger", ""); rr.Body.String() != `{"data":[]}`+"\n" {
		t.Fatalf("another account saw the list: %s", rr.Body.String())
	}
}

// With nowhere to keep them, analysis still works — only the looking back is lost.
func TestAnalysisWorksWithoutBlobStorage(t *testing.T) {
	mux := NewHandler(repository.NewMemoryRepository())
	analyseAs(t, mux, "owner", []byte("bytes"))

	rr := call(t, mux, http.MethodGet, "/v1/prints", "owner", "")
	var out struct {
		Data []json.RawMessage `json:"data"`
	}
	decodeInto(t, rr.Body.Bytes(), &out)
	if len(out.Data) != 1 {
		t.Fatalf("the print record should still exist: %s", rr.Body.String())
	}
	// ...but the bytes are gone.
	var prints struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	decodeInto(t, rr.Body.Bytes(), &prints)
	if rr := call(t, mux, http.MethodGet, "/v1/prints/"+prints.Data[0].ID+"/image", "owner", ""); rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for a discarded image, got %d", rr.Code)
	}
}
