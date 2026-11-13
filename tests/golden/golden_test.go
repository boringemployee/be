// Package golden records full sessions and compares them with files in
// testdata. There are two stages:
//
//   - render: every agent with the fixed topic in fixture/. These files only
//     change when the engine, the renderer or an agent changes.
//   - content: the shipped topics. These files change with topic content.
//
// The web build must reproduce the content files byte for byte (make e2e).
//
// Update intentionally with: make golden (or golden-render, golden-content)
package golden

import (
	"flag"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/boringemployee/be/content"
	"github.com/boringemployee/be/internal/profile"
	"github.com/boringemployee/be/internal/scenario"
)

var update = flag.Bool("update", false, "rewrite golden files")

// TestGoldenRender records every agent working on the fixture topic.
func TestGoldenRender(t *testing.T) {
	compare(t, "render", scenario.RenderRecordings(fixtureCatalog(t)))
}

// TestGoldenContent records the shipped topics.
func TestGoldenContent(t *testing.T) {
	cat, err := profile.Load(content.FS)
	if err != nil {
		t.Fatal(err)
	}
	compare(t, "content", scenario.ContentRecordings(cat))
}

// fixtureCatalog loads the shipped agents with the topics in fixture/.
func fixtureCatalog(t *testing.T) *profile.Catalog {
	t.Helper()
	fsys := fstest.MapFS{}
	add := func(src fs.FS, root string) {
		err := fs.WalkDir(src, root, func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			data, err := fs.ReadFile(src, p)
			fsys[p] = &fstest.MapFile{Data: data}
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	add(content.FS, "agents")
	add(os.DirFS("fixture"), "topics")
	cat, err := profile.Load(fsys)
	if err != nil {
		t.Fatal(err)
	}
	return cat
}

// compare checks recordings against testdata/<stage>, and that the folder
// has no stale files.
func compare(t *testing.T, stage string, recs []scenario.Recording) {
	dir := filepath.Join("testdata", stage)
	if *update {
		if err := os.RemoveAll(dir); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	want := map[string]bool{}
	for _, rec := range recs {
		want[rec.File] = true
		t.Run(rec.File, func(t *testing.T) { check(t, filepath.Join(dir, rec.File), rec.Content) })
	}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if !want[e.Name()] {
			t.Errorf("%s: no recording produces this file; run make golden-%s", path.Join(dir, e.Name()), stage)
		}
	}
}

func check(t *testing.T, file, got string) {
	t.Helper()
	if m := profile.Giveaways.FindString(got); m != "" {
		t.Errorf("%s: UI text contains the giveaway word %q", file, m)
	}
	if *update {
		if err := os.WriteFile(file, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("%v (run make golden to create it)", err)
	}
	if string(want) != got {
		t.Errorf("%s differs from golden file; run make golden and review the diff", file)
	}
}
