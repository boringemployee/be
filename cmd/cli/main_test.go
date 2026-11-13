package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRun(t *testing.T) {
	tests := []struct {
		name   string
		args   []string
		code   int
		stdout string
		stderr string
	}{
		{"no args", nil, 0, "Usage:", ""},
		{"help", []string{"--help"}, 0, "Usage:", ""},
		{"version", []string{"--version"}, 0, "be dev", ""},
		{"unknown agent", []string{"claude"}, 2, "", "not available yet"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out, errOut bytes.Buffer
			if code := run(tt.args, &out, &errOut); code != tt.code {
				t.Fatalf("exit code = %d, want %d", code, tt.code)
			}
			if !strings.Contains(out.String(), tt.stdout) {
				t.Errorf("stdout %q does not contain %q", out.String(), tt.stdout)
			}
			if !strings.Contains(errOut.String(), tt.stderr) {
				t.Errorf("stderr %q does not contain %q", errOut.String(), tt.stderr)
			}
		})
	}
}
