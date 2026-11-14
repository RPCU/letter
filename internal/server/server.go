// Package server serves the letter, the notes Claude writes after each answer
// and the static assets.
package server

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/Banh-Canh/letter/internal/config"
	"github.com/Banh-Canh/letter/web"
)

// contentSecurityPolicy allows only same-origin scripts (no inline ones) plus
// Google Fonts for the typefaces.
const contentSecurityPolicy = "default-src 'none'; script-src 'self'; style-src 'self' https://fonts.googleapis.com; " +
	"font-src https://fonts.gstatic.com; img-src 'self' data:; connect-src 'self'; manifest-src 'self'; " +
	"base-uri 'none'; form-action 'none'; frame-ancestors 'none'"

// Server renders one configured letter.
type Server struct {
	cfg    config.Config
	log    *slog.Logger
	page   *template.Template
	static fs.FS
	// assetVersion fingerprints the CSS and JS so a deploy never serves new
	// HTML with stale cached assets.
	assetVersion string
	// tries and slots keep the page from running away with the Claude login;
	// write writes the notes (swapped in tests).
	tries *limiter
	// unlocks throttles the guesses at LETTER_PIN.
	unlocks *limiter
	slots   chan struct{}
	write   func(ctx context.Context, system, prompt string, emit func(string)) error
	// awayUntil is when to try Claude again after it failed; until then the
	// canned notes stand in, after a pause as if written.
	awayMu    sync.Mutex
	awayUntil time.Time
	typing    time.Duration
	answersMu sync.Mutex
	// recent are the last notes written (see notes.go).
	notesMu sync.Mutex
	recent  []string
	// pick draws the moods, lengths, images and canned asides (swapped in
	// tests).
	pick func(n int) int
	// claudeTurn lets one Claude Code run at a time in this process (see
	// takeClaudeTurn).
	claudeTurn chan struct{}
	// now is swapped in tests.
	now func() time.Time
}

// New parses the embedded template and prepares the static file tree.
func New(cfg config.Config, log *slog.Logger) (*Server, error) {
	page, err := template.ParseFS(web.FS, "templates/index.html")
	if err != nil {
		return nil, fmt.Errorf("parse template: %w", err)
	}
	static, err := fs.Sub(web.FS, "static")
	if err != nil {
		return nil, fmt.Errorf("static assets: %w", err)
	}
	version, err := fingerprint(static, "css/letter.css", "js/letter.js")
	if err != nil {
		return nil, fmt.Errorf("static assets: %w", err)
	}
	s := &Server{
		cfg:          cfg,
		log:          log,
		page:         page,
		static:       static,
		assetVersion: version,
		tries:        newLimiter(answerBurst, answerPerMinute),
		unlocks:      newLimiter(unlockBurst, unlockPerMinute),
		slots:        make(chan struct{}, answerSlots),
		typing:       fallbackTyping,
		claudeTurn:   make(chan struct{}, 1),
		now:          time.Now,
		pick:         rand.IntN,
	}
	s.write = s.claudeWrite
	return s, nil
}

// Handler returns the routes wrapped in the common middleware.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.handleIndex)
	// Same-origin only: rejects cross-site form posts and fetches.
	cop := http.NewCrossOriginProtection()
	mux.Handle("POST /api/answer", cop.Handler(http.HandlerFunc(s.handleAnswer)))
	mux.Handle("POST /api/unlock", cop.Handler(http.HandlerFunc(s.handleUnlock)))
	mux.HandleFunc("GET /healthz", handleHealth)
	mux.Handle("GET /static/", http.StripPrefix("/static/", cacheFor(time.Hour, http.FileServerFS(s.static))))
	return s.recoverPanics(securityHeaders(s.logRequests(mux)))
}

// view is everything the page shows. The gift is not in it: it's sent only
// with the reply to a yes.
type view struct {
	// Locked shows only the sealed envelope and the code field: none of
	// the fields below but Seal, Theme and AssetVersion are set.
	Locked bool
	// Numeric brings up the number pad for an all-digit code.
	Numeric bool

	Name       string
	From       string
	Title      string
	Paragraphs []string
	Question   string
	Yes        string
	No         string
	// NoLabels is the no button's words, Reasons the chips after a no
	// (empty without LETTER_ASK_WHY), as JSON for the script.
	NoLabels  string
	Reasons   string
	Why       string
	Bargain   bool
	Offer     string
	More      string
	Rounds    int
	Cloud     bool
	YesTitle  string
	Stamp     string
	HasGift   bool
	Seal      string
	Theme     string
	DateLabel string
	// Preview is the link preview's line: the title only, never the letter.
	Preview      string
	AssetVersion string
}

func (s *Server) lockedView() view {
	return view{
		Locked:       true,
		Numeric:      strings.Trim(s.cfg.Pin, "0123456789") == "",
		Seal:         s.cfg.Seal,
		Theme:        s.cfg.Theme,
		Preview:      "A sealed letter",
		AssetVersion: s.assetVersion,
	}
}

func (s *Server) view() view {
	labels, _ := json.Marshal(s.cfg.NoLabels)
	var reasons []string
	why := ""
	if s.cfg.AskWhy {
		reasons, why = s.cfg.Reasons, s.cfg.Why
	}
	reasonsJSON, _ := json.Marshal(reasons)
	return view{
		Reasons:      string(reasonsJSON),
		Why:          why,
		Bargain:      s.cfg.Bargain,
		Offer:        s.cfg.Offer,
		More:         s.cfg.More,
		Rounds:       config.BargainRounds,
		Cloud:        s.cfg.Cloud,
		Name:         s.cfg.Name,
		From:         s.cfg.From,
		Title:        s.cfg.Title,
		Paragraphs:   s.cfg.Paragraphs(),
		Question:     s.cfg.Question,
		Yes:          s.cfg.Yes,
		No:           s.cfg.NoLabel(0),
		NoLabels:     string(labels),
		YesTitle:     s.cfg.YesTitle,
		Stamp:        s.cfg.Stamp,
		HasGift:      s.cfg.Gift != "",
		Seal:         s.cfg.Seal,
		Theme:        s.cfg.Theme,
		DateLabel:    s.now().In(s.cfg.Location).Format("January 2, 2006"),
		Preview:      fmt.Sprintf("A letter for %s", s.cfg.Name),
		AssetVersion: s.assetVersion,
	}
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	// Render into a buffer so a template error becomes a clean 500 instead
	// of a half-written page.
	v := s.view()
	if s.locked(r) {
		v = s.lockedView()
	}
	var buf bytes.Buffer
	if err := s.page.Execute(&buf, v); err != nil {
		s.log.Error("render page", "error", err)
		http.Error(w, "something went wrong", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = buf.WriteTo(w)
}

func handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte("ok\n"))
}

// isLink reports whether the gift is a single web link, so the page can offer
// to open it.
func isLink(s string) bool {
	u, err := url.Parse(s)
	return err == nil && (u.Scheme == "https" || u.Scheme == "http") && u.Host != "" && !strings.ContainsAny(s, " \n")
}

// fingerprint returns a short content hash of the named files.
func fingerprint(fsys fs.FS, names ...string) (string, error) {
	h := sha256.New()
	for _, name := range names {
		b, err := fs.ReadFile(fsys, name)
		if err != nil {
			return "", err
		}
		h.Write(b)
	}
	return hex.EncodeToString(h.Sum(nil))[:12], nil
}

func writeJSONError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
