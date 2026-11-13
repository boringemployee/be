// Package engine is the fake agent session: input in, events out.
//
// It has no goroutines, clocks or I/O. Frontends call Tick with elapsed time,
// which makes every session deterministic for a given seed and input.
package engine

import (
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/boringemployee/be/internal/generator"
	"github.com/boringemployee/be/internal/profile"
)

// MaxInputLen caps how much of a typed line is kept.
const MaxInputLen = 200

// Engine is a single fake agent session.
type Engine struct {
	catalog *profile.Catalog
	agent   *profile.Agent
	gen     *generator.Generator
	state   State
	topic   *profile.Topic
	queue   []generator.Step
	wait    time.Duration
}

// New starts a session for the named agent.
func New(catalog *profile.Catalog, agent string, seed uint64) (*Engine, error) {
	a, ok := catalog.Agents[agent]
	if !ok {
		return nil, fmt.Errorf("unknown agent %q (available: %s)", agent, strings.Join(catalog.AgentNames(), ", "))
	}
	return &Engine{catalog: catalog, agent: a, gen: generator.New(seed, a)}, nil
}

// State returns the current session state.
func (e *Engine) State() State { return e.state }

// Agent returns the session's agent profile.
func (e *Engine) Agent() *profile.Agent { return e.agent }

// Start returns the welcome screen and first prompt.
func (e *Engine) Start() []Event {
	return []Event{{Kind: EventWelcome, Text: e.agent.Title}, e.prompt()}
}

func (e *Engine) prompt() Event { return Event{Kind: EventPrompt, Text: e.agent.InputBlock().Marker} }

// HandleKey reacts to Esc (stop the task) and Ctrl+C (exit).
func (e *Engine) HandleKey(k Key) []Event {
	if e.state == StateExited {
		return nil
	}
	switch k {
	case KeyCtrlC:
		return e.exit()
	case KeyEsc:
		if e.state == StateRunning {
			e.state, e.topic, e.queue, e.wait = StateIdle, nil, nil, 0
			return []Event{{Kind: EventStopped, Text: e.agent.StoppedMessage}, e.prompt()}
		}
	}
	return nil
}

func (e *Engine) exit() []Event {
	e.state, e.queue = StateExited, nil
	return []Event{{Kind: EventExit}}
}

// HandleLine reacts to a line the user typed and submitted.
func (e *Engine) HandleLine(line string) []Event {
	if e.state == StateExited {
		return nil
	}
	line = Sanitize(line)
	if e.state == StateRunning {
		return []Event{{Kind: EventNotice, Lines: []string{"Busy. Press Esc to stop the current task."}}}
	}
	if line == "" {
		return []Event{e.prompt()}
	}
	out := []Event{{Kind: EventEcho, Text: line}}
	fields := strings.Fields(line)
	switch strings.ToLower(fields[0]) {
	case "run":
		return append(out, e.run(fields[1:])...)
	case "help", "/help", "?":
		out = append(out, Event{Kind: EventNotice, Lines: e.help()})
	case "topics", "/topics":
		out = append(out, Event{Kind: EventNotice, Lines: e.topics()})
	case "exit", "quit", "/exit", "/quit":
		return append(out, e.exit()...)
	default:
		out = append(out, Event{Kind: EventNotice, Lines: []string{
			"I'd love to help with that, but I only pretend to work.",
			"Try: run <topic>   (type 'topics' to see them)",
		}})
	}
	return append(out, e.prompt())
}

func (e *Engine) run(args []string) []Event {
	if len(args) != 1 {
		return []Event{{Kind: EventError, Text: "usage: run <topic>"}, {Kind: EventNotice, Lines: e.topics()}, e.prompt()}
	}
	t, ok := e.catalog.Topics[strings.ToLower(args[0])]
	if !ok {
		return []Event{{Kind: EventError, Text: fmt.Sprintf("unknown topic %q", args[0])}, {Kind: EventNotice, Lines: e.topics()}, e.prompt()}
	}
	e.state, e.topic, e.queue = StateRunning, t, nil
	return []Event{e.advance()}
}

// Tick advances fake time by dt and returns any events that became due.
func (e *Engine) Tick(dt time.Duration) []Event {
	if e.state != StateRunning || dt <= 0 {
		return nil
	}
	var out []Event
	e.wait -= dt
	for e.wait <= 0 { // every step has a positive delay, so this ends
		out = append(out, e.advance())
	}
	return out
}

// advance emits the next step, refilling the queue with a new task forever.
func (e *Engine) advance() Event {
	if len(e.queue) == 0 {
		e.queue = e.gen.Next(e.topic)
	}
	s := e.queue[0]
	e.queue = e.queue[1:]
	e.wait += s.Delay
	return stepEvent(s)
}

var stepKinds = map[generator.StepKind]Kind{
	generator.StepStart:   EventTaskStart,
	generator.StepThink:   EventThinking,
	generator.StepRead:    EventRead,
	generator.StepEdit:    EventEdit,
	generator.StepCommand: EventCommand,
	generator.StepDone:    EventTaskDone,
}

func stepEvent(s generator.Step) Event {
	return Event{Kind: stepKinds[s.Kind], Text: s.Text, Lines: s.Lines, OK: s.OK}
}

func (e *Engine) help() []string {
	return []string{
		"Commands:",
		"  run <topic>   pretend to work on a topic, forever",
		"  topics        list topics",
		"  help          show this help",
		"  exit          leave the session",
		"Keys:",
		"  Esc           stop the current task",
		"  Ctrl+C        exit",
	}
}

func (e *Engine) topics() []string {
	lines := []string{"Topics:"}
	for _, name := range e.catalog.TopicNames() {
		lines = append(lines, fmt.Sprintf("  %-14s %s", name, e.catalog.Topics[name].Description))
	}
	return lines
}

// Sanitize removes control and invisible characters from user input, trims
// it, and caps its length, so typed text can never inject escape sequences.
func Sanitize(s string) string {
	var b strings.Builder
	n := 0
	for _, r := range strings.ToValidUTF8(s, "") {
		if n == MaxInputLen {
			break
		}
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) || r == ' ' || r == ' ' {
			continue
		}
		b.WriteRune(r)
		n++
	}
	return strings.TrimSpace(b.String())
}
