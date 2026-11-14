package server

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Banh-Canh/letter/internal/config"
)

// capture makes Claude reply with reply, keeping the prompt it was given.
func capture(s *Server, reply ...string) *string {
	var prompt string
	s.write = func(_ context.Context, _, p string, emit func(string)) error {
		prompt = p
		for _, r := range reply {
			emit(r)
		}
		return nil
	}
	return &prompt
}

func TestEachVisitDrawsTheFormsInItsOwnOrder(t *testing.T) {
	s := newServer(t)
	order := func(seed uint32) string {
		var out []string
		for noes := range len(s.cfg.NoLabels) - 1 {
			out = append(out, s.noStyle(seed, noes))
		}
		return strings.Join(out, "|")
	}
	if order(1) != order(1) {
		t.Error("the same visit changed order")
	}
	if order(1) == order(2) && order(2) == order(3) {
		t.Error("every visit gets the same order")
	}
	if got := s.noStyle(7, len(s.cfg.NoLabels)-1); got != config.DefaultNoLastStyle {
		t.Errorf("last no: %q", got)
	}
}

func TestTheCloudGetsTheLastWord(t *testing.T) {
	s := newServer(t)
	prompt := capture(s, "Rain, then sun.\n", "☁ Don't mind me", ", I'm only weather.")
	rec := post(t, s, `{"answer":"no","noes":0,"seed":5}`)
	body := rec.Body.String()
	text, last := events(t, rec)
	if strings.Contains(text, "☁") || strings.TrimSpace(text) != "Rain, then sun." || !last.Done {
		t.Errorf("note %q", text)
	}
	if !strings.Contains(body, `"cloud":"Don't mind me, I'm only weather."`) {
		t.Errorf("no aside: %s", body)
	}
	for _, want := range []string{"Mood: ", "Length: ", "this image: ", "☁ "} {
		if !strings.Contains(*prompt, want) {
			t.Errorf("prompt lacks %q:\n%s", want, *prompt)
		}
	}

	// The yes has no aside: the cloud is leaving.
	s.cfg.AI = false
	if strings.Contains(post(t, s, `{"answer":"yes"}`).Body.String(), `"cloud"`) {
		t.Error("aside on the yes")
	}
	if !strings.Contains(post(t, s, `{"answer":"no","noes":1}`).Body.String(), `"cloud"`) {
		t.Error("no canned aside")
	}
}

func TestWhyNot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "letter-answers.md")
	s := newServer(t, func(c *config.Config) { c.AnswersPath = path })
	prompt := capture(s, "Then I'll wait.")
	if rec := post(t, s, `{"answer":"why","noes":0,"reason":0}`); rec.Code != http.StatusConflict {
		t.Errorf("why before any no: status %d", rec.Code)
	}
	if rec := post(t, s, `{"answer":"why","noes":1,"reason":9}`); rec.Code != http.StatusBadRequest {
		t.Errorf("unknown reason: status %d", rec.Code)
	}
	text, _ := events(t, post(t, s, `{"answer":"why","noes":2,"reason":1}`))
	if text != "Then I'll wait." || !strings.Contains(*prompt, `tapped: "Still a bit upset"`) {
		t.Errorf("note %q, prompt:\n%s", text, *prompt)
	}
	if b, _ := os.ReadFile(path); !strings.Contains(string(b), `why not: "Still a bit upset"`) {
		t.Errorf("answers: %s", b)
	}
}

func TestBargain(t *testing.T) {
	path := filepath.Join(t.TempDir(), "letter-answers.md")
	s := newServer(t, func(c *config.Config) { c.AI = false; c.AnswersPath = path })
	text, _ := events(t, post(t, s, `{"answer":"offer","round":1}`))
	lines := strings.Split(text, "\n")
	if len(lines) != 3 || !strings.HasPrefix(lines[0], "— ") || lines[0] != "— "+config.DefaultFallbackOffers[3] {
		t.Errorf("offers %q", lines)
	}
	if rec := post(t, s, `{"answer":"offer","round":3}`); rec.Code != http.StatusBadRequest {
		t.Errorf("fourth round: status %d", rec.Code)
	}
	if rec := post(t, s, `{"answer":"deal","deal":"  "}`); rec.Code != http.StatusBadRequest {
		t.Errorf("empty deal: status %d", rec.Code)
	}
	events(t, post(t, s, `{"answer":"deal","deal":"Breakfast in bed,\ncroissants included."}`))
	if b, _ := os.ReadFile(path); !strings.Contains(string(b), `**deal**: "Breakfast in bed, croissants included."`) {
		t.Errorf("answers: %s", b)
	}

	s.cfg.AI = true
	prompt := capture(s, "Thank you.")
	events(t, post(t, s, `{"answer":"yes","noes":2,"deal":"Breakfast in bed"}`))
	if !strings.Contains(*prompt, `accepted this offer: "Breakfast in bed"`) {
		t.Errorf("yes prompt lacks the deal:\n%s", *prompt)
	}

	s.cfg.Bargain = false
	if rec := post(t, s, `{"answer":"offer"}`); rec.Code != http.StatusConflict {
		t.Errorf("offer with LETTER_BARGAIN=false: status %d", rec.Code)
	}
}

func TestLaterVisitsKnowTheEarlierNotes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "letter-notes.json")
	s := newServer(t, func(c *config.Config) { c.NotesPath = path })
	capture(s, "A lighthouse, blinking sorry.\n☁ Subtle.")
	events(t, post(t, s, `{"answer":"no","noes":0}`))

	// Another replica, or a restart, reads them from the volume.
	other := newServer(t, func(c *config.Config) { c.NotesPath = path })
	prompt := capture(other, "Something new.")
	events(t, post(t, other, `{"answer":"no","noes":0}`))
	if !strings.Contains(*prompt, "earlier visits") || !strings.Contains(*prompt, "- A lighthouse, blinking sorry.\n") || strings.Contains(*prompt, "Subtle") {
		t.Errorf("prompt:\n%s", *prompt)
	}
}
