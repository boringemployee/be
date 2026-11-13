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
mascot: ["##"]
header: ["model", "{cwd}"]
notice: ["hello"]
prompt: "> "
accent: "#112233"
spinner: ["-", "+"]
thinking_words: [Thinking]
layout: inline
status_right: low
footer_mode: mode
footer_hint: hint
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
	if c.Agents["demo"].Layout != LayoutInline {
		t.Errorf("layout = %q", c.Agents["demo"].Layout)
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
		{"unknown field", "layout: inline", "layout: inline\ncolour: red", "field colour not found"},
		{"bad name", "name: demo", "name: Demo!", "name:"},
		{"missing title", "title: Demo", "title: ''", "title: required"},
		{"bad accent", "#112233", "red", "accent:"},
		{"no spinner", `spinner: ["-", "+"]`, "spinner: []", "spinner: need at least 1"},
		{"bad layout", "layout: inline", "layout: grid", "layout:"},
		{"unsafe notice", `notice: ["hello"]`, `notice: ["see https://x.example"]`, "notice[0]: contains a URL"},
		{"no notice", `notice: ["hello"]`, `notice: []`, "notice: need at least 1"},
		{"tall mascot", `mascot: ["##"]`, `mascot: [a, b, c, d, e, f, g]`, "mascot: at most 6"},
		{"bad version", "version: v1.2.3", "version: latest", "version:"},
		{"bad header placeholder", `"{cwd}"`, `"{home}"`, "header[1]: unknown placeholder {home}"},
		{"unsafe footer", "footer_hint: hint", "footer_hint: 'rm -rf /'", "footer_hint: contains dangerous"},
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

func FuzzParse(f *testing.F) {
	f.Add([]byte(validAgent))
	f.Add([]byte(validTopic))
	f.Fuzz(func(t *testing.T, data []byte) {
		if a, err := ParseAgent(data); err == nil {
			for _, s := range append(append([]string{a.Title, a.FooterHint}, a.Notice...), a.ThinkingWords...) {
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
