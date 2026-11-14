package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Banh-Canh/letter/internal/config"
)

func locked(t *testing.T) *Server {
	t.Helper()
	return newServer(t, func(c *config.Config) {
		c.Pin = "2468"
		c.Gift = "GIFT-CODE-42"
	})
}

func unlock(t *testing.T, s *Server, body string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/unlock", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.Header.Set("X-Forwarded-Proto", "https")
	s.Handler().ServeHTTP(rec, req)
	return rec
}

func withKey(t *testing.T, s *Server, req *http.Request, key *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	if key != nil {
		req.AddCookie(key)
	}
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	return rec
}

func TestLockedPageShowsOnlyTheEnvelope(t *testing.T) {
	s := locked(t)
	body := get(t, s, "/").Body.String()
	if !strings.Contains(body, `class="lock"`) || !strings.Contains(body, `inputmode="numeric"`) {
		t.Error("no code field")
	}
	for _, never := range []string{"Chihiro", "Victor", "I’m sorry", "Do you forgive me?", "2468", "GIFT-CODE-42", "Still no"} {
		if strings.Contains(body, never) {
			t.Errorf("locked page leaks %q", never)
		}
	}
	rec := post(t, s, `{"answer":"no","noes":0}`)
	if rec.Code != http.StatusForbidden {
		t.Errorf("answer while locked: status %d", rec.Code)
	}
}

func TestTheCodeOpensTheLetter(t *testing.T) {
	s := locked(t)
	if rec := unlock(t, s, `{"pin":"1357"}`); rec.Code != http.StatusForbidden || len(rec.Result().Cookies()) != 0 {
		t.Fatalf("wrong code: status %d, cookies %v", rec.Code, rec.Result().Cookies())
	}
	rec := unlock(t, s, `{"pin":" 2468 "}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("right code: status %d", rec.Code)
	}
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("cookies %v", cookies)
	}
	key := cookies[0]
	if !key.HttpOnly || !key.Secure || key.SameSite != http.SameSiteLaxMode || strings.Contains(key.Value, "2468") {
		t.Errorf("cookie %+v", key)
	}

	body := withKey(t, s, httptest.NewRequest(http.MethodGet, "/", nil), key).Body.String()
	if !strings.Contains(body, "Dear Chihiro,") || strings.Contains(body, `class="lock"`) {
		t.Error("the key doesn't open the letter")
	}
	req := httptest.NewRequest(http.MethodPost, "/api/answer", strings.NewReader(`{"answer":"no","noes":0}`))
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	s.cfg.AI = false
	if rec := withKey(t, s, req, key); rec.Code != http.StatusOK {
		t.Errorf("answer with the key: status %d", rec.Code)
	}

	// A forged key, or one from an older code, doesn't.
	forged := &http.Cookie{Name: keyCookie, Value: strings.Repeat("0", 64)}
	if body := withKey(t, s, httptest.NewRequest(http.MethodGet, "/", nil), forged).Body.String(); strings.Contains(body, "Chihiro") {
		t.Error("forged key opens the letter")
	}
	s.cfg.Pin = "9999"
	if body := withKey(t, s, httptest.NewRequest(http.MethodGet, "/", nil), key).Body.String(); strings.Contains(body, "Chihiro") {
		t.Error("key from the old code still opens the letter")
	}
}

func TestGuessesAreThrottled(t *testing.T) {
	s := locked(t)
	for range unlockBurst {
		if rec := unlock(t, s, `{"pin":"0000"}`); rec.Code != http.StatusForbidden {
			t.Fatalf("status %d", rec.Code)
		}
	}
	if rec := unlock(t, s, `{"pin":"2468"}`); rec.Code != http.StatusTooManyRequests {
		t.Errorf("status %d after %d guesses", rec.Code, unlockBurst)
	}
}

func TestUnlockedWithoutACode(t *testing.T) {
	s := newServer(t)
	if !strings.Contains(get(t, s, "/").Body.String(), "Dear Chihiro,") {
		t.Error("no code set, yet locked")
	}
	if rec := unlock(t, s, `{"pin":"2468"}`); rec.Code != http.StatusNotFound {
		t.Errorf("unlock without a code: status %d", rec.Code)
	}
}
