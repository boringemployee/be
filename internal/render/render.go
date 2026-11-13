// Package render turns engine events into ANSI text.
//
// It is the only package allowed to produce escape sequences. Both the CLI and
// the web frontend print its output unchanged, so they look identical.
//
// The look of each agent comes from its layout in YAML: a list of welcome
// blocks drawn once, and live blocks (status, input, footer, rules) drawn
// under the output and redrawn in place. This package interprets those blocks;
// adding an agent needs no Go code.
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
	agent    *profile.Agent
	accent   string
	width    int
	cwd      string
	inputRow int // row of the input line in the last live area drawn
}

// New returns a renderer for agent at width columns.
func New(agent *profile.Agent, width int) *Renderer {
	r := &Renderer{agent: agent, accent: colorCode(agent.Accent), cwd: "~"}
	r.SetWidth(width)
	return r
}

// SetWidth updates the terminal width; lines longer than it are truncated.
func (r *Renderer) SetWidth(width int) { r.width = max(width, MinWidth) }

// SetCwd sets the folder shown by {cwd}. It is display-only and is sanitized
// like user input.
func (r *Renderer) SetCwd(cwd string) {
	if cwd = engine.Sanitize(cwd); cwd != "" {
		r.cwd = cwd
	}
}

// rgb parses a validated #rrggbb color.
func rgb(hex string) (int, int, int) {
	v, _ := strconv.ParseUint(strings.TrimPrefix(hex, "#"), 16, 32)
	return int(v >> 16 & 0xff), int(v >> 8 & 0xff), int(v & 0xff)
}

// colorCode converts a validated #rrggbb color into a 24-bit foreground code.
func colorCode(hex string) string {
	cr, cg, cb := rgb(hex)
	return fmt.Sprintf("\x1b[38;2;%d;%d;%dm", cr, cg, cb)
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
	return string([]rune(s)[:n-1]) + "…"
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

// expand fills {title}, {version} and {cwd}.
func (r *Renderer) expand(s string) string {
	return strings.NewReplacer("{title}", r.agent.Title, "{version}", r.agent.Version, "{cwd}", r.cwd).Replace(s)
}

// text expands a layout string, fits it to room and paints it with code. When
// it fits, {version} is shown dim, like the version next to a product name.
func (r *Renderer) text(s, code string, room int) string {
	full := r.expand(s)
	if width(full) > room || !strings.Contains(s, "{version}") {
		return paint(code, fit(full, room))
	}
	var b strings.Builder
	for i, part := range strings.Split(s, "{version}") {
		if i > 0 {
			b.WriteString(paint(dim, r.agent.Version))
		}
		b.WriteString(paint(code, r.expand(part)))
	}
	return b.String()
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
	switch ev.Kind {
	case engine.EventWelcome:
		b.WriteString(NL)
		for _, blk := range r.agent.Welcome {
			b.WriteString(r.block(blk) + NL)
		}
	case engine.EventEcho:
		b.WriteString(r.line("", "", "", dim, r.marker()+" "+ev.Text))
	case engine.EventNotice:
		for _, l := range ev.Lines {
			b.WriteString(r.line("  ", "", "", "", l))
		}
	case engine.EventError:
		b.WriteString(r.line("  ", "✗", red, red, ev.Text))
	case engine.EventTaskStart:
		b.WriteString(NL + r.line("", "●", r.accent, bold, ev.Text))
	case engine.EventRead:
		b.WriteString(r.tool(toolRead, ev.Text))
		b.WriteString(r.result(fmt.Sprintf("Read %d lines", lineCount(ev.Text))))
	case engine.EventEdit:
		b.WriteString(r.tool(toolEdit, ev.Text))
		for _, l := range ev.Lines {
			code := green
			if l[0] == '-' {
				code = red
			}
			b.WriteString(r.line("      ", "", "", code, l))
		}
	case engine.EventCommand:
		b.WriteString(r.tool(toolRun, ev.Text))
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
		b.WriteString(NL + r.line("", "", "", dim, "Session ended."))
	}
	return b.String()
}

// marker returns the input marker, e.g. "❯".
func (r *Renderer) marker() string { return r.agent.InputBlock().Marker }

// block draws one welcome block.
func (r *Renderer) block(b profile.Block) string {
	switch {
	case b.Header != nil:
		return r.header(b.Header)
	case b.Banner != nil:
		return r.banner(b.Banner)
	case b.Card != nil:
		return r.card(*b.Card)
	case b.Notice != nil:
		return r.notice(*b.Notice)
	case b.List != nil:
		return r.list(b.List)
	case b.Text != nil:
		var s strings.Builder
		for _, l := range *b.Text {
			s.WriteString(" " + r.text(l, dim, r.width-2) + NL)
		}
		return s.String()
	default: // rule
		return r.rule() + NL
	}
}

func (r *Renderer) rule() string { return paint(dim, strings.Repeat("─", r.width-1)) }

// header draws art on the left and lines on the right; the first line is
// bold and the rest dim.
func (r *Renderer) header(h *profile.Header) string {
	art := 0
	for _, a := range h.Art {
		art = max(art, width(a))
	}
	gap := 0
	if art > 0 {
		gap = 3
	}
	room := r.width - 2 - art - gap
	var b strings.Builder
	for i := range max(len(h.Art), len(h.Lines)) {
		b.WriteString(" ")
		if art > 0 {
			a := ""
			if i < len(h.Art) {
				a = h.Art[i]
			}
			b.WriteString(paint(r.accent, a) + strings.Repeat(" ", art-width(a)+gap))
		}
		if i < len(h.Lines) {
			code := dim
			if i == 0 {
				code = bold
			}
			b.WriteString(r.text(h.Lines[i], code, room))
		}
		b.WriteString(NL)
	}
	return b.String()
}

// banner draws block letters with a left-to-right color gradient.
func (r *Renderer) banner(bn *profile.Banner) string {
	rows := bannerRows(bn.Text)
	cols := width(rows[0])
	stops := bn.Gradient
	if len(stops) == 0 {
		stops = []string{r.agent.Accent}
	}
	var b strings.Builder
	for _, row := range rows {
		b.WriteString(" ")
		for i, ch := range []rune(row) {
			if i >= r.width-2 {
				break
			}
			if ch == ' ' {
				b.WriteRune(ch)
				continue
			}
			b.WriteString(paint(gradient(stops, i, cols), string(ch)))
		}
		b.WriteString(NL)
	}
	return b.String()
}

// gradient returns the color code for column i of n across stops.
func gradient(stops []string, i, n int) string {
	if len(stops) == 1 || n < 2 {
		return colorCode(stops[0])
	}
	pos := float64(i) / float64(n-1) * float64(len(stops)-1)
	seg := min(int(pos), len(stops)-2)
	t := pos - float64(seg)
	r1, g1, b1 := rgb(stops[seg])
	r2, g2, b2 := rgb(stops[seg+1])
	mix := func(a, b int) int { return a + int(float64(b-a)*t+0.5) }
	return fmt.Sprintf("\x1b[38;2;%d;%d;%dm", mix(r1, r2), mix(g1, g2), mix(b1, b2))
}

// card draws lines in a rounded box; the first line is bold.
func (r *Renderer) card(lines []string) string {
	inner := 0
	for _, l := range lines {
		inner = max(inner, width(r.expand(l)))
	}
	inner = min(inner, r.width-6)
	edge := func(left, right string) string {
		return " " + paint(r.accent, left+strings.Repeat("─", inner+2)+right) + NL
	}
	var b strings.Builder
	b.WriteString(edge("╭", "╮"))
	for i, l := range lines {
		code := ""
		if i == 0 {
			code = bold
		}
		body := r.text(l, code, inner)
		body += strings.Repeat(" ", inner-width(fit(r.expand(l), inner)))
		b.WriteString(" " + paint(r.accent, "│") + " " + body + " " + paint(r.accent, "│") + NL)
	}
	b.WriteString(edge("╰", "╯"))
	return b.String()
}

// notice draws lines with an accent bar; the first line is bold.
func (r *Renderer) notice(lines []string) string {
	var b strings.Builder
	for i, l := range lines {
		code := dim
		if i == 0 {
			code = bold
		}
		for _, w := range wrap(r.expand(l), r.width-4) {
			b.WriteString(" " + paint(r.accent, "▎") + " " + paint(code, w) + NL)
		}
	}
	return b.String()
}

// list draws an optional bold title and bullet or numbered items.
func (r *Renderer) list(l *profile.List) string {
	var b strings.Builder
	if l.Title != "" {
		b.WriteString(" " + r.text(l.Title, bold, r.width-2) + NL)
	}
	for i, item := range l.Items {
		mark := "•"
		if l.Numbered {
			mark = strconv.Itoa(i+1) + "."
		}
		room := r.width - 4 - width(mark)
		b.WriteString(" " + mark + " " + r.text(item, "", room) + NL)
	}
	return b.String()
}

// Tool call styles, indexed by tool kind.
type toolKind int

const (
	toolRead toolKind = iota
	toolEdit
	toolRun
)

var toolStyles = map[profile.ToolStyle]struct {
	marker, format, result string
	names                  [3]string
}{
	profile.ToolsBoxed:  {"●", "%s(%s)", "⎿", [3]string{"Read", "Update", "Run"}},
	profile.ToolsInline: {"→", "%s %s", "└", [3]string{"read", "edit", "run"}},
	profile.ToolsBullet: {"•", "%s %s", "└", [3]string{"Read", "Edited", "Ran"}},
	profile.ToolsCheck:  {"✓", "%s %s", "└", [3]string{"ReadFile", "Edit", "Shell"}},
}

// tool formats a tool call header in the agent's style.
func (r *Renderer) tool(k toolKind, arg string) string {
	st := toolStyles[r.agent.Tools]
	code := r.accent
	if r.agent.Tools == profile.ToolsCheck {
		code = green
	}
	return r.line("  ", st.marker, code, "", fmt.Sprintf(st.format, st.names[k], arg))
}

// result formats the dim line under a tool call.
func (r *Renderer) result(text string) string {
	return r.line("    ", toolStyles[r.agent.Tools].result, dim, dim, text)
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

// LiveArea returns the live area rows without moving the cursor. Use it for
// static output such as previews.
func (r *Renderer) LiveArea(s LiveState) string {
	rows, _, _ := r.live(s)
	return strings.Join(rows, NL)
}

// Live returns the live area and leaves the cursor at the end of the input,
// ready for ClearLive. It must start at the beginning of an empty line.
func (r *Renderer) Live(s LiveState) string {
	rows, row, col := r.live(s)
	r.inputRow = row
	out := strings.Join(rows, NL)
	if up := len(rows) - 1 - row; up > 0 {
		out += fmt.Sprintf("\x1b[%dA", up)
	}
	out += "\r"
	if col > 0 {
		out += fmt.Sprintf("\x1b[%dC", col)
	}
	return out
}

// ClearLive erases the live area drawn by the last Live call, leaving the
// cursor where it started so permanent output can be written there.
func (r *Renderer) ClearLive() string {
	if r.inputRow > 0 {
		return fmt.Sprintf("\x1b[%dA\r\x1b[J", r.inputRow)
	}
	return "\r\x1b[J"
}

// live builds the live area rows and returns them with the input row and
// the cursor column on it.
func (r *Renderer) live(s LiveState) (rows []string, inputRow, col int) {
	full := r.width - 1
	for _, b := range r.agent.Live {
		switch {
		case b.Status != nil:
			if s.Running {
				spin := r.agent.Spinner[s.Frame%len(r.agent.Spinner)]
				text := fmt.Sprintf("%s… (%ds · Esc to stop)", s.Word, int(s.Elapsed/time.Second))
				rows = append(rows, strings.TrimSuffix(r.line("", spin, r.accent, dim, text), NL))
			} else {
				idle := fit(r.expand(b.Status.Idle), full)
				rows = append(rows, strings.Repeat(" ", full-width(idle))+paint(dim, idle))
			}
		case b.Rule != nil:
			rows = append(rows, r.rule())
		case b.Footer != nil:
			rows = append(rows, r.footer(b.Footer))
		default: // input
			var lines []string
			lines, inputRow, col = r.input(b.Input, s.Input)
			inputRow += len(rows)
			rows = append(rows, lines...)
		}
	}
	return rows, inputRow, col
}

// input draws the input in its style and returns the rows, the row and the
// cursor column of the text.
func (r *Renderer) input(in *profile.Input, typed string) ([]string, int, int) {
	inset := 0
	if in.Style == profile.InputBoxed {
		inset = 4 // "│ " and " │"
	}
	prefix := in.Marker + " "
	room := r.width - 1 - inset - width(prefix)
	shown := typed
	if runes := []rune(typed); len(runes) > room {
		shown = "…" + string(runes[len(runes)-room+1:])
	}
	body := paint(r.accent, in.Marker) + " " + shown
	if typed == "" && in.Placeholder != "" {
		body += paint(dim, fit(r.expand(in.Placeholder), room))
	}
	col := width(prefix) + width(shown)

	switch in.Style {
	case profile.InputRuled:
		return []string{r.rule(), body, r.rule()}, 1, col
	case profile.InputBoxed:
		inner := r.width - 3
		used := width(prefix) + width(shown)
		if typed == "" && in.Placeholder != "" {
			used += width(fit(r.expand(in.Placeholder), room))
		}
		line := paint(r.accent, "│") + " " + body + strings.Repeat(" ", max(inner-2-used, 0)) + " " + paint(r.accent, "│")
		top := paint(r.accent, "╭"+strings.Repeat("─", inner)+"╮")
		bottom := paint(r.accent, "╰"+strings.Repeat("─", inner)+"╯")
		return []string{top, line, bottom}, 1, col + 2
	default: // bare
		return []string{body}, 0, col
	}
}

// footer lays out highlight+left, center and right segments on one line,
// dropping segments that do not fit.
func (r *Renderer) footer(f *profile.Footer) string {
	full := r.width - 1
	left, leftW := "  ", 2
	if f.Highlight != "" {
		h := fit(r.expand(f.Highlight), full-leftW)
		left += paint(yellow, h)
		leftW += width(h)
	}
	if f.Left != "" {
		sep := ""
		if f.Highlight != "" {
			sep = " "
		}
		l := fit(r.expand(f.Left), full-leftW-width(sep))
		left += sep + paint(dim, l)
		leftW += width(sep) + width(l)
	}
	out, used := left, leftW
	if f.Center != "" {
		c := r.expand(f.Center)
		if pos := (full - width(c)) / 2; pos > used {
			out += strings.Repeat(" ", pos-used) + paint(dim, c)
			used = pos + width(c)
		}
	}
	if f.Right != "" {
		rt := r.expand(f.Right)
		if pos := full - width(rt); pos > used {
			out += strings.Repeat(" ", pos-used) + paint(dim, rt)
		}
	}
	return out
}

// Preview renders a static sample: the welcome screen, one complete task on
// topic, and the idle live area. It is used by "be preview" and golden tests.
func (r *Renderer) Preview(e *engine.Engine, topic string) string {
	var b strings.Builder
	emit := func(events []engine.Event) bool {
		done := false
		for _, ev := range events {
			b.WriteString(r.Event(ev))
			done = done || ev.Kind == engine.EventTaskDone
		}
		return done
	}
	emit(e.Start())
	emit(e.HandleLine("run " + topic))
	for e.State() == engine.StateRunning && !emit(e.Tick(100*time.Millisecond)) {
	}
	e.HandleKey(engine.KeyEsc)
	b.WriteString(NL + r.LiveArea(LiveState{}) + NL)
	return b.String()
}
