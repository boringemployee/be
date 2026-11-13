package generator

import (
	"reflect"
	"strings"
	"testing"

	"github.com/boringemployee/be/internal/profile"
)

var (
	agent = &profile.Agent{ThinkingWords: []string{"Thinking"}, DoneMessages: []string{"Done"}}
	pack  = &profile.Pack{
		Tasks:    []string{"Fix {file} in {module} for {test}"},
		Files:    []string{"a.go", "b.go", "c.go"},
		Modules:  []string{"mod"},
		Tests:    []string{"TestX"},
		Commands: []string{"make test"},
		Logs:     []string{"checking {file}", "ok", "lint: 0 issues", "compiling"},
		Edits:    []profile.Edit{{From: "a", To: "a2"}, {To: "b"}, {From: "c", To: "c2"}, {From: "d", To: "d2"}},
	}
	topic = &profile.Topic{Packs: []*profile.Pack{pack}}
)

func TestDeterministic(t *testing.T) {
	a, b := New(42, agent), New(42, agent)
	for range 20 {
		if x, y := a.Next(topic), b.Next(topic); !reflect.DeepEqual(x, y) {
			t.Fatalf("same seed produced different tasks:\n%v\n%v", x, y)
		}
	}
	if reflect.DeepEqual(New(1, agent).Next(topic), New(2, agent).Next(topic)) {
		t.Error("different seeds produced identical tasks")
	}
}

func TestTaskShape(t *testing.T) {
	g := New(7, agent)
	sawFailure := false
	for range 200 {
		steps := g.Next(topic)
		if steps[0].Kind != StepStart || steps[len(steps)-1].Kind != StepDone {
			t.Fatalf("task must start with StepStart and end with StepDone: %+v", steps)
		}
		if strings.ContainsAny(steps[0].Text, "{}") {
			t.Fatalf("unfilled placeholder in %q", steps[0].Text)
		}
		last := steps[len(steps)-2]
		if last.Kind != StepCommand || !last.OK || !strings.HasPrefix(last.Lines[len(last.Lines)-1], "PASS ") {
			t.Fatalf("task must end with a passing command: %+v", last)
		}
		reads := map[string]bool{}
		for _, s := range steps {
			if s.Delay <= 0 {
				t.Fatalf("step %+v has no delay", s)
			}
			switch s.Kind {
			case StepRead:
				if reads[s.Text] {
					t.Fatalf("file %q read twice", s.Text)
				}
				reads[s.Text] = true
			case StepEdit:
				if len(s.Numbers) != len(s.Lines) {
					t.Fatalf("every diff line needs a line number: %v %v", s.Lines, s.Numbers)
				}
				for i, l := range s.Lines {
					if l[0] != '+' && l[0] != '-' {
						t.Fatalf("diff line %q has no +/- prefix", l)
					}
					// A removed line is always followed by its replacement
					// on the same line number.
					if l[0] == '-' && (i+1 == len(s.Lines) || s.Lines[i+1][0] != '+' || s.Numbers[i+1] != s.Numbers[i]) {
						t.Fatalf("removed line %q is not followed by its replacement: %v", l, s.Lines)
					}
					if i > 0 && s.Numbers[i] < s.Numbers[i-1] {
						t.Fatalf("line numbers must not go down: %v", s.Numbers)
					}
				}
			case StepCommand:
				seenLog := map[string]bool{}
				for _, l := range s.Lines[:len(s.Lines)-1] {
					if seenLog[l] {
						t.Fatalf("command output repeats %q", l)
					}
					seenLog[l] = true
				}
				if !s.OK {
					sawFailure = true
				}
			}
		}
	}
	if !sawFailure {
		t.Error("expected some tasks to fail first and then pass")
	}
}

func TestDistinctWithSingleFile(t *testing.T) {
	g := New(1, agent)
	if got := g.distinct([]string{"only.go"}, 3, "only.go"); !reflect.DeepEqual(got, []string{"only.go"}) {
		t.Fatalf("distinct = %v", got)
	}
}

func TestEditWithSingleLine(t *testing.T) {
	one := *pack
	one.Edits = []profile.Edit{{To: "only"}}
	s := New(1, agent).edit(&one, "a.go")
	if !reflect.DeepEqual(s.Lines, []string{"+only"}) {
		t.Fatalf("lines = %v", s.Lines)
	}
}

func TestTaskUsesOnePack(t *testing.T) {
	goPack := &profile.Pack{Tasks: []string{"go task"}, Files: []string{"a.go"}, Modules: []string{"m"},
		Tests: []string{"TestGo"}, Commands: []string{"go test"}, Logs: []string{"ok go"}, Edits: []profile.Edit{{To: "go1"}, {To: "go2"}}}
	pyPack := &profile.Pack{Tasks: []string{"py task"}, Files: []string{"a.py"}, Modules: []string{"m"},
		Tests: []string{"test_py"}, Commands: []string{"pytest"}, Logs: []string{"ok py"}, Edits: []profile.Edit{{To: "py1"}, {To: "py2"}}}
	two := &profile.Topic{Packs: []*profile.Pack{goPack, pyPack}}
	g := New(3, agent)
	seen := map[string]bool{}
	for range 50 {
		steps := g.Next(two)
		lang := "go"
		if steps[0].Text == "py task" {
			lang = "py"
		}
		seen[lang] = true
		for _, s := range steps {
			for _, l := range append([]string{s.Text}, s.Lines...) {
				if lang == "go" && strings.Contains(l, "py") || lang == "py" && strings.Contains(l, "go") {
					t.Fatalf("%s task mixed in %q", lang, l)
				}
			}
		}
	}
	if !seen["go"] || !seen["py"] {
		t.Errorf("both packs should be used: %v", seen)
	}
}

func TestNextSingleTopicSpendsNoDraw(t *testing.T) {
	// Passing one topic must give the same tasks as before multi-topic runs
	// existed, so seeded sessions stay stable.
	a, b := New(9, agent), New(9, agent)
	if !reflect.DeepEqual(a.Next(topic), b.task(pick(b, topic.Packs))) {
		t.Fatal("single-topic Next must not draw a topic")
	}
	other := &profile.Topic{Packs: []*profile.Pack{pack}}
	if steps := New(9, agent).Next(topic, other); len(steps) == 0 {
		t.Fatal("multi-topic Next returned no steps")
	}
}
