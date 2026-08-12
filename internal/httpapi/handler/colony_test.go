package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gcs-coding-team/tarikihonganncalendar/internal/repository"
)

type colonyBody struct {
	Data struct {
		ID         string `json:"id"`
		Name       string `json:"name"`
		InviteCode string `json:"inviteCode"`
	} `json:"data"`
}

func call(t *testing.T, mux http.Handler, method, path, userID, body string) *httptest.ResponseRecorder {
	t.Helper()
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, path, nil)
	} else {
		req = httptest.NewRequest(method, path, bytes.NewReader([]byte(body)))
		req.Header.Set("Content-Type", "application/json")
	}
	if userID != "" {
		req.Header.Set("X-User-ID", userID)
	}
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	return rr
}

func newColony(t *testing.T, mux http.Handler, userID, name string) colonyBody {
	t.Helper()
	rr := call(t, mux, http.MethodPost, "/v1/colonies", userID, `{"name":"`+name+`"}`)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create colony: got %d body=%s", rr.Code, rr.Body.String())
	}
	var out colonyBody
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode colony: %v", err)
	}
	return out
}

// A colony you joined has to show up in your list, not just the ones you made.
func TestListColoniesIncludesJoined(t *testing.T) {
	mux := NewHandler(repository.NewMemoryRepository())
	c := newColony(t, mux, "owner", "3-A")

	if rr := call(t, mux, http.MethodPost, "/v1/colonies/join", "guest",
		`{"inviteCode":"`+c.Data.InviteCode+`"}`); rr.Code != http.StatusOK {
		t.Fatalf("join: got %d body=%s", rr.Code, rr.Body.String())
	}

	rr := call(t, mux, http.MethodGet, "/v1/colonies", "guest", "")
	var list struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(list.Data) != 1 || list.Data[0].ID != c.Data.ID {
		t.Fatalf("joined colony missing from list: %s", rr.Body.String())
	}

	// The owner is a member too, so their own list keeps working.
	rr = call(t, mux, http.MethodGet, "/v1/colonies", "owner", "")
	if err := json.Unmarshal(rr.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode owner list: %v", err)
	}
	if len(list.Data) != 1 {
		t.Fatalf("owner lost sight of their colony: %s", rr.Body.String())
	}
}

// The invite code has to be enough on its own — the invitee never sees the ID.
func TestJoinByInviteCode(t *testing.T) {
	mux := NewHandler(repository.NewMemoryRepository())
	c := newColony(t, mux, "owner", "3-A")

	rr := call(t, mux, http.MethodPost, "/v1/colonies/join", "guest",
		`{"inviteCode":"`+c.Data.InviteCode+`"}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("join: got %d body=%s", rr.Code, rr.Body.String())
	}
	var joined colonyBody
	if err := json.Unmarshal(rr.Body.Bytes(), &joined); err != nil {
		t.Fatalf("decode join: %v", err)
	}
	if joined.Data.ID != c.Data.ID {
		t.Fatalf("joined the wrong colony: %q want %q", joined.Data.ID, c.Data.ID)
	}

	if rr := call(t, mux, http.MethodPost, "/v1/colonies/join", "guest",
		`{"inviteCode":"`+c.Data.InviteCode+`"}`); rr.Code != http.StatusConflict {
		t.Fatalf("joining twice: got %d want 409", rr.Code)
	}
	if rr := call(t, mux, http.MethodPost, "/v1/colonies/join", "guest",
		`{"inviteCode":"nope"}`); rr.Code != http.StatusNotFound {
		t.Fatalf("unknown code: got %d want 404", rr.Code)
	}
	if rr := call(t, mux, http.MethodPost, "/v1/colonies/join", "guest",
		`{"inviteCode":""}`); rr.Code != http.StatusBadRequest {
		t.Fatalf("empty code: got %d want 400", rr.Code)
	}
}

// Without the title, a shared item is an opaque ID to everyone who receives it.
func TestSharedItemKeepsTitleSnapshot(t *testing.T) {
	mux := NewHandler(repository.NewMemoryRepository())
	c := newColony(t, mux, "owner", "3-A")

	rr := call(t, mux, http.MethodPost, "/v1/colonies/"+c.Data.ID+"/shared-items", "owner",
		`{"sourceType":"TASK","sourceId":"task-1","titleSnapshot":"看板デザイン案"}`)
	if rr.Code != http.StatusCreated {
		t.Fatalf("share: got %d body=%s", rr.Code, rr.Body.String())
	}

	rr = call(t, mux, http.MethodGet, "/v1/colonies/"+c.Data.ID+"/feed", "owner", "")
	var feed struct {
		Data []struct {
			TitleSnapshot string `json:"titleSnapshot"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &feed); err != nil {
		t.Fatalf("decode feed: %v", err)
	}
	if len(feed.Data) != 1 || feed.Data[0].TitleSnapshot != "看板デザイン案" {
		t.Fatalf("title snapshot dropped: %s", rr.Body.String())
	}
}
