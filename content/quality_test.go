package content_test

import (
	"fmt"
	"reflect"
	"regexp"
	"testing"

	"github.com/boringemployee/be/content"
	"github.com/boringemployee/be/internal/generator"
	"github.com/boringemployee/be/internal/profile"
)

// Quality rules for shipped content. They are stricter than the schema in
// internal/profile, which small test fixtures must also pass.

// Minimum list sizes per pack, so sessions vary (see docs/adding-a-topic.md).
const (
	minTasks = 3
	minFiles = 3
	minEdits = 4
)

// products are real AI tools, models and office apps. The UI uses parody
// names instead. Agent names are CLI commands (be claude) and are exempt.
var products = regexp.MustCompile(`(?i)\b(claude|anthropic|openai|chatgpt|gpt-?\d\w*|gemini|grok|kiro|codex|copilot|excel|powerpoint|microsoft|google docs|google sheets|chrome)\b`)

func load(t *testing.T) *profile.Catalog {
	t.Helper()
	c, err := profile.Load(content.FS)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// walkStrings calls fn with every string reachable from v, and its path.
func walkStrings(v reflect.Value, path string, fn func(path, s string)) {
	switch v.Kind() {
	case reflect.Pointer, reflect.Interface:
		if !v.IsNil() {
			walkStrings(v.Elem(), path, fn)
		}
	case reflect.Struct:
		for i := range v.NumField() {
			if v.Type().Field(i).IsExported() {
				walkStrings(v.Field(i), path+"."+v.Type().Field(i).Name, fn)
			}
		}
	case reflect.Slice, reflect.Array:
		for i := range v.Len() {
			walkStrings(v.Index(i), fmt.Sprintf("%s[%d]", path, i), fn)
		}
	case reflect.String:
		fn(path, v.String())
	}
}

func TestNoGiveawaysOrProductNames(t *testing.T) {
	c := load(t)
	check := func(path, s string) {
		if m := profile.Giveaways.FindString(s); m != "" {
			t.Errorf("%s: giveaway word %q in %q", path, m, s)
		}
		if m := products.FindString(s); m != "" {
			t.Errorf("%s: real product name %q in %q", path, m, s)
		}
	}
	for name, a := range c.Agents {
		walkStrings(reflect.ValueOf(a), "agents/"+name, func(path, s string) {
			if path != "agents/"+name+".Name" {
				check(path, s)
			}
		})
	}
	for name, tp := range c.Topics {
		walkStrings(reflect.ValueOf(tp), "topics/"+name, check)
	}
}

func TestPackMinimums(t *testing.T) {
	for name, tp := range load(t).Topics {
		for _, p := range tp.Packs {
			for _, l := range []struct {
				field     string
				got, want int
			}{
				{"tasks", len(p.Tasks), minTasks},
				{"files", len(p.Files), minFiles},
				{"edits", len(p.Edits), minEdits},
			} {
				if l.got < l.want {
					t.Errorf("topics/%s/%s.yaml: %d %s, want at least %d", name, p.Name, l.got, l.field, l.want)
				}
			}
		}
	}
}

// placeholder matches a template left unfilled, such as {module}.
var placeholder = regexp.MustCompile(`\{\w+\}`)

// Every topic, rendered over many seeds, only shows filled, safe lines.
func TestGeneratedTasks(t *testing.T) {
	c := load(t)
	agent := c.Agents[c.AgentNames()[0]]
	for _, name := range c.TopicNames() {
		tp := c.Topics[name]
		for seed := range uint64(300) {
			for _, step := range generator.New(seed, agent).Next(tp) {
				if step.Kind == generator.StepEdit {
					continue // diff lines are code and may contain braces
				}
				for _, s := range append([]string{step.Text}, step.Lines...) {
					if m := placeholder.FindString(s); m != "" {
						t.Fatalf("%s seed %d: unfilled %s in %q", name, seed, m, s)
					}
					if err := profile.CheckText(s); err != nil {
						t.Fatalf("%s seed %d: %q: %v", name, seed, s, err)
					}
				}
			}
		}
	}
}
