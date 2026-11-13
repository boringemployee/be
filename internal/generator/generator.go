// Package generator produces deterministic fake work from a seed.
package generator

import (
	"math/rand/v2"
	"strings"
	"time"

	"github.com/boringemployee/be/internal/profile"
)

// StepKind is the kind of fake action in a task.
type StepKind int

// Step kinds, in the order they usually appear in a task.
const (
	StepStart   StepKind = iota // Text: task title
	StepThink                   // Text: thinking word
	StepRead                    // Text: file path
	StepEdit                    // Text: file path, Lines: diff lines ("+"/"-" prefixed)
	StepCommand                 // Text: command, Lines: output, OK: success
	StepDone                    // Text: completion message
)

// Step is one fake action. Delay is how long it "takes" before the next step.
type Step struct {
	Kind  StepKind
	Text  string
	Lines []string
	OK    bool
	Delay time.Duration
}

// Generator creates an endless stream of fake tasks.
type Generator struct {
	rng   *rand.Rand
	agent *profile.Agent
}

// New returns a generator for agent seeded with seed. The same seed always
// produces the same sequence of tasks on every platform.
func New(seed uint64, agent *profile.Agent) *Generator {
	return &Generator{rng: rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15)), agent: agent}
}

func pick[T any](g *Generator, items []T) T { return items[g.rng.IntN(len(items))] }

// between returns a duration in [min, max) milliseconds.
func (g *Generator) between(min, max int) time.Duration {
	return time.Duration(min+g.rng.IntN(max-min)) * time.Millisecond
}

// Next returns the steps of a new fake task on one of topics, using one of
// that topic's packs so the task's files, edits and logs belong together.
// With a single topic no random draw is spent choosing it.
func (g *Generator) Next(topics ...*profile.Topic) []Step {
	t := topics[0]
	if len(topics) > 1 {
		t = pick(g, topics)
	}
	return g.task(pick(g, t.Packs))
}

func (g *Generator) task(t *profile.Pack) []Step {
	file := pick(g, t.Files)
	steps := []Step{
		{Kind: StepStart, Text: g.fill(pick(g, t.Tasks), t, file), Delay: g.between(300, 700)},
		g.think(),
	}
	for _, f := range g.distinct(t.Files, 1+g.rng.IntN(3), file) {
		steps = append(steps, Step{Kind: StepRead, Text: f, Delay: g.between(400, 1200)})
	}
	steps = append(steps, g.think(), g.edit(t, file))

	cmd := pick(g, t.Commands)
	if g.rng.IntN(4) == 0 { // sometimes fail first, then "fix" it
		steps = append(steps,
			Step{Kind: StepCommand, Text: cmd, Lines: g.logs(t, false), Delay: g.between(1500, 4000)},
			g.think(),
			g.edit(t, file),
		)
	}
	steps = append(steps,
		Step{Kind: StepCommand, Text: cmd, Lines: g.logs(t, true), OK: true, Delay: g.between(1500, 4000)},
		Step{Kind: StepDone, Text: pick(g, g.agent.DoneMessages), Delay: g.between(1000, 2500)},
	)
	return steps
}

func (g *Generator) think() Step {
	return Step{Kind: StepThink, Text: pick(g, g.agent.ThinkingWords), Delay: g.between(1200, 4000)}
}

func (g *Generator) edit(t *profile.Pack, file string) Step {
	// Removed and added lines come from one shuffled pool, so a diff never
	// removes and re-adds the same line. With one edit line, it is only added.
	order := g.rng.Perm(len(t.Edits))
	removed := min(1+g.rng.IntN(2), len(order)-1)
	added := min(1+g.rng.IntN(3), len(order)-removed)
	var lines []string
	for _, i := range order[:removed] {
		lines = append(lines, "-"+t.Edits[i])
	}
	for _, i := range order[removed : removed+added] {
		lines = append(lines, "+"+t.Edits[i])
	}
	return Step{Kind: StepEdit, Text: file, Lines: lines, Delay: g.between(800, 2000)}
}

func (g *Generator) logs(t *profile.Pack, ok bool) []string {
	var lines []string
	order := g.rng.Perm(len(t.Logs))
	for _, i := range order[:min(2+g.rng.IntN(3), len(order))] {
		lines = append(lines, g.fill(t.Logs[i], t, pick(g, t.Files)))
	}
	if ok {
		return append(lines, "PASS "+pick(g, t.Tests))
	}
	return append(lines, "FAIL "+pick(g, t.Tests))
}

// distinct returns up to n different items, always starting with first.
func (g *Generator) distinct(items []string, n int, first string) []string {
	out := []string{first}
	for _, i := range g.rng.Perm(len(items)) {
		if len(out) == n {
			break
		}
		if items[i] != first {
			out = append(out, items[i])
		}
	}
	return out
}

func (g *Generator) fill(s string, t *profile.Pack, file string) string {
	return strings.NewReplacer(
		"{file}", file,
		"{module}", pick(g, t.Modules),
		"{test}", pick(g, t.Tests),
	).Replace(s)
}
