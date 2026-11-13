// Package render turns engine events into ANSI text.
//
// It is the only package allowed to produce escape sequences. Both the CLI and
// the web frontend print its output unchanged, so they look identical.
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

// Renderer formats events for one agent at a given terminal width.
type Renderer struct {
	agent  *profile.Agent
	accent string
	width  int
}

// New returns a renderer for agent at width columns.
func New(agent *profile.Agent, width int) *Renderer {
	r := &Renderer{agent: agent, accent: accentCode(agent.Accent)}
	r.SetWidth(width)
	return r
}

// SetWidth updates the terminal width; lines longer than it are truncated.
func (r *Renderer) SetWidth(width int) { r.width = max(width, MinWidth) }

// accentCode converts a validated #rrggbb color into a 24-bit foreground code.
func accentCode(hex string) string {
	v, _ := strconv.ParseUint(strings.TrimPrefix(hex, "#"), 16, 32)
	return fmt.Sprintf("\x1b[38;2;%d;%d;%dm", v>>16&0xff, v>>8&0xff, v&0xff)
}

func paint(code, s string) string { return code + s + reset }

// fit truncates s to n visible characters, adding an ellipsis when cut.
func fit(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	runes := []rune(s)
	return string(runes[:n-1]) + "…"
}

// line formats one output line: indent, optional colored marker, then text
// fitted to the remaining width.
func (r *Renderer) line(indent, marker, markerCode, textCode, text string) string {
	room := r.width - utf8.RuneCountInString(indent)
	if marker != "" {
		room -= utf8.RuneCountInString(marker) + 1
		marker = paint(markerCode, marker) + " "
	}
	text = fit(text, room)
	if textCode != "" {
		text = paint(textCode, text)
	}
	return indent + marker + text + NL
}

// Event returns the permanent output for ev. Some events, such as prompts and
// thinking, only change the live line and return "".
func (r *Renderer) Event(ev engine.Event) string {
	var b strings.Builder
	boxed := r.agent.Layout == profile.LayoutBoxed
	switch ev.Kind {
	case engine.EventWelcome:
		b.WriteString(r.line("", "", "", bold+r.accent, ev.Text))
		for _, l := range ev.Lines[:len(ev.Lines)-1] {
			b.WriteString(r.line("", "", "", r.accent, l))
		}
		b.WriteString(r.line("", "", "", dim, ev.Lines[len(ev.Lines)-1]))
		b.WriteString(NL)
	case engine.EventEcho:
		b.WriteString(r.line("", "", "", dim, r.agent.Prompt+ev.Text))
	case engine.EventNotice:
		for _, l := range ev.Lines {
			b.WriteString(r.line("", "", "", "", l))
		}
	case engine.EventError:
		b.WriteString(r.line("", "✗", red, red, ev.Text))
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

// Status returns the live status line shown while a task runs.
func (r *Renderer) Status(frame int, word string, elapsed time.Duration) string {
	spin := r.agent.Spinner[frame%len(r.agent.Spinner)]
	text := fmt.Sprintf("%s… (%ds · Esc to stop)", word, int(elapsed/time.Second))
	return ClearLine + strings.TrimSuffix(r.line("", spin, r.accent, dim, text), NL)
}

// Input returns the live input line: the prompt followed by what the user is
// typing. Only the end of long input is shown, like a scrolling text box.
func (r *Renderer) Input(text string) string {
	room := r.width - utf8.RuneCountInString(r.agent.Prompt) - 1
	if runes := []rune(text); len(runes) > room {
		text = "…" + string(runes[len(runes)-room+1:])
	}
	return ClearLine + paint(r.accent, r.agent.Prompt) + text
}
