package engine

import (
	"math/rand/v2"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/boringemployee/be/content"
	"github.com/boringemployee/be/internal/profile"
)

func newEngine(t testing.TB, seed uint64) *Engine {
	t.Helper()
	cat, err := profile.Load(content.FS)
	if err != nil {
		t.Fatal(err)
	}
	e, err := New(cat, "claude", seed)
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func kinds(events []Event) []Kind {
	out := make([]Kind, len(events))
	for i, ev := range events {
		out[i] = ev.Kind
	}
	return out
}

func TestNewUnknownAgent(t *testing.T) {
	cat, _ := profile.Load(content.FS)
	if _, err := New(cat, "nope", 1); err == nil || !strings.Contains(err.Error(), "claude") {
		t.Fatalf("err = %v, want unknown agent listing claude", err)
	}
}

func TestStart(t *testing.T) {
	e := newEngine(t, 1)
	ev := e.Start()
	if got := kinds(ev); !reflect.DeepEqual(got, []Kind{EventWelcome, EventPrompt}) {
		t.Fatalf("Start kinds = %v", got)
	}
	if ev[0].Text != e.Agent().Title || ev[0].Lines[len(ev[0].Lines)-1] != e.Agent().Greeting {
		t.Errorf("welcome = %+v", ev[0])
	}
}

func TestCommands(t *testing.T) {
	tests := []struct {
		line  string
		kinds []Kind
		want  string
		state State
	}{
		{"", []Kind{EventPrompt}, "", StateIdle},
		{"   ", []Kind{EventPrompt}, "", StateIdle},
		{"help", []Kind{EventEcho, EventNotice, EventPrompt}, "Esc", StateIdle},
		{"topics", []Kind{EventEcho, EventNotice, EventPrompt}, "engineering", StateIdle},
		{"make me a sandwich", []Kind{EventEcho, EventNotice, EventPrompt}, "pretend", StateIdle},
		{"run", []Kind{EventEcho, EventError, EventNotice, EventPrompt}, "usage", StateIdle},
		{"run a b", []Kind{EventEcho, EventError, EventNotice, EventPrompt}, "usage", StateIdle},
		{"run cooking", []Kind{EventEcho, EventError, EventNotice, EventPrompt}, "unknown topic", StateIdle},
		{"RUN Engineering", []Kind{EventEcho, EventTaskStart}, "", StateRunning},
		{"exit", []Kind{EventEcho, EventExit}, "", StateExited},
	}
	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			e := newEngine(t, 1)
			ev := e.HandleLine(tt.line)
			if got := kinds(ev); !reflect.DeepEqual(got, tt.kinds) {
				t.Fatalf("kinds = %v, want %v", got, tt.kinds)
			}
			if tt.want != "" && !strings.Contains(flatten(ev), tt.want) {
				t.Errorf("output %q does not contain %q", flatten(ev), tt.want)
			}
			if e.State() != tt.state {
				t.Errorf("state = %v, want %v", e.State(), tt.state)
			}
		})
	}
}

func flatten(events []Event) string {
	var b strings.Builder
	for _, ev := range events {
		b.WriteString(ev.Text + "\n" + strings.Join(ev.Lines, "\n") + "\n")
	}
	return b.String()
}

func TestRunForeverThenEsc(t *testing.T) {
	e := newEngine(t, 42)
	e.HandleLine("run engineering")

	if got := e.HandleLine("help"); len(got) != 1 || !strings.Contains(flatten(got), "Busy") {
		t.Fatalf("line while running = %+v", got)
	}

	done := 0
	for range 10000 { // ~17 fake minutes
		for _, ev := range e.Tick(100 * time.Millisecond) {
			if ev.Kind == EventTaskDone {
				done++
			}
		}
	}
	if done < 5 {
		t.Fatalf("only %d tasks finished; the loop should run forever", done)
	}
	if e.State() != StateRunning {
		t.Fatalf("state = %v, want running", e.State())
	}

	ev := e.HandleKey(KeyEsc)
	if got := kinds(ev); !reflect.DeepEqual(got, []Kind{EventStopped, EventPrompt}) {
		t.Fatalf("Esc kinds = %v", got)
	}
	if e.State() != StateIdle {
		t.Fatalf("state after Esc = %v, want idle (still in session)", e.State())
	}
	if ev := e.Tick(time.Hour); ev != nil {
		t.Fatalf("Tick while idle = %v, want nil", ev)
	}
	// Can start again after stopping.
	if got := kinds(e.HandleLine("run engineering")); got[1] != EventTaskStart {
		t.Fatalf("second run kinds = %v", got)
	}
}

func TestTickZeroOrNegative(t *testing.T) {
	e := newEngine(t, 1)
	e.HandleLine("run engineering")
	if e.Tick(0) != nil || e.Tick(-time.Second) != nil {
		t.Fatal("non-positive Tick must emit nothing")
	}
}

func TestKeys(t *testing.T) {
	e := newEngine(t, 1)
	if e.HandleKey(KeyEsc) != nil {
		t.Error("Esc while idle must do nothing")
	}
	if e.HandleKey(Key(99)) != nil {
		t.Error("unknown key must do nothing")
	}
	if got := kinds(e.HandleKey(KeyCtrlC)); !reflect.DeepEqual(got, []Kind{EventExit}) {
		t.Fatalf("Ctrl+C kinds = %v", got)
	}
	if e.HandleKey(KeyCtrlC) != nil || e.HandleLine("help") != nil || e.Tick(time.Second) != nil {
		t.Error("an exited session must ignore all input")
	}
}

func TestCtrlCWhileRunning(t *testing.T) {
	e := newEngine(t, 1)
	e.HandleLine("run engineering")
	e.Tick(10 * time.Second)
	e.HandleKey(KeyCtrlC)
	if e.State() != StateExited {
		t.Fatalf("state = %v, want exited", e.State())
	}
}

func TestDeterministicSession(t *testing.T) {
	record := func() []Event {
		e := newEngine(t, 99)
		out := e.Start()
		out = append(out, e.HandleLine("run engineering")...)
		for range 600 {
			out = append(out, e.Tick(100*time.Millisecond)...)
		}
		return append(out, e.HandleKey(KeyEsc)...)
	}
	if a, b := record(), record(); !reflect.DeepEqual(a, b) {
		t.Fatal("same seed and input produced different sessions")
	}
}

func TestSanitize(t *testing.T) {
	tests := map[string]string{
		"  run engineering  ":    "run engineering",
		"\x1b[31mred\x1b[0m":     "[31mred[0m",
		"tab\tnew\nline":         "tabnewline",
		"zero​width ":            "zerowidth",
		"bad\xffutf8":            "badutf8",
		strings.Repeat("é", 300): strings.Repeat("é", MaxInputLen),
	}
	for in, want := range tests {
		if got := Sanitize(in); got != want {
			t.Errorf("Sanitize(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestKindString(t *testing.T) {
	if EventExit.String() != "exit" || EventTaskStart.String() != "task_start" {
		t.Error("unexpected kind names")
	}
	if Kind(-1).String() != "unknown" || Kind(999).String() != "unknown" {
		t.Error("out-of-range kinds must be unknown")
	}
}

// TestRandomInputProperties drives sessions with random input and checks
// invariants that must always hold.
func TestRandomInputProperties(t *testing.T) {
	lines := []string{"run engineering", "help", "topics", "", "run nope", "hello", "\x1b]52;c;aGk=\x07"}
	for seed := range uint64(200) {
		rng := rand.New(rand.NewPCG(seed, 0))
		e := newEngine(t, seed)
		e.Start()
		for range 300 {
			var ev []Event
			switch rng.IntN(6) {
			case 0:
				ev = e.HandleLine(lines[rng.IntN(len(lines))])
			case 1:
				before := e.State()
				ev = e.HandleKey(KeyEsc)
				if before != StateExited && e.State() != StateIdle {
					t.Fatalf("seed %d: Esc left state %v", seed, e.State())
				}
			case 2:
				if rng.IntN(20) == 0 {
					ev = e.HandleKey(KeyCtrlC)
					if e.State() != StateExited {
						t.Fatalf("seed %d: Ctrl+C did not exit", seed)
					}
				}
			default:
				ev = e.Tick(time.Duration(rng.IntN(5000)) * time.Millisecond)
			}
			for _, x := range ev {
				if strings.ContainsAny(flatten([]Event{x}), "\x1b\x07") {
					t.Fatalf("seed %d: escape characters reached an event: %+v", seed, x)
				}
			}
		}
	}
}

func FuzzHandleLine(f *testing.F) {
	f.Add("run engineering")
	f.Add("\x1b[2J")
	f.Fuzz(func(t *testing.T, line string) {
		e := newEngine(t, 1)
		for _, ev := range e.HandleLine(line) {
			if strings.ContainsAny(flatten([]Event{ev}), "\x1b\x07\r") {
				t.Fatalf("escape characters reached an event: %+v", ev)
			}
		}
	})
}
