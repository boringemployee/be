package profile

import (
	"strings"
	"testing"
	"testing/fstest"
)

const validAgent = `
name: demo
title: Demo
version: v1.2.3
accent: "#112233"
welcome:
  - header: { art: ["##"], lines: ["{title} {version}", "{cwd}"] }
  - banner: { text: "DEMO", gradient: ["#000000", "#ffffff"] }
  - card: ["Card"]
  - notice: ["hello"]
  - list: { title: Tips, items: [one], numbered: true }
  - text: [plain]
  - rule: {}
live:
  - status: { idle: low }
  - input: { style: ruled, marker: ">", placeholder: Type }
  - footer: { highlight: mode, left: hint, center: mid, right: end }
tools: inline
spinner: ["-", "+"]
thinking_words: [Thinking]
done_messages: [Done]
stopped_message: Stopped
`

const validTopic = `
name: work
description: Work
tasks: ["Fix {file} in {module} for {test}"]
files: [a.go, b.go]
modules: [m]
tests: [TestA]
commands: [make test]
logs: [ok]
edits: [x := 1]
`

func fsWith(files map[string]string) fstest.MapFS {
	m := fstest.MapFS{}
	for k, v := range files {
		m[k] = &fstest.MapFile{Data: []byte(v)}
	}
	return m
}

func TestLoad(t *testing.T) {
	c, err := Load(fsWith(map[string]string{
		"agents/demo.yaml": validAgent,
		"topics/work.yaml": validTopic,
		"agents/README.md": "ignored",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if got := c.AgentNames(); len(got) != 1 || got[0] != "demo" {
		t.Errorf("AgentNames() = %v", got)
	}
	if got := c.TopicNames(); len(got) != 1 || got[0] != "work" {
		t.Errorf("TopicNames() = %v", got)
	}
	a := c.Agents["demo"]
	if a.Tools != ToolsInline || len(a.Welcome) != 7 || a.InputBlock().Marker != ">" {
		t.Errorf("agent = %+v", a)
	}
	if (&Agent{}).InputBlock() != nil {
		t.Error("InputBlock of an empty agent must be nil")
	}
}

func TestLoadErrors(t *testing.T) {
	tests := []struct {
		name  string
		files map[string]string
		want  string
	}{
		{"empty", nil, "at least one agent"},
		{"no topics", map[string]string{"agents/demo.yaml": validAgent}, "at least one agent"},
		{"bad agent", map[string]string{"agents/demo.yaml": "name: [", "topics/work.yaml": validTopic}, "agents/demo.yaml"},
		{"bad topic", map[string]string{"agents/demo.yaml": validAgent, "topics/work.yaml": "nope: 1"}, "topics/work.yaml"},
		{"name mismatch", map[string]string{"agents/other.yaml": validAgent, "topics/work.yaml": validTopic}, "must match file name"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Load(fsWith(tt.files))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("err = %v, want containing %q", err, tt.want)
			}
		})
	}
}

func TestLoadFSErrors(t *testing.T) {
	// A directory named like a YAML file cannot be read.
	_, err := Load(fstest.MapFS{"agents/demo.yaml/x": &fstest.MapFile{}})
	if err == nil {
		t.Fatal("want read error")
	}
	if err := loadDir(fstest.MapFS{}, "[", map[string]*Agent{}, ParseAgent); err == nil {
		t.Fatal("want glob pattern error")
	}
}

func TestParseAgentErrors(t *testing.T) {
	tests := []struct {
		name, replace, with, want string
	}{
		{"unknown field", "tools: inline", "tools: inline\ncolour: red", "field colour not found"},
		{"bad name", "name: demo", "name: Demo!", "name:"},
		{"missing title", "title: Demo", "title: ''", "title: required"},
		{"bad version", "version: v1.2.3", "version: latest", "version:"},
		{"bad accent", "#112233", "red", "accent:"},
		{"no spinner", `spinner: ["-", "+"]`, "spinner: []", "spinner: need at least 1"},
		{"bad tools", "tools: inline", "tools: grid", "tools:"},
		{"empty block", "  - rule: {}", "  - {}", "exactly one type, got 0"},
		{"two types", "  - rule: {}", "  - { rule: {}, text: [x] }", "exactly one type, got 2"},
		{"unknown block", "  - rule: {}", "  - grid: {}", "field grid not found"},
		{"live in welcome", "  - rule: {}", "  - footer: {}", "footer is not allowed in welcome"},
		{"welcome in live", "  - footer: {", "  - card: [x]\n  - footer: {", "card is not allowed in live"},
		{"two footers", "  - footer: {", "  - footer: {}\n  - footer: {", "at most one footer"},
		{"no input", "  - input: { style: ruled, marker: \">\", placeholder: Type }", "  - rule: {}", "exactly one status and one input"},
		{"too many live", "  - status:", "  - rule: {}\n  - rule: {}\n  - rule: {}\n  - rule: {}\n  - status:", "live: at most 6"},
		{"bad input style", "style: ruled", "style: fancy", "style:"},
		{"no marker", `marker: ">"`, `marker: ""`, "marker: required"},
		{"tall art", `art: ["##"]`, `art: [a, b, c, d, e, f, g]`, "art: at most 6"},
		{"wide art", `art: ["##"]`, `art: ["12345678901234567"]`, "at most 16 characters wide"},
		{"no header lines", `lines: ["{title} {version}", "{cwd}"]`, `lines: []`, "lines: need at least 1"},
		{"too many lines", `card: ["Card"]`, `card: [a, b, c, d, e, f, g, h, i, j, k]`, "at most 10"},
		{"bad placeholder", `"{cwd}"`, `"{home}"`, "unknown placeholder {home}"},
		{"bad banner text", `text: "DEMO"`, `text: "demo"`, "text:"},
		{"bad gradient", `"#ffffff"]`, `"white"]`, "gradient[1]:"},
		{"long gradient", `gradient: ["#000000", "#ffffff"]`, `gradient: ["#000000", "#000000", "#000000", "#000000", "#000000", "#000000", "#000000"]`, "at most 6 colors"},
		{"unsafe notice", `notice: ["hello"]`, `notice: ["see https://x.example"]`, "contains a URL"},
		{"unsafe footer", "left: hint", "left: 'rm -rf /'", "contains dangerous"},
		{"empty item", "thinking_words: [Thinking]", "thinking_words: ['']", "thinking_words[0]: required"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseAgent([]byte(strings.Replace(validAgent, tt.replace, tt.with, 1)))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("err = %v, want containing %q", err, tt.want)
			}
		})
	}
}

func TestParseTopicErrors(t *testing.T) {
	tests := []struct {
		name, replace, with, want string
	}{
		{"bad placeholder", "{test}", "{user}", "unknown placeholder {user}"},
		{"too many", "modules: [m]", "modules: [" + strings.Repeat("m,", MaxItems) + "m]", "at most"},
		{"dangerous command", "commands: [make test]", "commands: ['rm -rf /tmp/x']", "dangerous command"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseTopic([]byte(strings.Replace(validTopic, tt.replace, tt.with, 1)))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("err = %v, want containing %q", err, tt.want)
			}
		})
	}
	if _, err := ParseTopic([]byte("[")); err == nil {
		t.Fatal("want YAML syntax error")
	}
}

func TestNoWelcome(t *testing.T) {
	a := strings.Index(validAgent, "welcome:")
	b := strings.Index(validAgent, "live:")
	_, err := ParseAgent([]byte(validAgent[:a] + "welcome: []\n" + validAgent[b:]))
	if err == nil || !strings.Contains(err.Error(), "welcome: need 1 to 8") {
		t.Fatalf("err = %v", err)
	}
}

func FuzzParse(f *testing.F) {
	f.Add([]byte(validAgent))
	f.Add([]byte(validTopic))
	f.Fuzz(func(t *testing.T, data []byte) {
		if a, err := ParseAgent(data); err == nil {
			for _, s := range append(append([]string{a.Title, a.Version}, a.ThinkingWords...), a.ThinkingWords...) {
				if CheckText(s) != nil {
					t.Fatalf("accepted unsafe text %q", s)
				}
			}
		}
		if tp, err := ParseTopic(data); err == nil {
			for _, s := range append(append([]string{}, tp.Tasks...), tp.Commands...) {
				if CheckText(s) != nil {
					t.Fatalf("accepted unsafe text %q", s)
				}
			}
		}
	})
}
