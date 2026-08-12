package vision

import "testing"

// Models return JSON that is nearly right often enough that the parser, not the
// prompt, is what keeps a bad line out of someone's calendar.
func TestParseCandidatesKeepsOnlyUsableRows(t *testing.T) {
	out, err := ParseCandidates(`ここに結果です:
	{"items":[
	  {"type":"task","title":"数学プリント p.24","date":"2026-08-20"},
	  {"type":"event","title":"保護者会","date":"2026-08-25","time":"10:00"},
	  {"type":"task","title":"日付が読めなかったもの","date":""},
	  {"type":"task","title":"  ","date":"2026-08-21"},
	  {"type":"event","title":"時刻が変","date":"2026-08-26","time":"じゅう時"},
	  {"type":"task","title":"ありえない日","date":"2026-02-31"}
	]}
	以上です。`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(out) != 3 {
		t.Fatalf("expected 3 usable rows, got %d: %+v", len(out), out)
	}

	if out[0].Type != "task" || out[0].Date != "2026-08-20" {
		t.Errorf("first row wrong: %+v", out[0])
	}
	if out[1].Type != "event" || out[1].Time != "10:00" {
		t.Errorf("event lost its time: %+v", out[1])
	}
	// An unparseable time is dropped, but the event itself is worth keeping.
	if out[2].Title != "時刻が変" || out[2].Time != "" {
		t.Errorf("expected the event kept and the bad time dropped: %+v", out[2])
	}
}

func TestParseCandidatesEmptyResult(t *testing.T) {
	out, err := ParseCandidates(`{"items":[]}`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected nothing, got %+v", out)
	}
}

// Better an error the caller can report than a silent empty result that reads
// as "this handout had no deadlines".
func TestParseCandidatesRejectsNonJSON(t *testing.T) {
	if _, err := ParseCandidates("すみません、読み取れませんでした"); err == nil {
		t.Fatal("expected an error for output with no JSON in it")
	}
}

// A task that claims to be an unknown type is filed as a task rather than
// dropped: the date is the part that matters.
func TestParseCandidatesDefaultsToTask(t *testing.T) {
	out, err := ParseCandidates(`{"items":[{"type":"???","title":"何か","date":"2026-08-20"}]}`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(out) != 1 || out[0].Type != "task" {
		t.Fatalf("expected one task, got %+v", out)
	}
}
