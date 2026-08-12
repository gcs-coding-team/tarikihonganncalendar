package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
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

// Tokens are cached per handler so repeated calls as the same actor stay the
// same account. Registering is how a test gets in at all now — the X-User-ID
// shortcut it used to lean on is off by default, which is the point of it.
//
// The handler itself is the key. Keying on its address instead let a collected
// handler's tokens be handed to a new one that landed on the same address,
// which failed only when the whole package ran at once.
var testTokens = map[*Handler]map[string]string{}

func tokenFor(t *testing.T, mux *Handler, actor string) string {
	t.Helper()
	if tok, ok := testTokens[mux][actor]; ok {
		return tok
	}
	body := fmt.Sprintf(`{"email":%q,"password":"testpassword","displayName":%q}`,
		actor+"@test.local", actor)
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/register", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("register %s: got %d body=%s", actor, rr.Code, rr.Body.String())
	}
	var out struct {
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode register: %v", err)
	}
	if testTokens[mux] == nil {
		testTokens[mux] = map[string]string{}
	}
	testTokens[mux][actor] = out.Data.Token
	return out.Data.Token
}

// call makes a request as the named actor, registering them on first use. An
// empty actor sends no credentials at all.
func call(t *testing.T, mux *Handler, method, path, actor, body string) *httptest.ResponseRecorder {
	t.Helper()
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, path, nil)
	} else {
		req = httptest.NewRequest(method, path, bytes.NewReader([]byte(body)))
		req.Header.Set("Content-Type", "application/json")
	}
	if actor != "" {
		req.Header.Set("Authorization", "Bearer "+tokenFor(t, mux, actor))
	}
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	return rr
}

func newColony(t *testing.T, mux *Handler, userID, name string) colonyBody {
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
