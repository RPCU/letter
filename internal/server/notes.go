package server

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// keepNotes is how many of the last notes are kept for later visits.
const keepNotes = 30

// earlierNotes returns the last notes written, oldest first: from
// the notes file next to the Claude login when there is one (shared by
// replicas on the volume), from memory otherwise.
func (s *Server) earlierNotes() []string {
	s.notesMu.Lock()
	defer s.notesMu.Unlock()
	s.loadNotes()
	return append([]string(nil), s.recent...)
}

// remember keeps a note Claude wrote, so later visits don't repeat it. The
// offers aren't kept: they're a list, not a note.
func (s *Server) remember(kind, note string) {
	if note = strings.TrimSpace(note); note == "" || kind == kindOffer {
		return
	}
	s.notesMu.Lock()
	defer s.notesMu.Unlock()
	s.loadNotes()
	s.recent = append(s.recent, note)
	s.recent = s.recent[max(0, len(s.recent)-keepNotes):]
	if s.cfg.NotesPath == "" {
		return
	}
	b, err := json.Marshal(s.recent)
	if err == nil {
		err = os.MkdirAll(filepath.Dir(s.cfg.NotesPath), 0o700)
	}
	if err == nil {
		// Replace it whole, so another replica never reads half a file.
		tmp := s.cfg.NotesPath + ".tmp"
		if err = os.WriteFile(tmp, b, 0o600); err == nil {
			err = os.Rename(tmp, s.cfg.NotesPath)
		}
	}
	if err != nil {
		s.log.Warn("keep the note", "error", err)
	}
}

// loadNotes reads the kept notes into s.recent, if the file is there.
// s.notesMu must be held.
func (s *Server) loadNotes() {
	if s.cfg.NotesPath == "" {
		return
	}
	b, err := os.ReadFile(s.cfg.NotesPath)
	if err != nil {
		return
	}
	var notes []string
	if json.Unmarshal(b, &notes) == nil {
		s.recent = notes
	}
}
