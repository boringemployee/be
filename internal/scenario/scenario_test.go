package scenario

import (
	"strings"
	"testing"

	"github.com/boringemployee/be/content"
	"github.com/boringemployee/be/internal/profile"
)

func catalog(t *testing.T) *profile.Catalog {
	t.Helper()
	cat, err := profile.Load(content.FS)
	if err != nil {
		t.Fatal(err)
	}
	return cat
}

func TestRecordings(t *testing.T) {
	cat := catalog(t)
	agents, topics := len(cat.AgentNames()), len(cat.TopicNames())
	for _, tc := range []struct {
		name string
		recs []Recording
		want int
		has  []string
	}{
		{"render", RenderRecordings(cat), agents * 5, []string{"claude_commands_seed1.events", "claude_data_seed42.ansi", "gemini_preview.ansi"}},
		{"content", ContentRecordings(cat), topics + agents, []string{"claude_qa_seed42.events", "codex_mixed_seed42.ansi"}},
	} {
		if len(tc.recs) != tc.want {
			t.Errorf("%s: got %d recordings, want %d", tc.name, len(tc.recs), tc.want)
		}
		files := map[string]bool{}
		for _, r := range tc.recs {
			if files[r.File] {
				t.Errorf("%s: duplicate file %s", tc.name, r.File)
			}
			files[r.File] = true
			if r.Content == "" {
				t.Errorf("%s: %s is empty", tc.name, r.File)
			}
		}
		for _, f := range tc.has {
			if !files[f] {
				t.Errorf("%s: missing %s", tc.name, f)
			}
		}
	}
}

func TestPlayDeterministic(t *testing.T) {
	cat := catalog(t)
	sc := Render(cat)[1]
	e1, a1, _ := Play(cat, sc)
	e2, a2, _ := Play(cat, sc)
	if e1 != e2 || a1 != a2 {
		t.Fatal("same scenario produced different output")
	}
	if !strings.Contains(e1, "command ") || !strings.Contains(e1, "ok=") {
		t.Errorf("event log should include commands with results:\n%s", e1[:200])
	}
}

func TestUnknownAgent(t *testing.T) {
	if _, _, err := Play(catalog(t), Scenario{Agent: "nope"}); err == nil {
		t.Error("Play with an unknown agent must fail")
	}
}
