package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/boringemployee/be/internal/engine"
)

func TestRun(t *testing.T) {
	var got options
	startSession = func(_ *engine.Engine, o options) error {
		got = o
		if o.seed == 13 {
			return errors.New("boom")
		}
		return nil
	}
	t.Cleanup(func() { startSession = interactive })

	tests := []struct {
		name   string
		args   []string
		code   int
		stdout string
		stderr string
	}{
		{"no args", nil, 0, "Agents:\n  claude", ""},
		{"help flag", []string{"--help"}, 0, "Usage:", ""},
		{"help word", []string{"help"}, 0, "Usage:", ""},
		{"version flag", []string{"--version"}, 0, "be dev", ""},
		{"version word", []string{"version"}, 0, "be dev", ""},
		{"unknown agent", []string{"nope"}, 2, "", "unknown agent"},
		{"bad flag", []string{"--nope"}, 2, "", "flag provided but not defined"},
		{"too many", []string{"claude", "codex"}, 2, "", "too many arguments"},
		{"session", []string{"claude", "--seed", "42"}, 0, "", ""},
		{"session error", []string{"--seed=13", "claude"}, 1, "", "boom"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out, errOut bytes.Buffer
			if code := run(tt.args, &out, &errOut); code != tt.code {
				t.Fatalf("exit code = %d, want %d (stderr %q)", code, tt.code, errOut.String())
			}
			if !strings.Contains(out.String(), tt.stdout) {
				t.Errorf("stdout %q does not contain %q", out.String(), tt.stdout)
			}
			if !strings.Contains(errOut.String(), tt.stderr) {
				t.Errorf("stderr %q does not contain %q", errOut.String(), tt.stderr)
			}
		})
	}
	if got.agent != "claude" || got.seed != 13 {
		t.Errorf("last session options = %+v", got)
	}
}

func TestInteractiveNeedsTerminal(t *testing.T) {
	// Under go test, stdin is not a terminal.
	if err := interactive(nil, options{}); err == nil || !strings.Contains(err.Error(), "terminal") {
		t.Fatalf("err = %v", err)
	}
}
