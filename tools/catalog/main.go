// Command catalog prints the agents and topics as JSON for the web page, so
// the New session dialog can open before be.wasm has finished downloading.
// It is a build tool (make web), not part of the CLI.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/boringemployee/be/content"
	"github.com/boringemployee/be/internal/profile"
)

type agent struct {
	Name   string `json:"name"`
	Title  string `json:"title"`
	Accent string `json:"accent"`
}

type topic struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type catalog struct {
	Agents []agent `json:"agents"`
	Topics []topic `json:"topics"`
}

func main() {
	if err := write(os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "catalog:", err)
		os.Exit(1)
	}
}

func write(w io.Writer) error {
	cat, err := profile.Load(content.FS)
	if err != nil {
		return err
	}
	var out catalog
	for _, name := range cat.AgentNames() {
		a := cat.Agents[name]
		out.Agents = append(out.Agents, agent{name, a.Title, a.Accent})
	}
	for _, name := range cat.TopicNames() {
		out.Topics = append(out.Topics, topic{name, cat.Topics[name].Description})
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}
