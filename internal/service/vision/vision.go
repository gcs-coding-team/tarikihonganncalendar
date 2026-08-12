// Package vision reads a photographed handout and pulls the dates out of it.
//
// It talks to an Ollama server (OLLAMA_BASE_URL / OLLAMA_MODEL), which is where
// the project's own configuration already pointed. Nothing here invents data: if
// no model is reachable, Analyse says so and the caller reports the failure. A
// handout is turned into deadlines a student then relies on, and a confidently
// wrong due date is worse than an honest "could not read it".
package vision

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gcs-coding-team/tarikihonganncalendar/internal/repository"
)

// ErrUnavailable means no model answered. It is not a failure of the image.
var ErrUnavailable = errors.New("vision model unavailable")

type Config struct {
	BaseURL string
	Model   string
	Timeout time.Duration
}

type Analyser struct {
	cfg    Config
	client *http.Client
}

func New(cfg Config) *Analyser {
	if cfg.Timeout <= 0 {
		cfg.Timeout = 180 * time.Second
	}
	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")
	return &Analyser{cfg: cfg, client: &http.Client{Timeout: cfg.Timeout}}
}

// Available reports whether a model can be reached right now, so the API can say
// up front that analysis will not work rather than after an upload.
func (a *Analyser) Available(ctx context.Context) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.cfg.BaseURL+"/api/tags", nil)
	if err != nil {
		return false
	}
	res, err := a.client.Do(req)
	if err != nil {
		return false
	}
	defer res.Body.Close()
	io.Copy(io.Discard, res.Body)
	return res.StatusCode == http.StatusOK
}

// The model is asked for JSON and nothing else. Handouts are Japanese, and the
// year is usually missing from them, so "today" is supplied for resolving
// things like 「8/20まで」.
const promptTemplate = `あなたは学校のプリントを読み取る係です。
画像から「予定」と「タスク（提出物・宿題）」を抜き出してください。

今日は %s です。年が書かれていない日付は、今日以降で最も近い日付として解釈してください。

次の形式の JSON のみを出力してください。説明文は不要です。

{"items":[{"type":"task","title":"数学プリント p.24","date":"2026-08-20"},
          {"type":"event","title":"保護者会","date":"2026-08-25","time":"10:00"}]}

規則:
- type は "task"（提出物・宿題・持ち物）か "event"（行事・集会・時間の決まった予定）
- date は YYYY-MM-DD
- time は event のみ。分からなければ省略する
- 日付が読み取れないものは出力しない
- 何も見つからなければ {"items":[]}`

type ollamaRequest struct {
	Model  string   `json:"model"`
	Prompt string   `json:"prompt"`
	Images []string `json:"images"`
	Stream bool     `json:"stream"`
	Format string   `json:"format"`
}

type ollamaResponse struct {
	Response string `json:"response"`
	Error    string `json:"error"`
}

// Analyse returns the candidates found in the image. The caller shows them for
// confirmation — nothing is committed on the strength of this alone.
func (a *Analyser) Analyse(ctx context.Context, image []byte) ([]repository.Candidate, error) {
	body, err := json.Marshal(ollamaRequest{
		Model:  a.cfg.Model,
		Prompt: fmt.Sprintf(promptTemplate, time.Now().Format("2006-01-02")),
		Images: []string{base64.StdEncoding.EncodeToString(image)},
		Stream: false,
		Format: "json",
	})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		a.cfg.BaseURL+"/api/generate", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	res, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: status %d", ErrUnavailable, res.StatusCode)
	}
	var out ollamaResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	if out.Error != "" {
		return nil, fmt.Errorf("%w: %s", ErrUnavailable, out.Error)
	}
	return ParseCandidates(out.Response)
}

var jsonObject = regexp.MustCompile(`(?s)\{.*\}`)
var dateOnly = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
var timeOnly = regexp.MustCompile(`^\d{2}:\d{2}$`)

// ParseCandidates pulls the item list out of whatever the model returned. Even
// asked for JSON, models wrap it in prose often enough that finding the object
// is worth doing; anything malformed or undated is dropped rather than guessed.
func ParseCandidates(s string) ([]repository.Candidate, error) {
	match := jsonObject.FindString(s)
	if match == "" {
		return nil, fmt.Errorf("no JSON in model output")
	}
	var parsed struct {
		Items []struct {
			Type  string `json:"type"`
			Title string `json:"title"`
			Date  string `json:"date"`
			Time  string `json:"time"`
		} `json:"items"`
	}
	if err := json.Unmarshal([]byte(match), &parsed); err != nil {
		return nil, fmt.Errorf("decode items: %w", err)
	}
	out := make([]repository.Candidate, 0, len(parsed.Items))
	for _, it := range parsed.Items {
		title := strings.TrimSpace(it.Title)
		if title == "" || !dateOnly.MatchString(it.Date) {
			continue
		}
		if _, err := time.Parse("2006-01-02", it.Date); err != nil {
			continue
		}
		c := repository.Candidate{Type: "task", Title: title, Date: it.Date}
		if it.Type == "event" {
			c.Type = "event"
			if timeOnly.MatchString(it.Time) {
				c.Time = it.Time
			}
		}
		out = append(out, c)
	}
	return out, nil
}
