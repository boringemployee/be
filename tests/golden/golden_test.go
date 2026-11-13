// Package golden records full sessions and compares them with files in this
// directory. The CLI and the web build must both reproduce these files.
//
// Update intentionally with: make golden
package golden

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/boringemployee/be/content"
	"github.com/boringemployee/be/internal/profile"
	"github.com/boringemployee/be/internal/scenario"
)

var update = flag.Bool("update", false, "rewrite golden files")

// TestGolden compares every recording (sessions for each agent and topic,
// and a preview of each agent) with the files in testdata.
func TestGolden(t *testing.T) {
	cat, err := profile.Load(content.FS)
	if err != nil {
		t.Fatal(err)
	}
	for _, rec := range scenario.Recordings(cat) {
		t.Run(rec.File, func(t *testing.T) { check(t, rec.File, rec.Content) })
	}
}

func check(t *testing.T, name, got string) {
	t.Helper()
	if m := profile.Giveaways.FindString(got); m != "" {
		t.Errorf("%s: UI text contains the giveaway word %q", name, m)
	}
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
