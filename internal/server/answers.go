package server

import (
	"fmt"
	"os"
	"path/filepath"
)

// noteAnswer adds their answer to LETTER_ANSWERS' file next to the Claude
// login, for the page's owner: never cleared, never served.
func (s *Server) noteAnswer(yes bool, noes int) {
	what := fmt.Sprintf("no (%q, no number %d)", s.cfg.NoLabel(noes), noes+1)
	if yes {
		what = fmt.Sprintf("**yes** (%q, after %d no)", s.cfg.Yes, noes)
	}
	s.log.Info("answer", "yes", yes, "noes", noes)
	if s.cfg.AnswersPath == "" {
		return
	}
	s.answersMu.Lock()
	defer s.answersMu.Unlock()
	if err := os.MkdirAll(filepath.Dir(s.cfg.AnswersPath), 0o700); err != nil {
		s.log.Warn("note the answer", "error", err)
		return
	}
	f, err := os.OpenFile(s.cfg.AnswersPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		s.log.Warn("note the answer", "error", err)
		return
	}
	defer func() { _ = f.Close() }()
	when := s.now().In(s.cfg.Location).Format("2006-01-02 15:04 MST")
	if _, err := fmt.Fprintf(f, "- %s: %s\n", when, what); err != nil {
		s.log.Warn("note the answer", "error", err)
	}
}
