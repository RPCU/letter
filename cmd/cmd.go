// Package cmd implements the letter command line: `serve` (the default)
// and `version`.
package cmd

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	// Embed the zone database so LETTER_TIMEZONE works in minimal images.
	_ "time/tzdata"

	"github.com/Banh-Canh/letter/internal/config"
	"github.com/Banh-Canh/letter/internal/server"
)

// Version and Commit are set at build time with -ldflags -X.
var (
	Version = "dev"
	Commit  = "unknown"
)

const usage = `Usage: letter [command]

Commands:
  serve     serve the letter (default)
  version   print the version

Configuration comes from LETTER_* environment variables, all optional: NAME,
FROM, TITLE, MESSAGE, QUESTION, YES, NO, YES_TITLE, STAMP, GIFT, GIFT_INTRO,
SEAL, THEME, TIMEZONE, AI, CONTEXT, PROMPT, NO_STYLES, YES_STYLE,
FALLBACK_NO, FALLBACK_YES, ANSWERS, MODEL, COMMAND, HOST and PORT. See the
README.
`

// Execute runs the command named by args and returns the process exit code.
func Execute(args []string) int {
	command := "serve"
	if len(args) > 0 {
		command = args[0]
	}
	switch command {
	case "serve":
		return serve()
	case "version":
		fmt.Printf("letter %s (%s)\n", Version, Commit)
		return 0
	case "help", "-h", "--help":
		fmt.Print(usage)
		return 0
	default:
		fmt.Fprint(os.Stderr, usage)
		return 2
	}
}

func serve() int {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	cfg, err := config.Load(os.Getenv)
	if err != nil {
		logger.Error("invalid configuration", "error", err)
		return 1
	}
	srv, err := server.New(cfg, logger)
	if err != nil {
		logger.Error("start server", "error", err)
		return 1
	}

	httpServer := &http.Server{
		Addr:              net.JoinHostPort(cfg.Host, cfg.Port),
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
		ErrorLog:          slog.NewLogLogger(logger.Handler(), slog.LevelWarn),
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() { errCh <- httpServer.ListenAndServe() }()
	if cfg.AI {
		if _, err := exec.LookPath(cfg.Command); err != nil {
			logger.Warn("LETTER_AI is on but Claude Code was not found, the canned notes will stand in", "command", cfg.Command)
		}
	}
	logger.Info(
		"serving",
		"addr", httpServer.Addr,
		"name", cfg.Name,
		"ai", cfg.AI,
		"version", Version,
		"commit", Commit,
	)

	select {
	case err := <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			logger.Error("serve", "error", err)
			return 1
		}
	case <-ctx.Done():
		logger.Info("shutting down")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown", "error", err)
		return 1
	}
	return 0
}
