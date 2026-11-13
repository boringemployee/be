// Package guard enforces the "fake only" rule: shared packages must not be
// able to run commands, use the network, or touch files.
package guard

import (
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// Shared packages compiled into both the CLI and the web build.
var shared = []string{
	"internal/engine",
	"internal/generator",
	"internal/profile",
	"internal/render",
	"internal/scenario",
	"content",
}

var banned = []string{"os", "os/exec", "os/signal", "net", "syscall", "unsafe", "plugin", "io/ioutil", "runtime/cgo"}

func isBanned(path string) bool {
	for _, b := range banned {
		if path == b {
			return true
		}
	}
	return path == "C" || strings.HasPrefix(path, "net/") || strings.HasPrefix(path, "syscall/") ||
		strings.HasPrefix(path, "golang.org/x/sys")
}

func TestSharedPackagesHaveNoBannedImports(t *testing.T) {
	root := filepath.Join("..", "..")
	checked := 0
	for _, dir := range shared {
		files, _ := filepath.Glob(filepath.Join(root, dir, "*.go"))
		for _, f := range files {
			if strings.HasSuffix(f, "_test.go") {
				continue // tests may use os, e.g. for golden files
			}
			ast, err := parser.ParseFile(token.NewFileSet(), f, nil, parser.ImportsOnly)
			if err != nil {
				t.Fatal(err)
			}
			for _, imp := range ast.Imports {
				p, _ := strconv.Unquote(imp.Path.Value)
				if isBanned(p) {
					t.Errorf("%s imports banned package %q", f, p)
				}
			}
			checked++
		}
	}
	if checked == 0 {
		t.Fatal("no files checked; is the package list stale?")
	}
}

func TestBannedList(t *testing.T) {
	for _, p := range []string{"os", "os/exec", "net/http", "C", "syscall/js", "golang.org/x/sys/unix"} {
		if !isBanned(p) {
			t.Errorf("%q should be banned", p)
		}
	}
	for _, p := range []string{"io/fs", "strings", "embed", "math/rand/v2"} {
		if isBanned(p) {
			t.Errorf("%q should be allowed", p)
		}
	}
}
