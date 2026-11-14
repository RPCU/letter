package server

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
	"unicode/utf8"
)

// claudeWrite runs Claude Code once, isolated from the account it's logged
// in with: no tools, no MCP servers or claude.ai connectors, no plugins,
// skills, hooks, settings, CLAUDE.md or memory, no saved session, in an empty
// directory and with only the environment it needs (none of LETTER_*). It
// streams the note's text to emit.
func (s *Server) claudeWrite(ctx context.Context, system, prompt string, emit func(string)) error {
	dir, err := os.MkdirTemp("", "letter-note-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(dir) }()

	cmd := exec.CommandContext(ctx, s.cfg.Command, claudeArgs(s.cfg.Model, system)...)
	cmd.Dir = dir
	cmd.Env = claudeEnv(os.Getenv, dir)
	cmd.Stdin = strings.NewReader(prompt)
	cmd.WaitDelay = 5 * time.Second
	var stderr bytes.Buffer
	cmd.Stderr = &limitedBuffer{buf: &stderr, max: 4 << 10}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	release, err := s.takeClaudeTurn(ctx)
	if err != nil {
		return err
	}
	defer release()
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start %s: %w", s.cfg.Command, err)
	}

	var failure string
	sc := bufio.NewScanner(stdout)
	sc.Buffer(make([]byte, 64<<10), 8<<20)
	for sc.Scan() {
		var ev streamEvent
		if json.Unmarshal(sc.Bytes(), &ev) != nil {
			continue
		}
		switch ev.Type {
		case "stream_event":
			if ev.Event.Type == "content_block_delta" && ev.Event.Delta.Type == "text_delta" && ev.Event.Delta.Text != "" {
				emit(ev.Event.Delta.Text)
			}
		case "result":
			if ev.IsError || ev.Subtype != "success" {
				failure = valueOrString(ev.Result, ev.Subtype)
			}
		}
	}
	waitErr := cmd.Wait()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if failure != "" {
		// Not logged in, usage limits…: worth a look, not an alert.
		s.log.Warn("Claude could not write the note", "reason", truncate(failure, 300))
		return errAway
	}
	if waitErr != nil {
		return fmt.Errorf("%s: %w: %s", s.cfg.Command, waitErr, truncate(strings.TrimSpace(stderr.String()), 300))
	}
	return nil
}

// streamEvent is the part of Claude Code's stream-json output we read.
type streamEvent struct {
	Type    string `json:"type"`
	Subtype string `json:"subtype"`
	IsError bool   `json:"is_error"`
	Result  string `json:"result"`
	Event   struct {
		Type  string `json:"type"`
		Delta struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"delta"`
	} `json:"event"`
}

func claudeArgs(model, system string) []string {
	return []string{
		"--print",
		"--output-format", "stream-json", "--verbose", "--include-partial-messages",
		"--no-session-persistence",
		// No CLAUDE.md, skills, plugins, hooks, MCP servers or custom agents.
		"--safe-mode",
		"--setting-sources", "",
		"--strict-mcp-config",
		"--tools", "",
		"--disable-slash-commands",
		"--model", model,
		// Replaces Claude Code's own system prompt: just the letter's author.
		"--system-prompt", system,
	}
}

// claudeEnv passes only what Claude Code needs to find its login, plus the
// switches that keep the account's connectors and memory out.
func claudeEnv(getenv func(string) string, dir string) []string {
	env := []string{
		"TMPDIR=" + dir,
		"ENABLE_CLAUDEAI_MCP_SERVERS=false",
		"CLAUDE_CODE_DISABLE_AUTO_MEMORY=1",
		"CLAUDE_CODE_DISABLE_CLAUDE_MDS=1",
		"CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1",
		"DISABLE_AUTOUPDATER=1",
	}
	for _, k := range []string{"PATH", "HOME", "CLAUDE_CONFIG_DIR", "CLAUDE_CODE_OAUTH_TOKEN", "SSL_CERT_FILE", "LANG"} {
		if v := getenv(k); v != "" {
			env = append(env, k+"="+v)
		}
	}
	return env
}

// limitedBuffer keeps the first max bytes written to it.
type limitedBuffer struct {
	buf *bytes.Buffer
	max int
}

func (l *limitedBuffer) Write(p []byte) (int, error) {
	if room := l.max - l.buf.Len(); room > 0 {
		l.buf.Write(p[:min(len(p), room)])
	}
	return len(p), nil
}

func truncate(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n]) + "…"
}

func valueOrString(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}
