// Package render turns engine events into ANSI text.
//
// It is the only package allowed to produce escape sequences. Both the CLI and
// the web frontend print its output unchanged, so they look identical.
//
// Output has two parts: permanent lines produced by Event, which scroll up,
// and a live area produced by Live, which sits below them and is redrawn in
// place:
//
//	◐ Pondering… (3s · Esc to stop)      or      right-aligned status
//	─────────────────────────────────────
//	❯ typed input█
//	─────────────────────────────────────
//	  ▸▸ mode · hints
package render

import (
	"fmt"
	"hash/fnv"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/boringemployee/be/internal/engine"
	"github.com/boringemployee/be/internal/profile"
)

// Escape sequences. Lines end with CRLF so output is correct in raw mode
// terminals and in xterm.js alike.
const (
	NL        = "\r\n"
	ClearLine = "\r\x1b[2K"
	reset     = "\x1b[0m"
	bold      = "\x1b[1m"
	dim       = "\x1b[2m"
	red       = "\x1b[31m"
	green     = "\x1b[32m"
	yellow    = "\x1b[33m"
)

// MinWidth is the narrowest width the renderer lays out for.
const MinWidth = 20

// liveInputRow is the row of the input line within the live area (0-based).
const liveInputRow = 2

// Renderer formats events for one agent at a given terminal width.
type Renderer struct {
	agent  *profile.Agent
	accent string
	width  int
	cwd    string
}

// New returns a renderer for agent at width columns.
func New(agent *profile.Agent, width int) *Renderer {
	r := &Renderer{agent: agent, accent: accentCode(agent.Accent), cwd: "~"}
	r.SetWidth(width)
	return r
}

// SetWidth updates the terminal width; lines longer than it are truncated.
func (r *Renderer) SetWidth(width int) { r.width = max(width, MinWidth) }

// SetCwd sets the folder shown in the header. It is display-only and is
// sanitized like user input.
func (r *Renderer) SetCwd(cwd string) {
	if cwd = engine.Sanitize(cwd); cwd != "" {
		r.cwd = cwd
	}
}

// accentCode converts a validated #rrggbb color into a 24-bit foreground code.
func accentCode(hex string) string {
	v, _ := strconv.ParseUint(strings.TrimPrefix(hex, "#"), 16, 32)
	return fmt.Sprintf("\x1b[38;2;%d;%d;%dm", v>>16&0xff, v>>8&0xff, v&0xff)
}

func paint(code, s string) string {
	if code == "" || s == "" {
		return s
	}
	return code + s + reset
}

func width(s string) int { return utf8.RuneCountInString(s) }

// fit truncates s to n visible characters, adding an ellipsis when cut.
func fit(s string, n int) string {
	if width(s) <= n {
		return s
	}
	if n <= 0 {
		return ""
	}
	runes := []rune(s)
	return string(runes[:n-1]) + "…"
}

// wrap splits s into lines of at most n characters at spaces. Words longer
// than n are truncated.
func wrap(s string, n int) []string {
	var lines []string
	cur := ""
	for _, word := range strings.Fields(s) {
		switch {
		case cur == "":
			cur = word
		case width(cur)+1+width(word) <= n:
			cur += " " + word
		default:
			lines = append(lines, fit(cur, n))
			cur = word
		}
	}
	return append(lines, fit(cur, n))
}

// line formats one output line: indent, optional colored marker, then text
// fitted to the remaining width.
func (r *Renderer) line(indent, marker, markerCode, textCode, text string) string {
	room := r.width - width(indent)
	if marker != "" {
		room -= width(marker) + 1
		marker = paint(markerCode, marker) + " "
	}
	return indent + marker + paint(textCode, fit(text, room)) + NL
}

// Event returns the permanent output for ev. Some events, such as prompts and
// thinking, only change the live area and return "".
func (r *Renderer) Event(ev engine.Event) string {
	var b strings.Builder
	boxed := r.agent.Layout == profile.LayoutBoxed
	switch ev.Kind {
	case engine.EventWelcome:
		b.WriteString(r.header())
		b.WriteString(r.notice(ev.Lines))
	case engine.EventEcho:
		b.WriteString(r.line("", "", "", dim, r.agent.Prompt+ev.Text))
	case engine.EventNotice:
		for _, l := range ev.Lines {
			b.WriteString(r.line("  ", "", "", "", l))
		}
	case engine.EventError:
		b.WriteString(r.line("  ", "✗", red, red, ev.Text))
	case engine.EventTaskStart:
		b.WriteString(NL + r.line("", "●", r.accent, bold, ev.Text))
	case engine.EventRead:
		b.WriteString(r.tool(boxed, "Read", ev.Text))
		b.WriteString(r.line("    ", "⎿", dim, dim, fmt.Sprintf("Read %d lines", lineCount(ev.Text))))
	case engine.EventEdit:
		b.WriteString(r.tool(boxed, "Update", ev.Text))
		for _, l := range ev.Lines {
			code := green
			if l[0] == '-' {
				code = red
			}
			b.WriteString(r.line("      ", "", "", code, l))
		}
	case engine.EventCommand:
		b.WriteString(r.tool(boxed, "Run", ev.Text))
		for i, l := range ev.Lines {
			code := dim
			if i == len(ev.Lines)-1 {
				code = red
				if ev.OK {
					code = green
				}
			}
			b.WriteString(r.line("      ", "", "", code, l))
		}
	case engine.EventTaskDone:
		b.WriteString(r.line("", "●", green, "", ev.Text))
	case engine.EventStopped:
		b.WriteString(r.line("  ", "⎿", yellow, yellow, ev.Text) + NL)
	case engine.EventExit:
		b.WriteString(NL + r.line("", "", "", dim, "Session ended. Back to real work."))
	}
	return b.String()
}

// header draws the mascot on the left and "Title version" plus the header
// lines on the right.
func (r *Renderer) header() string {
	a := r.agent
	art := 0
	for _, m := range a.Mascot {
		art = max(art, width(m))
	}
	gap := 0
	if art > 0 {
		gap = 3
	}
	room := r.width - 1 - art - gap
	right := []string{paint(bold, fit(a.Title, room)) + paint(dim, fit(" "+a.Version, room-width(a.Title)))}
	for _, h := range a.Header {
		right = append(right, paint(dim, fit(strings.ReplaceAll(h, "{cwd}", r.cwd), room)))
	}
	var b strings.Builder
	b.WriteString(NL)
	for i := range max(len(a.Mascot), len(right)) {
		b.WriteString(" ")
		if art > 0 {
			m := ""
			if i < len(a.Mascot) {
				m = a.Mascot[i]
			}
			b.WriteString(paint(r.accent, m) + strings.Repeat(" ", art-width(m)+gap))
		}
		if i < len(right) {
			b.WriteString(right[i])
		}
		b.WriteString(NL)
	}
	return b.String() + NL
}

// notice draws a block with an accent bar; the first line is bold.
func (r *Renderer) notice(lines []string) string {
	var b strings.Builder
	for i, l := range lines {
		code := dim
		if i == 0 {
			code = bold
		}
		for _, w := range wrap(l, r.width-4) {
			b.WriteString(" " + paint(r.accent, "▎") + " " + paint(code, w) + NL)
		}
	}
	return b.String() + NL
}

// tool formats a tool call header in the agent's layout.
func (r *Renderer) tool(boxed bool, name, arg string) string {
	if boxed {
		return r.line("  ", "●", r.accent, "", name+"("+arg+")")
	}
	return r.line("  ", "→", r.accent, "", strings.ToLower(name)+" "+arg)
}

// lineCount derives a stable, plausible file length from its path.
func lineCount(path string) int {
	h := fnv.New32a()
	h.Write([]byte(path))
	return 40 + int(h.Sum32()%460)
}

// LiveState is what the live area shows.
type LiveState struct {
	Running bool
	Frame   int
	Word    string
	Elapsed time.Duration
	Input   string
}

// Live returns the live area and leaves the cursor at the end of the input,
// ready for ClearLive. It starts at the beginning of an empty line.
func (r *Renderer) Live(s LiveState) string {
	a := r.agent
	var b strings.Builder

	if s.Running {
		spin := a.Spinner[s.Frame%len(a.Spinner)]
		text := fmt.Sprintf("%s… (%ds · Esc to stop)", s.Word, int(s.Elapsed/time.Second))
		b.WriteString(r.line("", spin, r.accent, dim, text))
	} else {
		status := fit(a.StatusRight, r.width-1)
		b.WriteString(strings.Repeat(" ", r.width-1-width(status)) + paint(dim, status) + NL)
	}

	rule := paint(dim, strings.Repeat("─", r.width-1)) + NL
	marker := a.Prompt
	room := r.width - 1 - width(marker)
	input := s.Input
	if runes := []rune(input); len(runes) > room {
		input = "…" + string(runes[len(runes)-room+1:])
	}
	b.WriteString(rule)
	b.WriteString(paint(r.accent, marker) + input + NL)
	b.WriteString(rule)

	footer := ""
	if a.FooterMode != "" {
		footer = paint(yellow, fit(a.FooterMode, r.width-3))
	}
	if hint := fit(a.FooterHint, r.width-3-width(a.FooterMode)-1); hint != "" {
		if footer != "" {
			footer += " "
		}
		footer += paint(dim, hint)
	}
	b.WriteString("  " + footer)

	// Move back up to the input line and place the cursor after the text.
	fmt.Fprintf(&b, "\x1b[%dA\r", 4-liveInputRow)
	if col := width(marker) + width(input); col > 0 {
		fmt.Fprintf(&b, "\x1b[%dC", col)
	}
	return b.String()
}

// ClearLive erases a live area drawn by Live, leaving the cursor where it
// started so permanent output can be written there.
func (r *Renderer) ClearLive() string {
	return fmt.Sprintf("\x1b[%dA\r\x1b[J", liveInputRow)
}
