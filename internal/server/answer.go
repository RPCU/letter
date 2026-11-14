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

	"github.com/Banh-Canh/letter/internal/config"
)

const (
	// answerBurst answers at once, then answerPerMinute, across every client:
	// the notes are written on one Claude login.
	answerBurst     = 20
	answerPerMinute = 8
	// answerSlots notes are written at the same time at most.
	answerSlots = 2
	// maxNoes bounds the count the page sends; maxNotes and maxNoteText the
	// earlier notes it sends back so Claude doesn't repeat itself.
	maxNoes      = 1000
	maxNotes     = 12
	maxNoteText  = 1500
	maxDealText  = 300
	writeTimeout = 2 * time.Minute
	// awayFor is how long canned notes stand in after Claude fails, before
	// it's tried again.
	awayFor = time.Minute
	// fallbackTyping is the pause before a canned note, as if written.
	fallbackTyping = 1200 * time.Millisecond
	// cloudMark starts the cloud's aside at the end of a note.
	cloudMark   = "☁"
	maxCloudLen = 120
)

// The kinds of answer the page sends.
const (
	kindYes = "yes"
	kindNo  = "no"
	// kindWhy is a reason tapped after a no ("Why not?").
	kindWhy = "why"
	// kindOffer asks for three offers to make it up to them, kindDeal
	// accepts one.
	kindOffer = "offer"
	kindDeal  = "deal"
)

type answerRequest struct {
	// Answer is one of the kinds above.
	Answer string `json:"answer"`
	// Noes is how many times they said no before this answer.
	Noes int `json:"noes"`
	// Notes are the notes already on the page, oldest first.
	Notes []string `json:"notes"`
	// Seed is drawn once per visit and orders the no notes' forms.
	Seed uint32 `json:"seed"`
	// Reason is the tapped reason's index (why), Round the bargain's round
	// (offer), Deal the accepted offer (deal, and yes after one).
	Reason int    `json:"reason"`
	Round  int    `json:"round"`
	Deal   string `json:"deal"`
}

// answerEvent is one line of the streamed note (application/x-ndjson). The
// last one has Done, and after a yes the gift if there is one. Cloud is the
// cloud's aside, sent apart from the note just before Done.
type answerEvent struct {
	Text  string `json:"text,omitempty"`
	Cloud string `json:"cloud,omitempty"`
	Done  bool   `json:"done,omitempty"`
	Error string `json:"error,omitempty"`
	Gift  string `json:"gift,omitempty"`
	Link  bool   `json:"link,omitempty"`
	Intro string `json:"intro,omitempty"`
}

// errAway means Claude Code ran but couldn't write (not logged in, usage
// limit…); the details go to the log, not to the page.
var errAway = errors.New("claude away")

// handleAnswer takes their answer and streams back the note that follows.
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
	if code, err := s.validAnswer(req); err != nil {
		writeJSONError(w, code, err.Error())
		return
	}
	req.Deal = oneLine(req.Deal)
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
	s.noteAnswer(req)

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
	yes := req.Answer == kindYes
	done := answerEvent{Done: true}
	if yes && s.cfg.Gift != "" {
		done.Gift, done.Link, done.Intro = s.cfg.Gift, isLink(s.cfg.Gift), s.cfg.GiftIntro
	}

	if !s.cfg.AI || s.away() {
		s.fallback(r.Context(), req, send, done)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), writeTimeout)
	defer cancel()
	split := &cloudSplit{}
	err := s.write(ctx, s.systemPrompt(), s.answerPrompt(req), func(delta string) {
		if text := split.add(delta); text != "" {
			send(answerEvent{Text: text})
		}
	})
	switch {
	case err == nil:
		if c := split.cloud(); c != "" && s.hasCloud(req.Answer) {
			send(answerEvent{Cloud: c})
		}
		send(done)
		s.remember(req.Answer, split.note())
	case r.Context().Err() != nil:
		// The page went away: nobody to tell.
	default:
		if !errors.Is(err, errAway) {
			s.log.Warn("writing the note failed", "error", err)
		}
		s.markAway()
		if split.note() != "" {
			// Cut off mid-note: a canned one wouldn't follow on.
			send(answerEvent{Error: "away"})
			if yes {
				// The gift still comes with the yes.
				send(done)
			}
			return
		}
		s.fallback(r.Context(), req, send, done)
	}
}

// validAnswer checks the request against what the page can send, with the
// status to answer when it can't.
func (s *Server) validAnswer(req answerRequest) (int, error) {
	bad := func(format string, a ...any) (int, error) { return http.StatusBadRequest, fmt.Errorf(format, a...) }
	if req.Noes < 0 || req.Noes > maxNoes {
		return bad("noes: 0 to %d", maxNoes)
	}
	if len(req.Notes) > maxNotes {
		return bad("notes: at most %d", maxNotes)
	}
	for i, n := range req.Notes {
		if utf8.RuneCountInString(n) > maxNoteText {
			return bad("note %d: at most %d characters", i+1, maxNoteText)
		}
	}
	if utf8.RuneCountInString(req.Deal) > maxDealText {
		return bad("deal: at most %d characters", maxDealText)
	}
	c := s.cfg
	switch req.Answer {
	case kindYes:
	case kindNo:
		if req.Noes >= len(c.NoLabels) {
			// The no button is gone by now: only yes is left.
			return http.StatusConflict, errors.New("only yes is left")
		}
	case kindWhy:
		if !c.AskWhy || req.Noes < 1 {
			return http.StatusConflict, errors.New("no reason to give")
		}
		if req.Reason < 0 || req.Reason >= len(c.Reasons) {
			return bad("reason: 0 to %d", len(c.Reasons)-1)
		}
	case kindOffer:
		if !c.Bargain {
			return http.StatusConflict, errors.New("no bargaining")
		}
		if req.Round < 0 || req.Round >= config.BargainRounds {
			return bad("round: 0 to %d", config.BargainRounds-1)
		}
	case kindDeal:
		if !c.Bargain {
			return http.StatusConflict, errors.New("no bargaining")
		}
		if strings.TrimSpace(req.Deal) == "" {
			return bad("deal: the accepted offer")
		}
	default:
		return bad(`answer: "yes", "no", "why", "offer" or "deal"`)
	}
	return 0, nil
}

// hasCloud reports whether the cloud comments on this kind of note: not the
// yes (it drifts away) nor the offers (a list).
func (s *Server) hasCloud(kind string) bool {
	return s.cfg.Cloud && (kind == kindNo || kind == kindWhy || kind == kindDeal)
}

// cloudSplit splits Claude's stream into the note and the cloud's aside on
// its last line (after cloudMark), so the aside never shows in the note.
type cloudSplit struct {
	full strings.Builder
	sent int
}

// add takes the next piece of the stream and returns the note's new text.
func (c *cloudSplit) add(delta string) string {
	c.full.WriteString(delta)
	note := c.visible()
	out := note[min(c.sent, len(note)):]
	c.sent = len(note)
	return out
}

func (c *cloudSplit) visible() string {
	full := c.full.String()
	if i := strings.Index(full, cloudMark); i >= 0 {
		return full[:i]
	}
	return full
}

func (c *cloudSplit) note() string { return strings.TrimSpace(c.visible()) }

func (c *cloudSplit) cloud() string {
	full := c.full.String()
	i := strings.Index(full, cloudMark)
	if i < 0 {
		return ""
	}
	line, _, _ := strings.Cut(strings.TrimSpace(full[i+len(cloudMark):]), "\n")
	return truncate(strings.Trim(strings.TrimSpace(line), `"“”`), maxCloudLen)
}

// fallback sends the canned note for this answer after a short pause, as if
// written: the page still answers without Claude.
func (s *Server) fallback(ctx context.Context, req answerRequest, send func(answerEvent), done answerEvent) {
	select {
	case <-ctx.Done():
		return
	case <-time.After(s.typing):
	}
	c := s.cfg
	var note string
	switch req.Answer {
	case kindYes:
		note = c.FallbackYes
	case kindNo:
		note = c.FallbackNo[min(req.Noes, len(c.FallbackNo)-1)]
	case kindWhy:
		note = c.FallbackWhy[min(req.Reason, len(c.FallbackWhy)-1)]
	case kindDeal:
		note = c.FallbackDeal
	case kindOffer:
		lines := make([]string, 3)
		for i := range lines {
			lines[i] = "— " + c.FallbackOffers[(req.Round*3+i)%len(c.FallbackOffers)]
		}
		note = strings.Join(lines, "\n")
	}
	send(answerEvent{Text: strings.ReplaceAll(note, "{name}", c.Name)})
	if s.hasCloud(req.Answer) {
		send(answerEvent{Cloud: c.FallbackCloud[s.pick(len(c.FallbackCloud))]})
	}
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

// oneLine collapses s's whitespace onto one line.
func oneLine(s string) string { return strings.Join(strings.Fields(s), " ") }
