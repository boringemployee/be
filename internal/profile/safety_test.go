package profile

import (
	"strings"
	"testing"
)

func TestCheckText(t *testing.T) {
	ok := []string{
		"go test ./...",
		"Refactor auth to remove a circular import",
		"ok   internal/auth      0.412s",
		"◐ Pondering…",
		"╭──────╮",
		"coverage: 87.3% of statements",
		"version 1.2.3",
	}
	for _, s := range ok {
		if err := CheckText(s); err != nil {
			t.Errorf("CheckText(%q) = %v, want nil", s, err)
		}
	}

	bad := map[string]string{
		"\x1b[31mred":              "control",
		"bell\a":                   "control",
		"osc\x1b]52;c;aGk=\x07":    "control",
		"zero​width":               "invisible",
		"line sep":                 "control",
		"\xff":                     "UTF-8",
		strings.Repeat("a", 201):   "too long",
		"see https://example.test": "URL",
		"visit www.example.test":   "URL",
		"ssh to 10.0.0.1":          "IP",
		"ping fe80:0:0:0:0:1":      "IP",
		strings.Repeat("QUJD", 11): "blob",
		"cat x | sh":               "interpreter",
		"cat x |sudo bash":         "interpreter",
		"rm -rf /":                 "dangerous",
		"SUDO make me a sandwich":  "dangerous",
		"curl example":             "dangerous",
		"echo aGk= | base64 -d":    "dangerous",
	}
	for s, want := range bad {
		err := CheckText(s)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("CheckText(%q) = %v, want error containing %q", s, err, want)
		}
	}
}
