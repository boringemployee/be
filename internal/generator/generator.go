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
	StepEdit                    // Text: file path, Lines: diff lines ("+"/"-" prefixed), Numbers: their line numbers
	StepCommand                 // Text: command, Lines: output, OK: success
	StepDone                    // Text: completion message
)

// Step is one fake action. Delay is how long it "takes" before the next step.
type Step struct {
	Kind    StepKind
	Text    string
	Lines   []string
	Numbers []int // line numbers for Lines, for edits
	OK      bool
	Delay   time.Duration
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
// that topic's packs so the task's files, edits and output belong together.
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
	// One module and test per task, so the task, its commands and their
	// output all talk about the same thing.
	v := vars{file: file, module: pick(g, t.Modules), test: pick(g, t.Tests)}
	steps := []Step{
		{Kind: StepStart, Text: v.fill(pick(g, t.Tasks)), Delay: g.between(300, 700)},
		g.think(),
	}
	for _, f := range g.distinct(t.Files, 1+g.rng.IntN(3), file) {
		steps = append(steps, Step{Kind: StepRead, Text: f, Delay: g.between(400, 1200)})
	}
	steps = append(steps, g.think(), g.edit(t, file))

	cmd := pick(g, t.Commands)
	// Sometimes the command fails first and the change is fixed. Only
	// commands with failure output can fail.
	if g.rng.IntN(4) == 0 && len(cmd.Failure) > 0 {
		steps = append(steps,
			Step{Kind: StepCommand, Text: v.fill(cmd.Run), Lines: v.output(cmd.Failure), Delay: g.between(1500, 4000)},
			g.think(),
			g.edit(t, file),
		)
	}
	steps = append(steps,
		Step{Kind: StepCommand, Text: v.fill(cmd.Run), Lines: v.output(cmd.Output), OK: true, Delay: g.between(1500, 4000)},
		Step{Kind: StepDone, Text: pick(g, g.agent.DoneMessages), Delay: g.between(1000, 2500)},
	)
	return steps
}

func (g *Generator) think() Step {
	return Step{Kind: StepThink, Text: pick(g, g.agent.ThinkingWords), Delay: g.between(1200, 4000)}
}

func (g *Generator) edit(t *profile.Pack, file string) Step {
	// One or two changes at increasing line numbers. Each change shows its
	// old line (if any) directly followed by its new line, numbered alike.
	order := g.rng.Perm(len(t.Edits))
	line := 8 + g.rng.IntN(240)
	var lines []string
	var numbers []int
	for _, i := range order[:min(1+g.rng.IntN(2), len(order))] {
		e := t.Edits[i]
		if e.From != "" {
			lines, numbers = append(lines, "-"+e.From), append(numbers, line)
		}
		lines, numbers = append(lines, "+"+e.To), append(numbers, line)
		line += 4 + g.rng.IntN(40)
	}
	return Step{Kind: StepEdit, Text: file, Lines: lines, Numbers: numbers, Delay: g.between(800, 2000)}
}

// vars are the values of a task's placeholders. {file} is the file the task
// edits, so errors point at the code that was changed.
type vars struct{ file, module, test string }

// output fills the placeholders in a command's output lines, keeping their
// order (the last line is usually a summary).
func (v vars) output(lines []string) []string {
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = v.fill(l)
	}
	return out
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

func (v vars) fill(s string) string {
	return strings.NewReplacer("{file}", v.file, "{module}", v.module, "{test}", v.test).Replace(s)
}
