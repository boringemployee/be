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

// run is a session that works on topics for 90 seconds, then stops and exits.
func run(agent, name, topics string) Scenario {
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

// Render returns the scenarios that check how each agent looks and behaves:
// built-in commands and a run of the first topic. Recorded with a small fixed
// topic, they only change when the engine, the renderer or an agent changes.
func Render(cat *profile.Catalog) []Scenario {
	var out []Scenario
	for _, agent := range cat.AgentNames() {
		out = append(out,
			Scenario{
				Agent: agent, Name: "commands", Seed: 1, Width: 80,
				Actions: []Action{{Line: "help"}, {Line: "topics"}, {Line: "run"}, {Line: "run nope"}, {Line: "hello"}, {Line: "exit"}},
			},
			run(agent, cat.TopicNames()[0], cat.TopicNames()[0]),
		)
	}
	return out
}

// Content returns the scenarios that check the shipped content: each topic
// with the first agent, and each agent working across all topics.
func Content(cat *profile.Catalog) []Scenario {
	agents, topics := cat.AgentNames(), cat.TopicNames()
	var out []Scenario
	for _, topic := range topics {
		out = append(out, run(agents[0], topic, topic))
	}
	for _, agent := range agents {
		out = append(out, run(agent, "mixed", strings.Join(topics, " ")))
	}
	return out
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
			for i, l := range e.Lines {
				if i < len(e.Numbers) {
					fmt.Fprintf(&ev, "\n    %d %q", e.Numbers[i], l)
				} else {
					fmt.Fprintf(&ev, "\n    %q", l)
				}
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

// file is the base name of a scenario's golden files.
func file(sc Scenario) string { return fmt.Sprintf("%s_%s_seed%d", sc.Agent, sc.Name, sc.Seed) }

// RenderRecordings returns the render stage golden files: the event log and
// rendered output of each Render scenario, and a preview of each agent. Agent
// names come from cat itself, so playing them cannot fail.
func RenderRecordings(cat *profile.Catalog) []Recording {
	var out []Recording
	for _, sc := range Render(cat) {
		events, ansi, _ := Play(cat, sc)
		out = append(out, Recording{file(sc) + ".events", events}, Recording{file(sc) + ".ansi", ansi})
	}
	for _, agent := range cat.AgentNames() {
		eng, _ := engine.New(cat, agent, 42)
		r := render.New(eng.Agent(), 80)
		r.SetCwd(PreviewCwd)
		out = append(out, Recording{agent + "_preview.ansi", r.Preview(eng, cat.TopicNames()[0])})
	}
	return out
}

// ContentRecordings returns the content stage golden files. Topic runs keep
// only the readable event log, so content diffs are easy to review; mixed
// runs keep the rendered output, so the web build can prove it matches the
// CLI byte for byte for every agent.
func ContentRecordings(cat *profile.Catalog) []Recording {
	var out []Recording
	for _, sc := range Content(cat) {
		events, ansi, _ := Play(cat, sc)
		if sc.Name == "mixed" {
			out = append(out, Recording{file(sc) + ".ansi", ansi})
		} else {
			out = append(out, Recording{file(sc) + ".events", events})
		}
	}
	return out
}
