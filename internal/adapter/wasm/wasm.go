//go:build js && wasm

// Package wasm exposes a session to JavaScript as globalThis.be.
//
// It is deliberately thin: the session logic (key parsing, live area, status
// line) is the same tty.Session the CLI uses, writing ANSI text into a buffer
// that the page passes to xterm.js unchanged.
package wasm

import (
	"bytes"
	"strconv"
	"strings"
	"syscall/js"
	"time"

	"github.com/boringemployee/be/content"
	"github.com/boringemployee/be/internal/adapter/tty"
	"github.com/boringemployee/be/internal/engine"
	"github.com/boringemployee/be/internal/profile"
	"github.com/boringemployee/be/internal/render"
	"github.com/boringemployee/be/internal/scenario"
	"github.com/boringemployee/be/internal/version"
)

type app struct {
	cat     *profile.Catalog
	session *tty.Session
	out     bytes.Buffer
}

// Register installs globalThis.be. It panics if the embedded content is
// invalid, which tests prevent.
func Register() {
	cat, err := profile.Load(content.FS)
	if err != nil {
		panic(err)
	}
	a := &app{cat: cat}
	api := map[string]any{
		"version":    js.FuncOf(func(js.Value, []js.Value) any { return version.Version }),
		"agents":     js.FuncOf(func(js.Value, []js.Value) any { return strings.Join(cat.AgentNames(), ",") }),
		"topics":     js.FuncOf(func(js.Value, []js.Value) any { return strings.Join(cat.TopicNames(), ",") }),
		"start":      js.FuncOf(a.start),
		"input":      js.FuncOf(a.input),
		"frame":      js.FuncOf(a.frame),
		"resize":     js.FuncOf(a.resize),
		"exited":     js.FuncOf(func(js.Value, []js.Value) any { return a.session == nil || a.session.Exited() }),
		"recordings": js.FuncOf(a.recordings),
		"catalog":    js.FuncOf(a.catalog),
	}
	js.Global().Set("be", js.ValueOf(api))
}

// take returns and clears the pending output.
func (a *app) take() string {
	s := a.out.String()
	a.out.Reset()
	return s
}

// start(agent, topics, seed, cols, cwd) starts a session and returns its
// output. topics is a comma-separated list to run right away; seed is a
// decimal string (empty for random). Unknown agents fall back to the first.
func (a *app) start(_ js.Value, args []js.Value) any {
	agent, topics, seedText := args[0].String(), args[1].String(), args[2].String()
	cols, cwd := args[3].Int(), args[4].String()

	if _, ok := a.cat.Agents[agent]; !ok {
		agent = a.cat.AgentNames()[0]
	}
	seed, err := strconv.ParseUint(seedText, 10, 64)
	if err != nil {
		seed = uint64(time.Now().UnixNano())
	}
	eng, _ := engine.New(a.cat, agent, seed) // agent is known to exist
	r := render.New(eng.Agent(), cols)
	r.SetCwd(cwd)
	a.out.Reset()
	a.session = &tty.Session{Engine: eng, Renderer: r, Out: &a.out}
	a.session.Start()

	var run []string
	for _, t := range strings.Split(topics, ",") {
		if _, ok := a.cat.Topics[t]; ok {
			run = append(run, t)
		}
	}
	if len(run) > 0 {
		a.session.Input([]byte("run " + strings.Join(run, " ") + "\r"))
	}
	return a.take()
}

// input(data) handles key bytes from xterm.js and returns the output.
func (a *app) input(_ js.Value, args []js.Value) any {
	if a.session == nil {
		return ""
	}
	a.session.Input([]byte(args[0].String()))
	return a.take()
}

// frame() advances one frame (100ms of fake time) and returns the output.
func (a *app) frame(js.Value, []js.Value) any {
	if a.session == nil || a.session.Exited() {
		return ""
	}
	a.session.Frame()
	return a.take()
}

// resize(cols) changes the layout width for output drawn from now on.
func (a *app) resize(_ js.Value, args []js.Value) any {
	if a.session != nil {
		a.session.Renderer.SetWidth(args[0].Int())
	}
	return nil
}

// recordings() returns the content stage golden recordings as [{file, content}], so a
// browser test can compare the web build with the golden files.
func (a *app) recordings(js.Value, []js.Value) any {
	var out []any
	for _, r := range scenario.ContentRecordings(a.cat) {
		out = append(out, map[string]any{"file": r.File, "content": r.Content})
	}
	return js.ValueOf(out)
}

// catalog() returns the agents and topics for the setup dialog as
// {agents: [{name, title, accent}], topics: [{name, description}]}.
func (a *app) catalog(js.Value, []js.Value) any {
	var agents, topics []any
	for _, name := range a.cat.AgentNames() {
		ag := a.cat.Agents[name]
		agents = append(agents, map[string]any{"name": name, "title": ag.Title, "accent": ag.Accent})
	}
	for _, name := range a.cat.TopicNames() {
		topics = append(topics, map[string]any{"name": name, "description": a.cat.Topics[name].Description})
	}
	return js.ValueOf(map[string]any{"agents": agents, "topics": topics})
}
