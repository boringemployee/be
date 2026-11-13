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
		Title: "Demo", Prompt: "> ", Accent: "#D97757", Layout: layout,
		Spinner: []string{"a", "b"},
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
		{"welcome", profile.LayoutBoxed, engine.Event{Kind: engine.EventWelcome, Text: "Demo", Lines: []string{"banner", "hi"}}, "Demo\r\nbanner\r\nhi\r\n\r\n"},
		{"prompt", profile.LayoutBoxed, engine.Event{Kind: engine.EventPrompt}, ""},
		{"thinking", profile.LayoutBoxed, engine.Event{Kind: engine.EventThinking, Text: "Hmm"}, ""},
		{"echo", profile.LayoutBoxed, engine.Event{Kind: engine.EventEcho, Text: "help"}, "> help\r\n"},
		{"notice", profile.LayoutBoxed, engine.Event{Kind: engine.EventNotice, Lines: []string{"a", "b"}}, "a\r\nb\r\n"},
		{"error", profile.LayoutBoxed, engine.Event{Kind: engine.EventError, Text: "bad"}, "✗ bad\r\n"},
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
	if want := strings.Repeat("é", MinWidth-1) + "…\r\n"; got != want {
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

func TestStatus(t *testing.T) {
	r := New(agent(profile.LayoutBoxed), 80)
	got := r.Status(3, "Pondering", 12500*time.Millisecond)
	if !strings.HasPrefix(got, ClearLine) {
		t.Errorf("status must clear the line first: %q", got)
	}
	if want := "b Pondering… (12s · Esc to stop)"; plain(got[len(ClearLine):]) != want {
		t.Errorf("status = %q, want %q", plain(got[len(ClearLine):]), want)
	}
	if strings.HasSuffix(got, NL) {
		t.Error("status must not end with a newline")
	}
}

func TestInput(t *testing.T) {
	r := New(agent(profile.LayoutBoxed), 20)
	if got := plain(r.Input("run")[len(ClearLine):]); got != "> run" {
		t.Errorf("Input = %q", got)
	}
	long := plain(r.Input(strings.Repeat("ab", 30))[len(ClearLine):])
	if n := len([]rune(long)); n != 19 || !strings.HasPrefix(long, "> …") || !strings.HasSuffix(long, "ab") {
		t.Errorf("long input = %q (%d columns)", long, n)
	}
}
