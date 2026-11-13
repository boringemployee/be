// Package profile loads and validates agent and topic definitions.
//
// Content is untrusted data: every string is checked by the safety rules in
// safety.go before it can reach the engine or the screen.
package profile

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Layout names the ways an agent can present tool calls.
type Layout string

// Supported layouts.
const (
	LayoutBoxed  Layout = "boxed"
	LayoutInline Layout = "inline"
)

// Agent describes how a fake agent looks and talks.
type Agent struct {
	Name    string `yaml:"name"`
	Title   string `yaml:"title"`
	Version string `yaml:"version"`
	// Mascot is a small original pixel-art icon shown left of the header.
	Mascot []string `yaml:"mascot"`
	// Header lines appear under "Title Version"; {cwd} is the current folder.
	Header []string `yaml:"header"`
	// Notice is a highlighted block under the header; the first line is bold.
	Notice         []string `yaml:"notice"`
	Prompt         string   `yaml:"prompt"`
	Accent         string   `yaml:"accent"`
	Spinner        []string `yaml:"spinner"`
	ThinkingWords  []string `yaml:"thinking_words"`
	Layout         Layout   `yaml:"layout"`
	StatusRight    string   `yaml:"status_right"`
	FooterMode     string   `yaml:"footer_mode"`
	FooterHint     string   `yaml:"footer_hint"`
	DoneMessages   []string `yaml:"done_messages"`
	StoppedMessage string   `yaml:"stopped_message"`
}

// Topic describes the fake work done by "run <topic>".
type Topic struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Tasks       []string `yaml:"tasks"`
	Files       []string `yaml:"files"`
	Modules     []string `yaml:"modules"`
	Tests       []string `yaml:"tests"`
	Commands    []string `yaml:"commands"`
	Logs        []string `yaml:"logs"`
	Edits       []string `yaml:"edits"`
}

// Catalog holds every loaded agent and topic, keyed by name.
type Catalog struct {
	Agents map[string]*Agent
	Topics map[string]*Topic
}

// AgentNames returns the agent names in sorted order.
func (c *Catalog) AgentNames() []string { return sortedKeys(c.Agents) }

// TopicNames returns the topic names in sorted order.
func (c *Catalog) TopicNames() []string { return sortedKeys(c.Topics) }

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

var (
	nameRE    = regexp.MustCompile(`^[a-z][a-z0-9-]{0,31}$`)
	colorRE   = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
	versionRE = regexp.MustCompile(`^v[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}$`)
	// Placeholders a task template may use.
	placeholderRE = regexp.MustCompile(`\{([a-z]+)\}`)
	placeholders  = map[string]bool{"file": true, "module": true, "test": true}
)

// Load reads agents/*.yaml and topics/*.yaml from fsys and validates them.
func Load(fsys fs.FS) (*Catalog, error) {
	c := &Catalog{Agents: map[string]*Agent{}, Topics: map[string]*Topic{}}
	if err := loadDir(fsys, "agents", c.Agents, ParseAgent); err != nil {
		return nil, err
	}
	if err := loadDir(fsys, "topics", c.Topics, ParseTopic); err != nil {
		return nil, err
	}
	if len(c.Agents) == 0 || len(c.Topics) == 0 {
		return nil, errors.New("profile: need at least one agent and one topic")
	}
	return c, nil
}

type named interface{ name() string }

func (a *Agent) name() string { return a.Name }
func (t *Topic) name() string { return t.Name }

func loadDir[T named](fsys fs.FS, dir string, into map[string]T, parse func([]byte) (T, error)) error {
	files, err := fs.Glob(fsys, dir+"/*.yaml")
	if err != nil {
		return err
	}
	for _, file := range files {
		data, err := fs.ReadFile(fsys, file)
		if err != nil {
			return fmt.Errorf("%s: %w", file, err)
		}
		v, err := parse(data)
		if err != nil {
			return fmt.Errorf("%s: %w", file, err)
		}
		if want := strings.TrimSuffix(path.Base(file), ".yaml"); v.name() != want {
			return fmt.Errorf("%s: name %q must match file name %q", file, v.name(), want)
		}
		into[v.name()] = v
	}
	return nil
}

func decodeStrict(data []byte, v any) error {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	return dec.Decode(v)
}

// ParseAgent decodes and validates a single agent definition.
func ParseAgent(data []byte) (*Agent, error) {
	var a Agent
	if err := decodeStrict(data, &a); err != nil {
		return nil, err
	}
	return &a, a.validate()
}

// ParseTopic decodes and validates a single topic definition.
func ParseTopic(data []byte) (*Topic, error) {
	var t Topic
	if err := decodeStrict(data, &t); err != nil {
		return nil, err
	}
	return &t, t.validate()
}

type checker struct{ errs []error }

func (c *checker) fail(format string, args ...any) {
	c.errs = append(c.errs, fmt.Errorf(format, args...))
}

func (c *checker) str(field, s string, required bool) {
	if s == "" {
		if required {
			c.fail("%s: required", field)
		}
		return
	}
	if err := CheckText(s); err != nil {
		c.fail("%s: %w", field, err)
	}
}

func (c *checker) list(field string, items []string, min int) {
	c.listMax(field, items, min, MaxItems)
}

func (c *checker) listMax(field string, items []string, min, max int) {
	if len(items) < min {
		c.fail("%s: need at least %d item(s)", field, min)
	}
	if len(items) > max {
		c.fail("%s: at most %d items", field, max)
	}
	for i, s := range items {
		c.str(fmt.Sprintf("%s[%d]", field, i), s, true)
	}
}

func (c *checker) name(s string) {
	if !nameRE.MatchString(s) {
		c.fail("name: %q must match %s", s, nameRE)
	}
}

func (a *Agent) validate() error {
	var c checker
	c.name(a.Name)
	c.str("title", a.Title, true)
	if !versionRE.MatchString(a.Version) {
		c.fail("version: %q must look like v1.2.3", a.Version)
	}
	c.listMax("mascot", a.Mascot, 0, 6)
	c.listMax("header", a.Header, 0, 4)
	for i, h := range a.Header {
		for _, m := range placeholderRE.FindAllStringSubmatch(h, -1) {
			if m[1] != "cwd" {
				c.fail("header[%d]: unknown placeholder {%s}", i, m[1])
			}
		}
	}
	c.listMax("notice", a.Notice, 1, 6)
	c.str("prompt", a.Prompt, true)
	if !colorRE.MatchString(a.Accent) {
		c.fail("accent: %q must be a #rrggbb color", a.Accent)
	}
	c.list("spinner", a.Spinner, 1)
	c.list("thinking_words", a.ThinkingWords, 1)
	if a.Layout != LayoutBoxed && a.Layout != LayoutInline {
		c.fail("layout: %q must be %q or %q", a.Layout, LayoutBoxed, LayoutInline)
	}
	c.str("status_right", a.StatusRight, false)
	c.str("footer_mode", a.FooterMode, false)
	c.str("footer_hint", a.FooterHint, false)
	c.list("done_messages", a.DoneMessages, 1)
	c.str("stopped_message", a.StoppedMessage, true)
	return errors.Join(c.errs...)
}

func (t *Topic) validate() error {
	var c checker
	c.name(t.Name)
	c.str("description", t.Description, true)
	c.list("tasks", t.Tasks, 1)
	c.list("files", t.Files, 1)
	c.list("modules", t.Modules, 1)
	c.list("tests", t.Tests, 1)
	c.list("commands", t.Commands, 1)
	c.list("logs", t.Logs, 1)
	c.list("edits", t.Edits, 1)
	for i, task := range t.Tasks {
		for _, m := range placeholderRE.FindAllStringSubmatch(task, -1) {
			if !placeholders[m[1]] {
				c.fail("tasks[%d]: unknown placeholder {%s}", i, m[1])
			}
		}
	}
	return errors.Join(c.errs...)
}
