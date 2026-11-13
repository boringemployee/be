// Package golden records full sessions and compares them with files in this
// directory. The CLI and the web build must both reproduce these files.
//
// Update intentionally with: make golden
package golden

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/boringemployee/be/content"
	"github.com/boringemployee/be/internal/engine"
	"github.com/boringemployee/be/internal/profile"
	"github.com/boringemployee/be/internal/render"
)

var update = flag.Bool("update", false, "rewrite golden files")

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

// scenarios covers every agent and every topic without recording every
// combination: agents change how events look, topics change which events
// happen, so each agent runs the first topic and each topic runs the first
// agent.
func scenarios(cat *profile.Catalog) []Scenario {
	run := func(agent, topic string) Scenario {
		return Scenario{
			Agent: agent, Name: topic, Seed: 42, Width: 80,
			Actions: []Action{
				{Line: "run " + topic},
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
		out = append(out, run(agent, topics[0]))
	}
	for _, topic := range topics[1:] {
		out = append(out, run(agents[0], topic))
	}
	mixed := run(agents[0], strings.Join(topics, " "))
	mixed.Name = "mixed"
	return append(out, mixed)
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

func TestGolden(t *testing.T) {
	cat, err := profile.Load(content.FS)
	if err != nil {
		t.Fatal(err)
	}
	for _, sc := range scenarios(cat) {
		base := fmt.Sprintf("%s_%s_seed%d", sc.Agent, sc.Name, sc.Seed)
		t.Run(base, func(t *testing.T) {
			events, ansi, err := Play(cat, sc)
			if err != nil {
				t.Fatal(err)
			}
			check(t, base+".events", events)
			check(t, base+".ansi", ansi)
		})
	}
}

// TestPreviews records "be preview" for every agent, so a new agent YAML is
// covered by a golden file as soon as it is added.
func TestPreviews(t *testing.T) {
	cat, err := profile.Load(content.FS)
	if err != nil {
		t.Fatal(err)
	}
	for _, agent := range cat.AgentNames() {
		t.Run(agent, func(t *testing.T) {
			eng, err := engine.New(cat, agent, 42)
			if err != nil {
				t.Fatal(err)
			}
			r := render.New(eng.Agent(), 80)
			r.SetCwd("~/work/project")
			check(t, agent+"_preview.ansi", r.Preview(eng, cat.TopicNames()[0]))
		})
	}
}

func check(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name)
	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (run make golden to create it)", err)
	}
	if string(want) != got {
		t.Errorf("%s differs from golden file; run make golden and review the diff", name)
	}
}
