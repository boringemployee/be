// Command be pretends to be an AI coding agent. See README.md.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"golang.org/x/term"

	"github.com/boringemployee/be/content"
	"github.com/boringemployee/be/internal/adapter/tty"
	"github.com/boringemployee/be/internal/engine"
	"github.com/boringemployee/be/internal/profile"
	"github.com/boringemployee/be/internal/render"
	"github.com/boringemployee/be/internal/version"
)

const usage = `be - look busy with a fake AI coding agent

Usage:
  be <agent> [--seed N]   start a fake agent session
  be --version            print the version
  be --help               show this help

Agents:
  %s

Inside a session:
  run <topic>             fake work on a topic forever
  Esc                     stop the current task
  Ctrl+C                  exit
`

// options are the parsed command line for a session.
type options struct {
	agent string
	seed  uint64
}

// startSession runs an interactive session; replaced in tests.
var startSession = interactive

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	cat, err := profile.Load(content.FS)
	if err != nil {
		fmt.Fprintln(stderr, "be: broken content:", err)
		return 1
	}
	help := fmt.Sprintf(usage, strings.Join(cat.AgentNames(), ", "))

	fs := flag.NewFlagSet("be", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	seed := fs.Uint64("seed", uint64(time.Now().UnixNano()), "random seed")
	showVersion := fs.Bool("version", false, "print the version")
	var positional []string
	for rest := args; ; {
		if err := fs.Parse(rest); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				fmt.Fprint(stdout, help)
				return 0
			}
			fmt.Fprintf(stderr, "be: %v\n\n%s", err, help)
			return 2
		}
		if fs.NArg() == 0 {
			break
		}
		positional = append(positional, fs.Arg(0)) // allow flags after the agent
		rest = fs.Args()[1:]
	}

	switch {
	case *showVersion || (len(positional) == 1 && positional[0] == "version"):
		fmt.Fprintln(stdout, version.String())
		return 0
	case len(positional) == 0 || positional[0] == "help":
		fmt.Fprint(stdout, help)
		return 0
	case len(positional) > 1:
		fmt.Fprintf(stderr, "be: too many arguments\n\n%s", help)
		return 2
	}

	eng, err := engine.New(cat, positional[0], *seed)
	if err != nil {
		fmt.Fprintf(stderr, "be: %v\n", err)
		return 2
	}
	if err := startSession(eng, options{agent: positional[0], seed: *seed}); err != nil {
		fmt.Fprintf(stderr, "be: %v\n", err)
		return 1
	}
	return 0
}

// displayDir returns the current folder with the home folder shortened to
// "~". It is only shown in the header, never used.
func displayDir() string {
	dir, err := os.Getwd()
	if err != nil {
		return "~"
	}
	if home, err := os.UserHomeDir(); err == nil && strings.HasPrefix(dir, home) {
		dir = "~" + strings.TrimPrefix(dir, home)
	}
	return dir
}

// interactive puts the terminal in raw mode and runs the session.
func interactive(eng *engine.Engine, _ options) error {
	in, out := int(os.Stdin.Fd()), int(os.Stdout.Fd())
	if !term.IsTerminal(in) || !term.IsTerminal(out) {
		return errors.New("needs an interactive terminal")
	}
	old, err := term.MakeRaw(in)
	if err != nil {
		return err
	}
	defer term.Restore(in, old)

	width := func() int {
		if w, _, err := term.GetSize(out); err == nil && w > 0 {
			return w
		}
		return 80
	}
	keys := make(chan []byte)
	go func() {
		defer close(keys)
		for {
			buf := make([]byte, 256)
			n, err := os.Stdin.Read(buf)
			if err != nil {
				return
			}
			keys <- buf[:n]
		}
	}()
	ticker := time.NewTicker(tty.FrameInterval)
	defer ticker.Stop()

	r := render.New(eng.Agent(), width())
	r.SetCwd(displayDir())
	s := &tty.Session{Engine: eng, Renderer: r, Out: os.Stdout, Width: width}
	return s.Run(keys, ticker.C)
}
