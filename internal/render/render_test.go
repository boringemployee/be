package render

import (
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/boringemployee/be/content"
	"github.com/boringemployee/be/internal/engine"
	"github.com/boringemployee/be/internal/profile"
)

func lines(s ...string) *[]string { return &s }

func agent(tools profile.ToolStyle, live ...profile.Block) *profile.Agent {
	if live == nil {
		live = []profile.Block{
			{Status: &profile.Status{Idle: "low"}},
			{Input: &profile.Input{Style: profile.InputRuled, Marker: ">"}},
			{Footer: &profile.Footer{Highlight: "mode", Left: "hint"}},
		}
	}
	return &profile.Agent{
		Title: "Demo", Version: "v1.0.0", Accent: "#D97757", Tools: tools,
		Spinner: []string{"a", "b"}, Live: live,
		Welcome: []profile.Block{{Header: &profile.Header{Art: []string{"##", "#"}, Lines: []string{"{title} {version}", "{cwd}", "extra"}}}},
	}
}

var ansiRE = regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]`)

// plain strips the escape sequences the renderer itself produces.
func plain(s string) string { return ansiRE.ReplaceAllString(s, "") }

func TestColorCode(t *testing.T) {
	if got := colorCode("#D97757"); got != "\x1b[38;2;217;119;87m" {
		t.Fatalf("colorCode = %q", got)
	}
}

func TestEvents(t *testing.T) {
	tests := []struct {
		name  string
		tools profile.ToolStyle
		ev    engine.Event
		want  string
	}{
		{"welcome", profile.ToolsBoxed, engine.Event{Kind: engine.EventWelcome}, "\r\n ##   Demo v1.0.0\r\n #    ~\r\n      extra\r\n\r\n"},
		{"prompt", profile.ToolsBoxed, engine.Event{Kind: engine.EventPrompt}, ""},
		{"thinking", profile.ToolsBoxed, engine.Event{Kind: engine.EventThinking, Text: "Hmm"}, ""},
		{"echo", profile.ToolsBoxed, engine.Event{Kind: engine.EventEcho, Text: "help"}, "> help\r\n"},
		{"notice", profile.ToolsBoxed, engine.Event{Kind: engine.EventNotice, Lines: []string{"a", "b"}}, "  a\r\n  b\r\n"},
		{"error", profile.ToolsBoxed, engine.Event{Kind: engine.EventError, Text: "bad"}, "  ✗ bad\r\n"},
		{"task start", profile.ToolsBoxed, engine.Event{Kind: engine.EventTaskStart, Text: "Fix it"}, "\r\n● Fix it\r\n"},
		{"read boxed", profile.ToolsBoxed, engine.Event{Kind: engine.EventRead, Text: "a.go"}, "  ● Read(a.go)\r\n    ⎿ Read "},
		{"read inline", profile.ToolsInline, engine.Event{Kind: engine.EventRead, Text: "a.go"}, "  → read a.go\r\n    └ Read "},
		{"read bullet", profile.ToolsBullet, engine.Event{Kind: engine.EventRead, Text: "a.go"}, "  • Read a.go\r\n"},
		{"read check", profile.ToolsCheck, engine.Event{Kind: engine.EventRead, Text: "a.go"}, "  ✓ ReadFile a.go\r\n"},
		{"edit", profile.ToolsBoxed, engine.Event{Kind: engine.EventEdit, Text: "a.go", Lines: []string{"-old", "+new"}}, "  ● Update(a.go)\r\n      -old\r\n      +new\r\n"},
		{"edit bullet", profile.ToolsBullet, engine.Event{Kind: engine.EventEdit, Text: "a.go", Lines: []string{"+new"}}, "  • Edited a.go\r\n"},
		{"command ok", profile.ToolsBoxed, engine.Event{Kind: engine.EventCommand, Text: "make", Lines: []string{"log", "PASS x"}, OK: true}, "  ● Run(make)\r\n      log\r\n      PASS x\r\n"},
		{"command check", profile.ToolsCheck, engine.Event{Kind: engine.EventCommand, Text: "make", Lines: []string{"FAIL x"}}, "  ✓ Shell make\r\n      FAIL x\r\n"},
		{"done", profile.ToolsBoxed, engine.Event{Kind: engine.EventTaskDone, Text: "Done"}, "● Done\r\n"},
		{"stopped", profile.ToolsBoxed, engine.Event{Kind: engine.EventStopped, Text: "Stopped"}, "  ⎿ Stopped\r\n\r\n"},
		{"exit", profile.ToolsBoxed, engine.Event{Kind: engine.EventExit}, "\r\nSession ended. Back to real work.\r\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := plain(New(agent(tt.tools), 80).Event(tt.ev))
			if !strings.HasPrefix(got, tt.want) {
				t.Fatalf("got %q, want prefix %q", got, tt.want)
			}
		})
	}
}

func TestColors(t *testing.T) {
	r := New(agent(profile.ToolsBoxed), 80)
	edit := r.Event(engine.Event{Kind: engine.EventEdit, Text: "a.go", Lines: []string{"-old", "+new"}})
	if !strings.Contains(edit, red+"-old") || !strings.Contains(edit, green+"+new") {
		t.Errorf("diff colors wrong: %q", edit)
	}
	ok := r.Event(engine.Event{Kind: engine.EventCommand, Text: "m", Lines: []string{"PASS"}, OK: true})
	fail := r.Event(engine.Event{Kind: engine.EventCommand, Text: "m", Lines: []string{"FAIL"}})
	if !strings.Contains(ok, green+"PASS") || !strings.Contains(fail, red+"FAIL") {
		t.Errorf("result colors wrong: %q %q", ok, fail)
	}
	if head := r.Event(engine.Event{Kind: engine.EventWelcome}); !strings.Contains(head, bold+"Demo "+reset+dim+"v1.0.0"+reset) {
		t.Errorf("version should be dim next to a bold title: %q", head)
	}
}

func welcome(t *testing.T, w int, blocks ...profile.Block) string {
	t.Helper()
	a := agent(profile.ToolsBoxed)
	a.Welcome = blocks
	r := New(a, w)
	r.SetCwd("\x1b[2J~/work") // sanitized
	r.SetCwd("")              // ignored
	return plain(r.Event(engine.Event{Kind: engine.EventWelcome}))
}

func TestWelcomeBlocks(t *testing.T) {
	tests := []struct {
		name  string
		width int
		block profile.Block
		want  string
	}{
		{"header without art", 80, profile.Block{Header: &profile.Header{Lines: []string{"{title}", "{cwd}"}}}, " Demo\r\n [2J~/work\r\n"},
		{"header truncated version", 20, profile.Block{Header: &profile.Header{Lines: []string{"{title} {version} and more"}}}, " Demo v1.0.0 and m…\r\n"},
		{"card", 80, profile.Block{Card: lines("Title {version}", "body")}, " ╭──────────────╮\r\n │ Title v1.0.0 │\r\n │ body         │\r\n ╰──────────────╯\r\n"},
		{"card narrow", 20, profile.Block{Card: lines("a long title that will not fit")}, " ╭────────────────╮\r\n │ a long title … │\r\n ╰────────────────╯\r\n"},
		{"notice wraps", 30, profile.Block{Notice: lines("Bold", "one two three four five six seven")}, " ▎ Bold\r\n ▎ one two three four five\r\n ▎ six seven\r\n"},
		{"bullet list", 80, profile.Block{List: &profile.List{Title: "Tips:", Items: []string{"a", "b"}}}, " Tips:\r\n • a\r\n • b\r\n"},
		{"numbered list", 80, profile.Block{List: &profile.List{Items: []string{"a", "b"}, Numbered: true}}, " 1. a\r\n 2. b\r\n"},
		{"text", 80, profile.Block{Text: lines("in {cwd}")}, " in [2J~/work\r\n"},
		{"rule", 20, profile.Block{Rule: &profile.Rule{}}, strings.Repeat("─", 19) + "\r\n"},
		{"banner", 80, profile.Block{Banner: &profile.Banner{Text: "HI"}}, " █   █ ███\r\n █   █  █ \r\n █████  █ \r\n █   █  █ \r\n █   █ ███\r\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := welcome(t, tt.width, tt.block); got != "\r\n"+tt.want+"\r\n" {
				t.Fatalf("got\n%q\nwant\n%q", got, "\r\n"+tt.want+"\r\n")
			}
		})
	}
}

func TestBannerClippedAndColored(t *testing.T) {
	a := agent(profile.ToolsBoxed)
	a.Welcome = []profile.Block{{Banner: &profile.Banner{Text: "WWWWWW", Gradient: []string{"#000000", "#ffffff"}}}}
	out := New(a, 20).Event(engine.Event{Kind: engine.EventWelcome})
	for _, l := range strings.Split(plain(out), NL) {
		if width(l) > 19 {
			t.Fatalf("banner row %q is wider than the terminal", l)
		}
	}
	if !strings.Contains(out, "\x1b[38;2;0;0;0m█") {
		t.Errorf("banner should start with the first gradient color: %q", out[:60])
	}
}

func TestGradient(t *testing.T) {
	stops := []string{"#000000", "#ff0000", "#ffffff"}
	tests := map[int]string{0: "0;0;0", 2: "255;0;0", 4: "255;255;255", 1: "128;0;0"}
	for i, want := range tests {
		if got := gradient(stops, i, 5); got != "\x1b[38;2;"+want+"m" {
			t.Errorf("gradient(%d) = %q, want %s", i, got, want)
		}
	}
	if gradient([]string{"#000000", "#ffffff"}, 0, 1) != colorCode("#000000") {
		t.Error("single column must use the first color")
	}
}

func TestFontCoversBannerCharacters(t *testing.T) {
	for _, ch := range "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789 .!-" {
		g, ok := font[ch]
		if !ok {
			t.Fatalf("font has no glyph for %q", ch)
		}
		if !profile.BannerRE.MatchString(string(ch)) {
			t.Errorf("profile does not allow %q", ch)
		}
		for _, row := range g {
			if width(row) != width(g[0]) {
				t.Errorf("glyph %q has uneven rows", ch)
			}
		}
	}
	if rows := bannerRows("A~"); rows[0] != " ███ " {
		t.Errorf("unknown characters must be skipped: %q", rows[0])
	}
}

func TestTruncation(t *testing.T) {
	r := New(agent(profile.ToolsBoxed), 5) // clamped to MinWidth
	got := plain(r.Event(engine.Event{Kind: engine.EventNotice, Lines: []string{strings.Repeat("é", 50)}}))
	if want := "  " + strings.Repeat("é", MinWidth-3) + "…\r\n"; got != want {
		t.Fatalf("got %q, want %q", got, want)
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

func TestLineCountStable(t *testing.T) {
	a, b := lineCount("a.go"), lineCount("a.go")
	if a != b || a < 40 || a >= 500 {
		t.Fatalf("lineCount = %d, %d", a, b)
	}
}

func TestLiveRuled(t *testing.T) {
	r := New(agent(profile.ToolsBoxed), 20)
	idle := r.Live(LiveState{Input: "run"})
	rows := strings.Split(plain(idle), NL)
	want := []string{strings.Repeat(" ", 16) + "low", strings.Repeat("─", 19), "> run", strings.Repeat("─", 19), "  mode hint\r"}
	if strings.Join(rows, "|") != strings.Join(want, "|") {
		t.Fatalf("rows = %q, want %q", rows, want)
	}
	if !strings.HasSuffix(idle, "\x1b[2A\r\x1b[5C") {
		t.Errorf("cursor should return to the input line: %q", idle[len(idle)-12:])
	}
	if got := r.ClearLive(); got != "\x1b[2A\r\x1b[J" {
		t.Errorf("ClearLive = %q", got)
	}

	running := plain(New(agent(profile.ToolsBoxed), 80).LiveArea(LiveState{Running: true, Frame: 3, Word: "Hmm", Elapsed: 12500 * time.Millisecond}))
	if !strings.HasPrefix(running, "b Hmm… (12s · Esc to stop)\r\n") {
		t.Errorf("running status = %q", running)
	}

	long := strings.Split(plain(r.LiveArea(LiveState{Input: strings.Repeat("ab", 30)})), NL)[2]
	if n := width(long); n != 19 || !strings.HasPrefix(long, "> …") || !strings.HasSuffix(long, "ab") {
		t.Errorf("long input = %q (%d columns)", long, n)
	}
}

func TestLiveBareInputFirst(t *testing.T) {
	a := agent(profile.ToolsBoxed,
		profile.Block{Input: &profile.Input{Style: profile.InputBare, Marker: "›", Placeholder: "Ask anything"}},
		profile.Block{Status: &profile.Status{}},
	)
	r := New(a, 30)
	got := r.Live(LiveState{})
	if rows := strings.Split(plain(got), NL); rows[0] != "› Ask anything" {
		t.Errorf("rows = %q", rows)
	}
	if !strings.HasSuffix(got, "\x1b[1A\r\x1b[2C") {
		t.Errorf("cursor = %q", got[len(got)-10:])
	}
	if r.ClearLive() != "\r\x1b[J" {
		t.Errorf("ClearLive with input on the first row = %q", r.ClearLive())
	}
	// Input as the last row with an empty marker: no cursor movement needed.
	a.Live = []profile.Block{{Status: &profile.Status{}}, {Input: &profile.Input{Style: profile.InputBare}}}
	if got := New(a, 30).Live(LiveState{}); strings.Contains(got, "A") || !strings.HasSuffix(got, "\r\x1b[1C") {
		t.Errorf("live = %q", got)
	}
}

func TestLiveBoxed(t *testing.T) {
	a := agent(profile.ToolsCheck,
		profile.Block{Status: &profile.Status{}},
		profile.Block{Input: &profile.Input{Style: profile.InputBoxed, Marker: ">", Placeholder: "Type your message"}},
		profile.Block{Footer: &profile.Footer{Left: "{cwd}", Center: "no box", Right: "model"}},
	)
	r := New(a, 40)
	rows := strings.Split(plain(r.Live(LiveState{})), NL)
	want := []string{
		strings.Repeat(" ", 39),
		"╭" + strings.Repeat("─", 37) + "╮",
		"│ > Type your message                 │",
		"╰" + strings.Repeat("─", 37) + "╯",
		"  ~             no box            model",
	}
	if strings.Join(rows[:4], "|") != strings.Join(want[:4], "|") || !strings.HasPrefix(rows[4], want[4]) {
		t.Fatalf("rows =\n%q\nwant\n%q", rows, want)
	}
	typed := strings.Split(plain(r.LiveArea(LiveState{Input: "hi"})), NL)[2]
	if typed != "│ > hi"+strings.Repeat(" ", 32)+"│" {
		t.Errorf("typed = %q", typed)
	}
}

func TestFooterDropsWhatDoesNotFit(t *testing.T) {
	r := New(agent(profile.ToolsBoxed), 20)
	got := plain(r.footer(&profile.Footer{Left: "a very long left side", Center: "c", Right: "r"}))
	if got != "  a very long left…" {
		t.Errorf("footer = %q", got)
	}
	if got := plain(r.footer(&profile.Footer{})); got != "  " {
		t.Errorf("empty footer = %q", got)
	}
}

func TestPreview(t *testing.T) {
	cat, err := profile.Load(content.FS)
	if err != nil {
		t.Fatal(err)
	}
	e, _ := engine.New(cat, "claude", 1)
	r := New(e.Agent(), 80)
	out := plain(r.Preview(e, "engineering"))
	for _, want := range []string{"Clock Code", "❯ run engineering", "●", "○ low"} {
		if !strings.Contains(out, want) {
			t.Errorf("preview missing %q", want)
		}
	}
	if e.State() != engine.StateIdle {
		t.Errorf("preview must leave the session idle, got %v", e.State())
	}
	e2, _ := engine.New(cat, "claude", 1)
	if out := plain(r.Preview(e2, "nope")); !strings.Contains(out, "unknown topic") {
		t.Errorf("preview of unknown topic = %q", out)
	}
}

func TestLiveWithRule(t *testing.T) {
	a := agent(profile.ToolsBoxed,
		profile.Block{Status: &profile.Status{}},
		profile.Block{Rule: &profile.Rule{}},
		profile.Block{Input: &profile.Input{Style: profile.InputBare, Marker: ">"}},
	)
	r := New(a, 20)
	rows := strings.Split(plain(r.LiveArea(LiveState{})), NL)
	if len(rows) != 3 || rows[1] != strings.Repeat("─", 19) || rows[2] != "> " {
		t.Fatalf("rows = %q", rows)
	}
	r.Live(LiveState{})
	if r.ClearLive() != "\x1b[2A\r\x1b[J" {
		t.Errorf("ClearLive = %q", r.ClearLive())
	}
}
