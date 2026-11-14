package server

import (
	"fmt"
	"os"
	"path/filepath"
)

// noteAnswer adds their answer to LETTER_ANSWERS' file next to the Claude
// login, for the page's owner: never cleared, never served.
func (s *Server) noteAnswer(req answerRequest) {
	c := s.cfg
	var what string
	switch req.Answer {
	case kindYes:
		what = fmt.Sprintf("**yes** (%q, after %d no)", c.Yes, req.Noes)
		if req.Deal != "" {
			what += fmt.Sprintf(", with the deal %q", req.Deal)
		}
	case kindNo:
		what = fmt.Sprintf("no (%q, no number %d)", c.NoLabel(req.Noes), req.Noes+1)
	case kindWhy:
		what = fmt.Sprintf("why not: %q", c.Reasons[req.Reason])
	case kindOffer:
		what = fmt.Sprintf("asked for an offer (round %d)", req.Round+1)
	case kindDeal:
		what = fmt.Sprintf("**deal**: %q", req.Deal)
	}
	s.log.Info("answer", "kind", req.Answer, "noes", req.Noes)
	if c.AnswersPath == "" {
		return
	}
	s.answersMu.Lock()
	defer s.answersMu.Unlock()
	if err := os.MkdirAll(filepath.Dir(c.AnswersPath), 0o700); err != nil {
		s.log.Warn("note the answer", "error", err)
		return
	}
	f, err := os.OpenFile(c.AnswersPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		s.log.Warn("note the answer", "error", err)
		return
	}
	defer func() { _ = f.Close() }()
	when := s.now().In(c.Location).Format("2006-01-02 15:04 MST")
	if _, err := fmt.Fprintf(f, "- %s: %s\n", when, what); err != nil {
		s.log.Warn("note the answer", "error", err)
	}
}
