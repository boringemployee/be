// Command be pretends to be an AI coding agent. See README.md.
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/boringemployee/be/internal/version"
)

const usage = `be - look busy with a fake AI coding agent

Usage:
  be <agent>        start a fake agent session (coming soon)
  be --version      print the version
  be --help         show this help

Inside a session:
  run <topic>       fake work on a topic forever
  Esc               stop the current task
  Ctrl+C            exit
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stdout, usage)
		return 0
	}
	switch args[0] {
	case "-h", "--help", "help":
		fmt.Fprint(stdout, usage)
		return 0
	case "-v", "--version", "version":
		fmt.Fprintln(stdout, version.String())
		return 0
	default:
		fmt.Fprintf(stderr, "be: agent %q is not available yet\n", args[0])
		return 2
	}
}
