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
	recs := Recordings(cat)
	want := len(All(cat))*2 + len(cat.AgentNames())
	if len(recs) != want {
		t.Fatalf("got %d recordings, want %d", len(recs), want)
	}
	files := map[string]bool{}
	for _, r := range recs {
		if files[r.File] {
			t.Errorf("duplicate file %s", r.File)
		}
		files[r.File] = true
		if r.Content == "" {
			t.Errorf("%s is empty", r.File)
		}
	}
	for _, f := range []string{"claude_commands_seed1.events", "claude_mixed_seed42.ansi", "gemini_preview.ansi"} {
		if !files[f] {
			t.Errorf("missing %s", f)
		}
	}
}

func TestPlayDeterministic(t *testing.T) {
	cat := catalog(t)
	sc := All(cat)[1]
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
