package config

import (
	"strings"
	"testing"
)

func load(t *testing.T, env map[string]string) (Config, error) {
	t.Helper()
	return Load(func(k string) string { return env[k] })
}

func TestDefaults(t *testing.T) {
	cfg, err := load(t, map[string]string{"HOME": "/home/letter"})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Name != DefaultName || cfg.Title != DefaultTitle || cfg.Question != DefaultQuestion || cfg.Port != "8080" || cfg.Theme != "rose" {
		t.Errorf("defaults: %+v", cfg)
	}
	if !cfg.AI || cfg.Location.String() != "UTC" {
		t.Errorf("AI %v, zone %s", cfg.AI, cfg.Location)
	}
	if cfg.AnswersPath != "/home/letter/.claude/letter-answers.md" || cfg.LockPath != "/home/letter/.claude/letter-claude.lock" {
		t.Errorf("paths %q %q", cfg.AnswersPath, cfg.LockPath)
	}
	if len(cfg.Paragraphs()) != 3 {
		t.Errorf("paragraphs %q", cfg.Paragraphs())
	}
	if len(cfg.NoLabels) != len(DefaultNoLabels) || len(cfg.NoLabels) != len(cfg.FallbackNo) || len(cfg.Reasons) != len(cfg.FallbackWhy) {
		t.Error("each no and each reason needs a fallback")
	}
	if !cfg.Shuffle || !cfg.AskWhy || !cfg.Bargain || !cfg.Cloud {
		t.Errorf("%+v", cfg)
	}
}

func TestATrip(t *testing.T) {
	cfg, err := load(t, map[string]string{
		"LETTER_NAME":       "Andie",
		"LETTER_TITLE":      "Bon voyage",
		"LETTER_MESSAGE":    "Singapore is lucky.\r\n\r\n  Bring back kaya toast.  \n",
		"LETTER_QUESTION":   "Will you send me a photo from Marina Bay?",
		"LETTER_NO":         "Nope\n\nNot a chance\n",
		"LETTER_THEME":      "Ocean",
		"LETTER_SEAL":       "✈️",
		"LETTER_TIMEZONE":   "Asia/Singapore",
		"LETTER_ANSWERS":    "false",
		"CLAUDE_CONFIG_DIR": "/c",
	})
	if err != nil {
		t.Fatal(err)
	}
	if p := cfg.Paragraphs(); len(p) != 2 || p[1] != "Bring back kaya toast." {
		t.Errorf("paragraphs %q", p)
	}
	if cfg.NoLabel(0) != "Nope" || cfg.NoLabel(1) != "Not a chance" || cfg.NoLabel(9) != "Not a chance" {
		t.Errorf("labels %q", cfg.NoLabels)
	}
	if cfg.Theme != "ocean" || cfg.AnswersPath != "" || cfg.LockPath != "/c/letter-claude.lock" {
		t.Errorf("%+v", cfg)
	}
}

func TestEveryProblemAtOnce(t *testing.T) {
	_, err := load(t, map[string]string{
		"LETTER_PORT":     "http",
		"LETTER_THEME":    "neon",
		"LETTER_AI":       "sure",
		"LETTER_TIMEZONE": "Mars/Olympus",
		"LETTER_MODEL":    "--evil",
		"LETTER_NAME":     strings.Repeat("x", 65),
		"LETTER_NO":       strings.Repeat("x", 41),
		"LETTER_PIN":      "123",
	})
	if err == nil {
		t.Fatal("no error")
	}
	for _, key := range []string{"LETTER_PORT", "LETTER_THEME", "LETTER_AI", "LETTER_TIMEZONE", "LETTER_MODEL", "LETTER_NAME", "LETTER_NO", "LETTER_PIN"} {
		if !strings.Contains(err.Error(), key) {
			t.Errorf("error lacks %s: %v", key, err)
		}
	}
}
