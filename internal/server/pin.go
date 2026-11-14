package server

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	// keyCookie holds proof that the code was entered.
	keyCookie = "letter_key"
	keyMaxAge = 90 * 24 * time.Hour
	// unlockBurst tries at once, then unlockPerMinute, across every client:
	// a 4-digit code then takes days to guess, a 6-digit one years.
	unlockBurst     = 8
	unlockPerMinute = 4
)

// locked reports whether this request may not see the letter: a code is set
// and the request doesn't carry the key it unlocks.
func (s *Server) locked(r *http.Request) bool {
	if s.cfg.Pin == "" {
		return false
	}
	c, err := r.Cookie(keyCookie)
	return err != nil || !hmac.Equal([]byte(c.Value), []byte(s.key()))
}

// key is what the cookie carries once unlocked: derived from the code, so
// it works on every replica and changing LETTER_PIN locks every visitor out
// again. It never reveals the code.
func (s *Server) key() string {
	m := hmac.New(sha256.New, []byte("letter-pin\x00"+s.cfg.Pin))
	m.Write([]byte("unlocked"))
	return hex.EncodeToString(m.Sum(nil))
}

// handleUnlock checks the code and hands out the key.
func (s *Server) handleUnlock(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if s.cfg.Pin == "" {
		writeJSONError(w, http.StatusNotFound, "not locked")
		return
	}
	var req struct {
		Pin string `json:"pin"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "bad request")
		return
	}
	if !s.unlocks.allow(s.now()) {
		w.Header().Set("Retry-After", strconv.Itoa(60/unlockPerMinute))
		writeJSONError(w, http.StatusTooManyRequests, "slow down")
		return
	}
	// Compare digests so the time taken says nothing about the code's length.
	got, want := sha256.Sum256([]byte(strings.TrimSpace(req.Pin))), sha256.Sum256([]byte(s.cfg.Pin))
	if subtle.ConstantTimeCompare(got[:], want[:]) != 1 {
		s.log.Info("wrong code entered")
		writeJSONError(w, http.StatusForbidden, "wrong code")
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     keyCookie,
		Value:    s.key(),
		Path:     "/",
		MaxAge:   int(keyMaxAge.Seconds()),
		HttpOnly: true,
		Secure:   r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https",
		// Lax, not Strict: opening the link from a chat app is a cross-site
		// navigation, and the letter should stay open then.
		SameSite: http.SameSiteLaxMode,
	})
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte("{\"ok\":true}\n"))
}
