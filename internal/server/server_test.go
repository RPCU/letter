package server

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Banh-Canh/letter/internal/config"
)

var fixedNow = time.Date(2026, time.November, 14, 20, 30, 0, 0, time.UTC)

// newServer builds a server on the default config, changed by edit, with no
// Claude (tests swap s.write) and no typing pause.
func newServer(t *testing.T, edit ...func(*config.Config)) *Server {
	t.Helper()
	env := map[string]string{"LETTER_NAME": "Chihiro", "LETTER_FROM": "Victor", "LETTER_TIMEZONE": "Europe/Paris"}
	cfg, err := config.Load(func(k string) string { return env[k] })
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range edit {
		e(&cfg)
	}
	s, err := New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	s.now = func() time.Time { return fixedNow }
	s.typing = 0
	s.write = func(context.Context, string, string, func(string)) error {
		t.Error("Claude ran unexpectedly")
		return nil
	}
	return s
}

func get(t *testing.T, s *Server, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func post(t *testing.T, s *Server, body string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/answer", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	s.Handler().ServeHTTP(rec, req)
	return rec
}

// events decodes a streamed reply.
func events(t *testing.T, rec *httptest.ResponseRecorder) (text string, last answerEvent) {
	t.Helper()
	sc := bufio.NewScanner(rec.Body)
	for sc.Scan() {
		var ev answerEvent
		if err := json.Unmarshal(sc.Bytes(), &ev); err != nil {
			t.Fatalf("bad line %q: %v", sc.Text(), err)
		}
		text += ev.Text
		last = ev
	}
	return text, last
}

func TestPage(t *testing.T) {
	s := newServer(t, func(c *config.Config) {
		c.Gift = "GIFT-CODE-42"
		c.Context = "I forgot our dinner"
		c.Theme = "ocean"
	})
	rec := get(t, s, "/")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{
		"A letter for Chihiro", "Dear Chihiro,", "— Victor", "I’m sorry", "Do you forgive me?",
		"theme-ocean", "November 14, 2026", `class="gift"`, "Still no",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("page lacks %q", want)
		}
	}
	// The gift comes only with the yes, and the context is for Claude only.
	for _, never := range []string{"GIFT-CODE-42", "I forgot our dinner"} {
		if strings.Contains(body, never) {
			t.Errorf("page leaks %q", never)
		}
	}
	if csp := rec.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "script-src 'self'") {
		t.Errorf("CSP %q", csp)
	}
}

func TestPageWithoutGift(t *testing.T) {
	if strings.Contains(get(t, newServer(t), "/").Body.String(), `class="gift"`) {
		t.Error("gift block without a gift")
	}
}

func TestStaticAndHealth(t *testing.T) {
	s := newServer(t)
	for _, p := range []string{"/static/css/letter.css", "/static/js/letter.js", "/static/favicon.svg", "/healthz"} {
		if rec := get(t, s, p); rec.Code != http.StatusOK {
			t.Errorf("%s: status %d", p, rec.Code)
		}
	}
	if !strings.Contains(get(t, s, "/").Body.String(), "letter.css?v="+s.assetVersion) {
		t.Error("CSS not fingerprinted")
	}
}

func TestNoWritesANoteInItsTurnsForm(t *testing.T) {
	s := newServer(t)
	var system, prompt string
	s.write = func(_ context.Context, sys, p string, emit func(string)) error {
		system, prompt = sys, p
		emit("Rain, ")
		emit("then sun.")
		return nil
	}
	rec := post(t, s, `{"answer":"no","noes":1,"notes":["first note"]}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	text, last := events(t, rec)
	if text != "Rain, then sun." || !last.Done || last.Gift != "" {
		t.Errorf("got %q, last %+v", text, last)
	}
	for _, want := range []string{"Still no", "no number 2", "haiku", "first note"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt lacks %q:\n%s", want, prompt)
		}
	}
	for _, want := range []string{"as Victor", "Chihiro", "Never guilt-trip"} {
		if !strings.Contains(system, want) {
			t.Errorf("system prompt lacks %q", want)
		}
	}
}

func TestYesBringsTheGift(t *testing.T) {
	s := newServer(t, func(c *config.Config) { c.Gift = "https://example.com/gift" })
	var system, prompt string
	s.write = func(_ context.Context, sys, p string, emit func(string)) error {
		system, prompt = sys, p
		emit("Thank you.\n— I'll listen.")
		return nil
	}
	text, last := events(t, post(t, s, `{"answer":"yes","noes":3}`))
	if text == "" || !last.Done || last.Gift != "https://example.com/gift" || !last.Link || last.Intro == "" {
		t.Errorf("got %q, last %+v", text, last)
	}
	if !strings.Contains(prompt, "after saying no 3 times") || !strings.Contains(prompt, "promises") {
		t.Errorf("prompt: %s", prompt)
	}
	if strings.Contains(system+prompt, "example.com") {
		t.Error("Claude was told the gift")
	}
}

func TestFallbackWhenClaudeIsAway(t *testing.T) {
	s := newServer(t)
	calls := 0
	s.write = func(context.Context, string, string, func(string)) error {
		calls++
		return errAway
	}
	text, last := events(t, post(t, s, `{"answer":"no","noes":2}`))
	if !strings.Contains(text, "Court of Chihiro") || !last.Done {
		t.Errorf("got %q, %+v", text, last)
	}
	// Claude isn't tried again for a minute.
	events(t, post(t, s, `{"answer":"no","noes":3}`))
	if calls != 1 {
		t.Errorf("Claude tried %d times", calls)
	}
	s.now = func() time.Time { return fixedNow.Add(awayFor + time.Second) }
	events(t, post(t, s, `{"answer":"no","noes":4}`))
	if calls != 2 {
		t.Errorf("Claude not tried again after a minute (%d)", calls)
	}
}

func TestCutOffYesStillGivesTheGift(t *testing.T) {
	s := newServer(t, func(c *config.Config) { c.Gift = "CODE" })
	s.write = func(_ context.Context, _, _ string, emit func(string)) error {
		emit("Thank")
		return errors.New("boom")
	}
	rec := post(t, s, `{"answer":"yes","noes":0}`)
	var evs []answerEvent
	for line := range strings.Lines(rec.Body.String()) {
		var ev answerEvent
		_ = json.Unmarshal([]byte(line), &ev)
		evs = append(evs, ev)
	}
	if !slices.ContainsFunc(evs, func(e answerEvent) bool { return e.Error == "away" }) || evs[len(evs)-1].Gift != "CODE" {
		t.Errorf("events %+v", evs)
	}
}

func TestAIOffUsesTheCannedNotes(t *testing.T) {
	s := newServer(t, func(c *config.Config) { c.AI = false })
	text, _ := events(t, post(t, s, `{"answer":"yes","noes":0}`))
	if text != config.DefaultFallbackYes {
		t.Errorf("got %q", text)
	}
	// Past the last no, the last canned line stays.
	text, _ = events(t, post(t, s, `{"answer":"no","noes":5}`))
	if text != config.DefaultFallbackNo[5] {
		t.Errorf("got %q", text)
	}
}

func TestBadAnswers(t *testing.T) {
	s := newServer(t)
	for body, code := range map[string]int{
		`{"answer":"maybe"}`:        http.StatusBadRequest,
		`{"answer":"no","noes":-1}`: http.StatusBadRequest,
		`not json`:                  http.StatusBadRequest,
		`{"answer":"no","notes":["` + strings.Repeat("a", maxNoteText+1) + `"]}`: http.StatusBadRequest,
		// Six labels: after six noes the no button is gone.
		`{"answer":"no","noes":6}`: http.StatusConflict,
	} {
		if rec := post(t, s, body); rec.Code != code {
			t.Errorf("%.40s: status %d, want %d", body, rec.Code, code)
		}
	}
}

func TestCrossSiteAnswerRejected(t *testing.T) {
	s := newServer(t, func(c *config.Config) { c.AI = false })
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/answer", strings.NewReader(`{"answer":"yes"}`))
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("status %d", rec.Code)
	}
}

func TestAnswersAreThrottled(t *testing.T) {
	s := newServer(t, func(c *config.Config) { c.AI = false })
	var last int
	for range answerBurst + 1 {
		last = post(t, s, `{"answer":"no","noes":0}`).Code
	}
	if last != http.StatusTooManyRequests {
		t.Errorf("status %d after the burst", last)
	}
}

func TestOnlyTwoNotesAtOnce(t *testing.T) {
	s := newServer(t)
	started, finish := make(chan struct{}, answerSlots), make(chan struct{})
	s.write = func(context.Context, string, string, func(string)) error {
		started <- struct{}{}
		<-finish
		return nil
	}
	var wg sync.WaitGroup
	for range answerSlots {
		wg.Go(func() { post(t, s, `{"answer":"no","noes":0}`) })
		<-started
	}
	if rec := post(t, s, `{"answer":"no","noes":0}`); rec.Code != http.StatusTooManyRequests {
		t.Errorf("third note at once: status %d", rec.Code)
	}
	close(finish)
	wg.Wait()
}

func TestAnswersAreNotedForTheOwner(t *testing.T) {
	path := filepath.Join(t.TempDir(), "claude", "letter-answers.md")
	s := newServer(t, func(c *config.Config) { c.AI = false; c.AnswersPath = path })
	events(t, post(t, s, `{"answer":"no","noes":0}`))
	events(t, post(t, s, `{"answer":"yes","noes":1}`))
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := string(b)
	for _, want := range []string{"2026-11-14 21:30 CET: no (\"No\", no number 1)", "**yes** (\"Yes\", after 1 no)"} {
		if !strings.Contains(got, want) {
			t.Errorf("answers lack %q:\n%s", want, got)
		}
	}
}

func TestClaudeIsIsolated(t *testing.T) {
	args := strings.Join(claudeArgs("sonnet", "SYS"), " ")
	for _, want := range []string{"--tools ", "--strict-mcp-config", "--safe-mode", "--no-session-persistence", "--setting-sources ", "--system-prompt SYS"} {
		if !strings.Contains(args, want) {
			t.Errorf("args lack %q", want)
		}
	}
	env := claudeEnv(func(k string) string {
		return map[string]string{"LETTER_GIFT": "x", "HOME": "/h", "CLAUDE_CONFIG_DIR": "/h/.claude"}[k]
	}, "/tmp/d")
	for _, e := range env {
		if strings.HasPrefix(e, "LETTER_") {
			t.Errorf("Claude gets %s", e)
		}
	}
	if !slices.Contains(env, "CLAUDE_CONFIG_DIR=/h/.claude") || !slices.Contains(env, "ENABLE_CLAUDEAI_MCP_SERVERS=false") {
		t.Errorf("env %v", env)
	}
}

func TestClaudeWriteStreamsAFakeCLI(t *testing.T) {
	dir := t.TempDir()
	cli := filepath.Join(dir, "claude")
	script := `#!/bin/sh
cat >/dev/null
echo '{"type":"stream_event","event":{"type":"content_block_delta","delta":{"type":"text_delta","text":"Hello"}}}'
echo '{"type":"result","subtype":"success","is_error":false,"result":"Hello"}'
`
	if err := os.WriteFile(cli, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	s := newServer(t, func(c *config.Config) { c.Command = cli })
	var got string
	if err := s.claudeWrite(context.Background(), "sys", "prompt", func(t string) { got += t }); err != nil || got != "Hello" {
		t.Fatalf("got %q, %v", got, err)
	}

	away := `#!/bin/sh
cat >/dev/null
echo '{"type":"result","subtype":"success","is_error":true,"result":"Not logged in"}'
`
	if err := os.WriteFile(cli, []byte(away), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := s.claudeWrite(context.Background(), "sys", "prompt", func(string) {}); !errors.Is(err, errAway) {
		t.Errorf("err %v, want errAway", err)
	}
}
