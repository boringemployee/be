package tty

import (
	"bytes"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/boringemployee/be/content"
	"github.com/boringemployee/be/internal/engine"
	"github.com/boringemployee/be/internal/profile"
	"github.com/boringemployee/be/internal/render"
)

func TestParse(t *testing.T) {
	r := func(c rune) Input { return Input{Kind: InputRune, Rune: c} }
	tests := []struct {
		name string
		in   string
		want []Input
	}{
		{"text", "hé", []Input{r('h'), r('é')}},
		{"enter", "a\r\n", []Input{r('a'), {Kind: InputEnter}, {Kind: InputEnter}}},
		{"backspace", "\x7f\x08", []Input{{Kind: InputBackspace}, {Kind: InputBackspace}}},
		{"ctrl keys", "\x15\x03\x04\x01", []Input{{Kind: InputClear}, {Kind: InputInterrupt}, {Kind: InputEOF}}},
		{"lone esc", "\x1b", []Input{{Kind: InputEsc}}},
		{"arrow ignored", "\x1b[Ax", []Input{r('x')}},
		{"csi with params", "\x1b[1;5Cx", []Input{r('x')}},
		{"unterminated csi", "\x1b[12", nil},
		{"ss3 ignored", "\x1bOPx", []Input{r('x')}},
		{"short ss3", "\x1bO", nil},
		{"alt key ignored", "\x1bxy", []Input{r('y')}},
		{"invalid utf8", "\xffa", []Input{r('a')}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Parse([]byte(tt.in)); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("Parse(%q) = %+v, want %+v", tt.in, got, tt.want)
			}
		})
	}
}

func newSession(t *testing.T, out *bytes.Buffer) *Session {
	t.Helper()
	cat, err := profile.Load(content.FS)
	if err != nil {
		t.Fatal(err)
	}
	eng, err := engine.New(cat, "claude", 42)
	if err != nil {
		t.Fatal(err)
	}
	return &Session{Engine: eng, Renderer: render.New(eng.Agent(), 80), Out: out}
}

// script feeds key chunks and ticks to a session in order.
type step struct {
	keys  string
	ticks int
}

func play(t *testing.T, s *Session, steps []step) error {
	t.Helper()
	keys, ticks := make(chan []byte), make(chan time.Time)
	done := make(chan error, 1)
	go func() { done <- s.Run(keys, ticks) }()
	for _, st := range steps {
		if st.keys != "" {
			select {
			case keys <- []byte(st.keys):
			case err := <-done:
				return err
			}
		}
		for range st.ticks {
			select {
			case ticks <- time.Time{}:
			case err := <-done:
				return err
			}
		}
	}
	close(keys)
	return <-done
}

func TestSessionRunEscAndExit(t *testing.T) {
	var out bytes.Buffer
	s := newSession(t, &out)
	s.Width = func() int { return 60 }
	err := play(t, s, []step{
		{ticks: 3},                        // idle ticks redraw nothing
		{keys: "helo\x7fp\r"},             // typing with backspace
		{keys: "xx\x15run engineering\r"}, // Ctrl+U clears
		{ticks: 300},                      // 30s of fake work
		{keys: "\x1b"},                    // Esc stops, still in session
		{keys: "\x04"},                    // Ctrl+D on empty line exits
	})
	if err != nil {
		t.Fatal(err)
	}
	text := out.String()
	for _, want := range []string{"Claude-ish", "> help", "Commands:", "> run engineering", "●", "Esc to stop", "Interrupted", "Session ended"} {
		if !strings.Contains(text, want) {
			t.Errorf("output missing %q", want)
		}
	}
	if strings.Contains(text, "xxrun") {
		t.Error("Ctrl+U should have cleared the line")
	}
	if s.Engine.State() != engine.StateExited {
		t.Errorf("state = %v, want exited", s.Engine.State())
	}
}

func TestSessionCtrlCAndEOFWithText(t *testing.T) {
	var out bytes.Buffer
	s := newSession(t, &out)
	if err := play(t, s, []step{{keys: "ab\x04"}, {keys: "\x03"}}); err != nil {
		t.Fatal(err)
	}
	if s.Engine.State() != engine.StateExited {
		t.Fatal("Ctrl+C must exit")
	}
	if !strings.Contains(out.String(), "ab") {
		t.Error("Ctrl+D with text must not exit or clear")
	}
}

func TestSessionClosedInput(t *testing.T) {
	var out bytes.Buffer
	s := newSession(t, &out)
	if err := play(t, s, nil); err != nil {
		t.Fatal(err)
	}
	if s.Engine.State() != engine.StateExited {
		t.Fatal("closing input must end the session")
	}
}

func TestSessionInputLimitAndBackspaceEmpty(t *testing.T) {
	var out bytes.Buffer
	s := newSession(t, &out)
	if err := play(t, s, []step{{keys: "\x7f" + strings.Repeat("a", engine.MaxInputLen+10)}}); err != nil {
		t.Fatal(err)
	}
	if len(s.input) != engine.MaxInputLen {
		t.Fatalf("input length = %d, want %d", len(s.input), engine.MaxInputLen)
	}
}

type failWriter struct{ after int }

func (w *failWriter) Write(p []byte) (int, error) {
	if w.after == 0 {
		return 0, errors.New("write failed")
	}
	w.after--
	return len(p), nil
}

func TestSessionWriteErrors(t *testing.T) {
	for _, tt := range []struct {
		name  string
		after int
		steps []step
	}{
		{"start", 0, nil},
		{"key", 1, []step{{keys: "a"}}},
		{"tick", 2, []step{{keys: "run engineering\r"}, {ticks: 1}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s := newSession(t, nil)
			s.Out = &failWriter{after: tt.after}
			if err := play(t, s, tt.steps); err == nil {
				t.Fatal("want write error")
			}
		})
	}
}
