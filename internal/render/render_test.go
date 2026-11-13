package render

import (
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/boringemployee/be/internal/engine"
	"github.com/boringemployee/be/internal/profile"
)

func agent(layout profile.Layout) *profile.Agent {
	return &profile.Agent{
		Title: "Demo", Version: "v1.0.0", Prompt: "> ", Accent: "#D97757", Layout: layout,
		Spinner: []string{"a", "b"}, Mascot: []string{"##", "#"}, Header: []string{"model", "{cwd}", "extra"},
		StatusRight: "low", FooterMode: "mode", FooterHint: "hint",
	}
}

var ansiRE = regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]`)

// plain strips the escape sequences the renderer itself produces.
func plain(s string) string { return ansiRE.ReplaceAllString(s, "") }

func TestAccentCode(t *testing.T) {
	if got := accentCode("#D97757"); got != "\x1b[38;2;217;119;87m" {
		t.Fatalf("accentCode = %q", got)
	}
}

func TestEvents(t *testing.T) {
	tests := []struct {
		name   string
		layout profile.Layout
		ev     engine.Event
		want   string
	}{
		{"welcome", profile.LayoutBoxed, engine.Event{Kind: engine.EventWelcome, Text: "Demo", Lines: []string{"Bold", "hi"}},
			"\r\n ##   Demo v1.0.0\r\n #    model\r\n      ~\r\n      extra\r\n\r\n ▎ Bold\r\n ▎ hi\r\n\r\n"},
		{"prompt", profile.LayoutBoxed, engine.Event{Kind: engine.EventPrompt}, ""},
		{"thinking", profile.LayoutBoxed, engine.Event{Kind: engine.EventThinking, Text: "Hmm"}, ""},
		{"echo", profile.LayoutBoxed, engine.Event{Kind: engine.EventEcho, Text: "help"}, "> help\r\n"},
		{"notice", profile.LayoutBoxed, engine.Event{Kind: engine.EventNotice, Lines: []string{"a", "b"}}, "  a\r\n  b\r\n"},
		{"error", profile.LayoutBoxed, engine.Event{Kind: engine.EventError, Text: "bad"}, "  ✗ bad\r\n"},
		{"task start", profile.LayoutBoxed, engine.Event{Kind: engine.EventTaskStart, Text: "Fix it"}, "\r\n● Fix it\r\n"},
		{"read boxed", profile.LayoutBoxed, engine.Event{Kind: engine.EventRead, Text: "a.go"}, "  ● Read(a.go)\r\n    ⎿ Read "},
		{"read inline", profile.LayoutInline, engine.Event{Kind: engine.EventRead, Text: "a.go"}, "  → read a.go\r\n"},
		{"edit", profile.LayoutBoxed, engine.Event{Kind: engine.EventEdit, Text: "a.go", Lines: []string{"-old", "+new"}}, "  ● Update(a.go)\r\n      -old\r\n      +new\r\n"},
		{"command ok", profile.LayoutBoxed, engine.Event{Kind: engine.EventCommand, Text: "make", Lines: []string{"log", "PASS x"}, OK: true}, "  ● Run(make)\r\n      log\r\n      PASS x\r\n"},
		{"command fail", profile.LayoutInline, engine.Event{Kind: engine.EventCommand, Text: "make", Lines: []string{"FAIL x"}}, "  → run make\r\n      FAIL x\r\n"},
		{"done", profile.LayoutBoxed, engine.Event{Kind: engine.EventTaskDone, Text: "Done"}, "● Done\r\n"},
		{"stopped", profile.LayoutBoxed, engine.Event{Kind: engine.EventStopped, Text: "Stopped"}, "  ⎿ Stopped\r\n\r\n"},
		{"exit", profile.LayoutBoxed, engine.Event{Kind: engine.EventExit}, "\r\nSession ended. Back to real work.\r\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := plain(New(agent(tt.layout), 80).Event(tt.ev))
			if !strings.HasPrefix(got, tt.want) {
				t.Fatalf("got %q, want prefix %q", got, tt.want)
			}
		})
	}
}

func TestColors(t *testing.T) {
	r := New(agent(profile.LayoutBoxed), 80)
	edit := r.Event(engine.Event{Kind: engine.EventEdit, Text: "a.go", Lines: []string{"-old", "+new"}})
	if !strings.Contains(edit, red+"-old") || !strings.Contains(edit, green+"+new") {
		t.Errorf("diff colors wrong: %q", edit)
	}
	ok := r.Event(engine.Event{Kind: engine.EventCommand, Text: "m", Lines: []string{"PASS"}, OK: true})
	fail := r.Event(engine.Event{Kind: engine.EventCommand, Text: "m", Lines: []string{"FAIL"}})
	if !strings.Contains(ok, green+"PASS") || !strings.Contains(fail, red+"FAIL") {
		t.Errorf("result colors wrong: %q %q", ok, fail)
	}
}

func TestTruncation(t *testing.T) {
	r := New(agent(profile.LayoutBoxed), 5) // clamped to MinWidth
	got := plain(r.Event(engine.Event{Kind: engine.EventNotice, Lines: []string{strings.Repeat("é", 50)}}))
	if want := "  " + strings.Repeat("é", MinWidth-3) + "…\r\n"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	r.SetWidth(30)
	for _, l := range strings.Split(plain(r.Event(engine.Event{Kind: engine.EventTaskStart, Text: strings.Repeat("x", 99)})), NL) {
		if n := len([]rune(l)); n > 30 {
			t.Fatalf("line %q has %d columns, want <= 30", l, n)
		}
	}
}

func TestLineCountStable(t *testing.T) {
	a, b := lineCount("a.go"), lineCount("a.go")
	if a != b || a < 40 || a >= 500 {
		t.Fatalf("lineCount = %d, %d", a, b)
	}
}

func TestWelcomeVariants(t *testing.T) {
	a := agent(profile.LayoutBoxed)
	a.Mascot, a.Header = nil, nil
	r := New(a, 80)
	r.SetCwd("\x1b[2J~/work")
	r.SetCwd("") // ignored
	got := plain(r.Event(engine.Event{Kind: engine.EventWelcome, Lines: []string{strings.Repeat("word ", 30)}}))
	if !strings.HasPrefix(got, "\r\n Demo v1.0.0\r\n\r\n") {
		t.Errorf("no-mascot header = %q", got)
	}
	if strings.Count(got, "▎") != 2 {
		t.Errorf("long notice line should wrap onto 2 lines: %q", got)
	}
	if r.cwd != "[2J~/work" {
		t.Errorf("cwd = %q, want sanitized", r.cwd)
	}
}

func TestWrapAndFit(t *testing.T) {
	if got := wrap("aa bb cc", 5); strings.Join(got, "|") != "aa bb|cc" {
		t.Errorf("wrap = %q", got)
	}
	if got := wrap("toolongword x", 4); strings.Join(got, "|") != "too…|x" {
		t.Errorf("wrap long word = %q", got)
	}
	if fit("abc", 0) != "" || fit("abc", -1) != "" {
		t.Error("fit with no room must be empty")
	}
}

func TestLive(t *testing.T) {
	r := New(agent(profile.LayoutBoxed), 20)
	idle := r.Live(LiveState{Input: "run"})
	lines := strings.Split(plain(idle), NL)
	if len(lines) != 5 {
		t.Fatalf("live area has %d lines, want 5: %q", len(lines), lines)
	}
	if lines[0] != strings.Repeat(" ", 16)+"low" {
		t.Errorf("status line = %q", lines[0])
	}
	if lines[1] != strings.Repeat("─", 19) || lines[3] != lines[1] {
		t.Errorf("rules = %q, %q", lines[1], lines[3])
	}
	if lines[2] != "> run" {
		t.Errorf("input line = %q", lines[2])
	}
	if !strings.HasPrefix(lines[4], "  mode hint") || !strings.HasSuffix(idle, "\x1b[2A\r\x1b[5C") {
		t.Errorf("footer/cursor = %q / %q", lines[4], idle[len(idle)-12:])
	}

	running := plain(New(agent(profile.LayoutBoxed), 80).Live(LiveState{Running: true, Frame: 3, Word: "Hmm", Elapsed: 12500 * time.Millisecond}))
	if !strings.HasPrefix(running, "b Hmm… (12s · Esc to stop)\r\n") {
		t.Errorf("running status = %q", running)
	}

	long := strings.Split(plain(r.Live(LiveState{Input: strings.Repeat("ab", 30)})), NL)[2]
	if n := len([]rune(long)); n != 19 || !strings.HasPrefix(long, "> …") || !strings.HasSuffix(long, "ab") {
		t.Errorf("long input = %q (%d columns)", long, n)
	}

	bare := agent(profile.LayoutBoxed)
	bare.FooterMode, bare.Prompt = "", ""
	got := New(bare, 20).Live(LiveState{})
	if !strings.Contains(plain(got), "\r\n  hint") || !strings.HasSuffix(got, "\x1b[2A\r") {
		t.Errorf("bare live = %q", got)
	}
	bare.FooterHint = ""
	if footer := strings.Split(plain(New(bare, 20).Live(LiveState{})), NL)[4]; footer != "  \r" {
		t.Errorf("empty footer should be blank: %q", footer)
	}
}

func TestClearLive(t *testing.T) {
	if got := New(agent(profile.LayoutBoxed), 80).ClearLive(); got != "\x1b[2A\r\x1b[J" {
		t.Fatalf("ClearLive = %q", got)
	}
}
