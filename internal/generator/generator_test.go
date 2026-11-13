package generator

import (
	"reflect"
	"strings"
	"testing"

	"github.com/boringemployee/be/internal/profile"
)

var (
	agent = &profile.Agent{ThinkingWords: []string{"Thinking"}, DoneMessages: []string{"Done"}}
	topic = &profile.Topic{
		Tasks:    []string{"Fix {file} in {module} for {test}"},
		Files:    []string{"a.go", "b.go", "c.go"},
		Modules:  []string{"mod"},
		Tests:    []string{"TestX"},
		Commands: []string{"make test"},
		Logs:     []string{"checking {file}"},
		Edits:    []string{"x := 1"},
	}
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
				for _, l := range s.Lines {
					if l[0] != '+' && l[0] != '-' {
						t.Fatalf("diff line %q has no +/- prefix", l)
					}
				}
			case StepCommand:
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
