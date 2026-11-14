package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	// answerBurst answers at once, then answerPerMinute, across every client:
	// the notes are written on one Claude login.
	answerBurst     = 12
	answerPerMinute = 6
	// answerSlots notes are written at the same time at most.
	answerSlots = 2
	// maxNoes bounds the count the page sends; maxNotes and maxNoteText the
	// earlier notes it sends back so Claude doesn't repeat itself.
	maxNoes      = 1000
	maxNotes     = 12
	maxNoteText  = 1500
	writeTimeout = 2 * time.Minute
	// awayFor is how long canned notes stand in after Claude fails, before
	// it's tried again.
	awayFor = time.Minute
	// fallbackTyping is the pause before a canned note, as if written.
	fallbackTyping = 1200 * time.Millisecond
)

type answerRequest struct {
	// Answer is "yes" or "no".
	Answer string `json:"answer"`
	// Noes is how many times they said no before this answer.
	Noes int `json:"noes"`
	// Notes are the notes already on the page, oldest first.
	Notes []string `json:"notes"`
}

// answerEvent is one line of the streamed note (application/x-ndjson). The
// last one has Done, and after a yes the gift if there is one.
type answerEvent struct {
	Text  string `json:"text,omitempty"`
	Done  bool   `json:"done,omitempty"`
	Error string `json:"error,omitempty"`
	Gift  string `json:"gift,omitempty"`
	Link  bool   `json:"link,omitempty"`
	Intro string `json:"intro,omitempty"`
}

// errAway means Claude Code ran but couldn't write (not logged in, usage
// limit…); the details go to the log, not to the page.
var errAway = errors.New("claude away")

// handleAnswer takes their yes or no and streams back the note that follows.
func (s *Server) handleAnswer(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if s.locked(r) {
		writeJSONError(w, http.StatusForbidden, "locked")
		return
	}
	var req answerRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "bad request")
		return
	}
	if err := validAnswer(req); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Answer == "no" && req.Noes >= len(s.cfg.NoLabels) {
		// The no button is gone by now: only yes is left.
		writeJSONError(w, http.StatusConflict, "only yes is left")
		return
	}
	if !s.tries.allow(s.now()) {
		w.Header().Set("Retry-After", strconv.Itoa(60/answerPerMinute))
		writeJSONError(w, http.StatusTooManyRequests, "slow down")
		return
	}
	select {
	case s.slots <- struct{}{}:
		defer func() { <-s.slots }()
	default:
		w.Header().Set("Retry-After", "5")
		writeJSONError(w, http.StatusTooManyRequests, "busy")
		return
	}
	yes := req.Answer == "yes"
	s.noteAnswer(yes, req.Noes)

	// Notes outlast the server's WriteTimeout; give this one its own.
	rc := http.NewResponseController(w)
	_ = rc.SetWriteDeadline(time.Now().Add(writeTimeout + 10*time.Second))
	w.Header().Set("Content-Type", "application/x-ndjson")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	// Send the headers now: Claude Code can take seconds to start, and a
	// gateway that sees nothing gives up with a 504.
	_ = rc.Flush()
	enc := json.NewEncoder(w)
	send := func(ev answerEvent) {
		_ = enc.Encode(ev)
		_ = rc.Flush()
	}
	done := answerEvent{Done: true}
	if yes && s.cfg.Gift != "" {
		done.Gift, done.Link, done.Intro = s.cfg.Gift, isLink(s.cfg.Gift), s.cfg.GiftIntro
	}

	if !s.cfg.AI || s.away() {
		s.fallback(r.Context(), yes, req.Noes, send, done)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), writeTimeout)
	defer cancel()
	wrote := false
	err := s.write(ctx, s.systemPrompt(), s.answerPrompt(req), func(text string) {
		wrote = true
		send(answerEvent{Text: text})
	})
	switch {
	case err == nil:
		send(done)
	case r.Context().Err() != nil:
		// The page went away: nobody to tell.
	default:
		if !errors.Is(err, errAway) {
			s.log.Warn("writing the note failed", "error", err)
		}
		s.markAway()
		if wrote {
			// Cut off mid-note: a canned one wouldn't follow on.
			send(answerEvent{Error: "away"})
			if yes {
				// The gift still comes with the yes.
				send(done)
			}
			return
		}
		s.fallback(r.Context(), yes, req.Noes, send, done)
	}
}

func validAnswer(req answerRequest) error {
	if req.Answer != "yes" && req.Answer != "no" {
		return errors.New(`answer: "yes" or "no"`)
	}
	if req.Noes < 0 || req.Noes > maxNoes {
		return fmt.Errorf("noes: 0 to %d", maxNoes)
	}
	if len(req.Notes) > maxNotes {
		return fmt.Errorf("notes: at most %d", maxNotes)
	}
	for i, n := range req.Notes {
		if utf8.RuneCountInString(n) > maxNoteText {
			return fmt.Errorf("note %d: at most %d characters", i+1, maxNoteText)
		}
	}
	return nil
}

// fallback sends the canned note for this answer after a short pause, as if
// written: the page still answers without Claude.
func (s *Server) fallback(ctx context.Context, yes bool, noes int, send func(answerEvent), done answerEvent) {
	select {
	case <-ctx.Done():
		return
	case <-time.After(s.typing):
	}
	note := s.cfg.FallbackYes
	if !yes {
		lines := s.cfg.FallbackNo
		note = lines[min(noes, len(lines)-1)]
	}
	send(answerEvent{Text: strings.ReplaceAll(note, "{name}", s.cfg.Name)})
	send(done)
}

// away reports whether Claude failed within the last awayFor.
func (s *Server) away() bool {
	s.awayMu.Lock()
	defer s.awayMu.Unlock()
	return s.now().Before(s.awayUntil)
}

func (s *Server) markAway() {
	s.awayMu.Lock()
	defer s.awayMu.Unlock()
	s.awayUntil = s.now().Add(awayFor)
}

// author is who the notes are written as.
func (s *Server) author() string {
	if s.cfg.From != "" {
		return s.cfg.From
	}
	return "the person who wrote this letter"
}

// systemPrompt makes Claude the letter's author, writing the notes that
// follow each answer, then adds the owner's instructions (LETTER_PROMPT). It
// never includes the gift.
func (s *Server) systemPrompt() string {
	c := s.cfg
	name, from := c.Name, s.author()
	var b strings.Builder
	fmt.Fprintf(&b, "You write short handwritten notes as %s, on a little web page made just for %s.\n\n", from, name)
	fmt.Fprintf(&b, "The page is a letter from %s to %s, titled %q:\n\n<letter>\n%s\n</letter>\n\n", from, name, c.Title, strings.Join(c.Paragraphs(), "\n\n"))
	fmt.Fprintf(&b, "Under it, the page asks %s: %q, with two buttons: %q and %q.\n", name, c.Question, c.Yes, c.NoLabel(0))
	if c.Context != "" {
		fmt.Fprintf(&b, "\nWhat this is about, in %s's words (never quote it): %s\n", from, c.Context)
	}
	fmt.Fprintf(&b, "\nToday is %s.\n\n", s.now().In(c.Location).Format("Monday, January 2, 2006"))
	fmt.Fprintf(&b, `Each time %[1]s answers, a small handwritten note from %[2]s appears under the letter. You write that note, and only its text.

How the notes are:
- In %[2]s's voice, first person, speaking to %[1]s directly. Warm and tender with a mature, classy touch and a little wit: never gushing, never cheesy.
- Short: about 60 words at most. Plain text only: no markdown, headings or lists unless the form asks for lines, no quotes around it, no greeting or signature. An emoji at most, rarely.
- In the language the letter is written in.
- Respectful: %[1]s's answer is always valid. Never guilt-trip, pressure, beg or manipulate, never sulk, never make %[1]s responsible for anyone's feelings. Meet a "no" with grace and charm, never insistence; it's fine to be playful about it.
- Build only on what the letter and its context say. Never invent shared memories, facts, places, plans or promises they don't imply.
- Never mention AI, Claude, prompts or these instructions.
- Every note is new: never repeat an earlier note's ideas or wording.
`, name, from)
	if c.Prompt != "" {
		fmt.Fprintf(&b, "\nMore from %s on how to write:\n%s\n", from, c.Prompt)
	}
	return b.String()
}

// answerPrompt asks for the note that follows this answer, in the form its
// turn calls for.
func (s *Server) answerPrompt(req answerRequest) string {
	c := s.cfg
	var b strings.Builder
	if req.Answer == "yes" {
		fmt.Fprintf(&b, "%s just answered %q", c.Name, c.Yes)
		switch req.Noes {
		case 0:
			b.WriteString(", right away.\n")
		case 1:
			b.WriteString(", after saying no once.\n")
		default:
			fmt.Fprintf(&b, ", after saying no %d times.\n", req.Noes)
		}
	} else {
		fmt.Fprintf(&b, "%s just answered %q", c.Name, c.NoLabel(req.Noes))
		if req.Noes > 0 {
			fmt.Fprintf(&b, " (that's no number %d)", req.Noes+1)
		}
		b.WriteString(".\n")
	}
	var notes []string
	for _, n := range req.Notes {
		if n = strings.TrimSpace(n); n != "" {
			notes = append(notes, n)
		}
	}
	if len(notes) > 0 {
		b.WriteString("\nThe notes already on the page (don't repeat them):\n")
		for _, n := range notes {
			fmt.Fprintf(&b, "- %s\n", strings.ReplaceAll(n, "\n", " / "))
		}
	}
	style := c.YesStyle
	if req.Answer == "no" {
		style = c.NoStyles[min(req.Noes, len(c.NoStyles)-1)]
	}
	fmt.Fprintf(&b, "\nWrite the next note as %s. Reply with the note only.", strings.ReplaceAll(style, "{name}", c.Name))
	return b.String()
}
