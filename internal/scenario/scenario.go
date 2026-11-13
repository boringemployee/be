// Package scenario defines the reproducible sessions behind the golden files.
//
// It is shared by the golden tests (native Go) and the web build (WebAssembly),
// so a browser test can prove both produce byte-identical output.
package scenario

import (
	"fmt"
	"strings"
	"time"

	"github.com/boringemployee/be/internal/engine"
	"github.com/boringemployee/be/internal/profile"
	"github.com/boringemployee/be/internal/render"
)

// Action is one step of a scripted session.
type Action struct {
	Line  string        // submit a line
	Key   engine.Key    // press a key
	Ticks time.Duration // advance fake time in 100ms frames
}

// Scenario is a reproducible session.
type Scenario struct {
	Agent, Name string
	Seed        uint64
	Width       int
	Actions     []Action
}

// Recording is the expected content of one golden file.
type Recording struct {
	File    string
	Content string
}

// PreviewCwd is the folder shown in recorded previews.
const PreviewCwd = "~/work/project"

// All returns every scenario. It covers every agent and every topic without
// recording every combination: agents change how events look and topics change
// which events happen, so each agent runs the first topic, each topic runs the
// first agent, and one run mixes all topics.
func All(cat *profile.Catalog) []Scenario {
	run := func(agent, name, topics string) Scenario {
		return Scenario{
			Agent: agent, Name: name, Seed: 42, Width: 80,
			Actions: []Action{
				{Line: "run " + topics},
				{Ticks: 90 * time.Second},
				{Key: engine.KeyEsc},
				{Key: engine.KeyCtrlC},
			},
		}
	}
	agents, topics := cat.AgentNames(), cat.TopicNames()
	var out []Scenario
	for _, agent := range agents {
		out = append(out, Scenario{
			Agent: agent, Name: "commands", Seed: 1, Width: 80,
			Actions: []Action{{Line: "help"}, {Line: "topics"}, {Line: "run"}, {Line: "run nope"}, {Line: "hello"}, {Line: "exit"}},
		})
		out = append(out, run(agent, topics[0], topics[0]))
	}
	for _, topic := range topics[1:] {
		out = append(out, run(agents[0], topic, topic))
	}
	return append(out, run(agents[0], "mixed", strings.Join(topics, " ")))
}

// Play runs a scenario and returns its event log and rendered output.
func Play(cat *profile.Catalog, sc Scenario) (events, ansi string, err error) {
	eng, err := engine.New(cat, sc.Agent, sc.Seed)
	if err != nil {
		return "", "", err
	}
	r := render.New(eng.Agent(), sc.Width)
	var ev, out strings.Builder
	record := func(list []engine.Event) {
		for _, e := range list {
			fmt.Fprintf(&ev, "%s %q", e.Kind, e.Text)
			for _, l := range e.Lines {
				fmt.Fprintf(&ev, "\n    %q", l)
			}
			if e.Kind == engine.EventCommand {
				fmt.Fprintf(&ev, "\n    ok=%v", e.OK)
			}
			ev.WriteString("\n")
			out.WriteString(r.Event(e))
		}
	}
	record(eng.Start())
	for _, a := range sc.Actions {
		switch {
		case a.Line != "":
			record(eng.HandleLine(a.Line))
		case a.Key != 0:
			record(eng.HandleKey(a.Key))
		default:
			for range a.Ticks / (100 * time.Millisecond) {
				record(eng.Tick(100 * time.Millisecond))
			}
		}
	}
	return ev.String(), out.String(), nil
}

// Recordings returns the content of every golden file: the event log and
// rendered output of each scenario, and a preview of each agent. Agent names
// come from cat itself, so playing them cannot fail.
func Recordings(cat *profile.Catalog) []Recording {
	var out []Recording
	for _, sc := range All(cat) {
		events, ansi, _ := Play(cat, sc)
		base := fmt.Sprintf("%s_%s_seed%d", sc.Agent, sc.Name, sc.Seed)
		out = append(out, Recording{base + ".events", events}, Recording{base + ".ansi", ansi})
	}
	for _, agent := range cat.AgentNames() {
		eng, _ := engine.New(cat, agent, 42)
		r := render.New(eng.Agent(), 80)
		r.SetCwd(PreviewCwd)
		out = append(out, Recording{agent + "_preview.ansi", r.Preview(eng, cat.TopicNames()[0])})
	}
	return out
}
