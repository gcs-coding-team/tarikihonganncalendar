package handler

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/gcs-coding-team/tarikihonganncalendar/internal/repository"
)

type eventBody struct {
	Data struct {
		ID      string             `json:"id"`
		Version int                `json:"version"`
		Repeat  *repository.Repeat `json:"repeat"`
		ExDates []string           `json:"exdates"`
	} `json:"data"`
}

func decodeEvent(t *testing.T, raw []byte) eventBody {
	t.Helper()
	var out eventBody
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decode event: %v (%s)", err, raw)
	}
	return out
}

const startEnd = `"startAt":"2026-08-12T16:00:00Z","endAt":"2026-08-12T18:00:00Z"`

func TestEventKeepsRepeatRule(t *testing.T) {
	mux := NewHandler(repository.NewMemoryRepository())

	rr := call(t, mux, http.MethodPost, "/v1/events", "u1",
		`{"title":"部活",`+startEnd+`,"repeat":{"freq":"weekly","until":"2026-12-31"}}`)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create: got %d body=%s", rr.Code, rr.Body.String())
	}
	ev := decodeEvent(t, rr.Body.Bytes())
	if ev.Data.Repeat == nil || ev.Data.Repeat.Freq != "weekly" || ev.Data.Repeat.Until != "2026-12-31" {
		t.Fatalf("repeat rule not kept: %s", rr.Body.String())
	}
	if ev.Data.ExDates == nil {
		t.Fatalf("exdates should always be present, got null: %s", rr.Body.String())
	}

	// Dropping one occurrence keeps the rule and records the day.
	rr = call(t, mux, http.MethodPatch, "/v1/events/"+ev.Data.ID, "u1",
		`{"exdates":["2026-08-19"],"version":1}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("patch exdates: got %d body=%s", rr.Code, rr.Body.String())
	}
	ev = decodeEvent(t, rr.Body.Bytes())
	if len(ev.Data.ExDates) != 1 || ev.Data.ExDates[0] != "2026-08-19" {
		t.Fatalf("exdate not kept: %s", rr.Body.String())
	}
	if ev.Data.Repeat == nil {
		t.Fatalf("patching exdates dropped the rule: %s", rr.Body.String())
	}

	// Leaving repeat out of the body must not disturb it.
	rr = call(t, mux, http.MethodPatch, "/v1/events/"+ev.Data.ID, "u1",
		`{"title":"部活（変更）","version":2}`)
	if ev = decodeEvent(t, rr.Body.Bytes()); ev.Data.Repeat == nil {
		t.Fatalf("omitted repeat cleared the rule: %s", rr.Body.String())
	}

	// Sending null clears it.
	rr = call(t, mux, http.MethodPatch, "/v1/events/"+ev.Data.ID, "u1",
		`{"repeat":null,"version":3}`)
	if ev = decodeEvent(t, rr.Body.Bytes()); ev.Data.Repeat != nil {
		t.Fatalf("explicit null did not clear the rule: %s", rr.Body.String())
	}
}

func TestEventRejectsBadRepeat(t *testing.T) {
	mux := NewHandler(repository.NewMemoryRepository())

	for _, body := range []string{
		`{"title":"x",` + startEnd + `,"repeat":{"freq":"yearly"}}`,
		`{"title":"x",` + startEnd + `,"repeat":{"freq":"weekly","until":"8/19"}}`,
	} {
		if rr := call(t, mux, http.MethodPost, "/v1/events", "u1", body); rr.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for %s, got %d", body, rr.Code)
		}
	}
}
