package profile

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"
)

// Pack is a coherent set of fake work, such as "go" or "python" within the
// engineering topic. Each task uses a single pack, so files, edits and logs
// never mix languages or tools.
type Pack struct {
	Name     string   `yaml:"-"`
	Tasks    []string `yaml:"tasks"`
	Files    []string `yaml:"files"`
	Modules  []string `yaml:"modules"`
	Tests    []string `yaml:"tests"`
	Commands []string `yaml:"commands"`
	Logs     []string `yaml:"logs"`
	Edits    []Edit   `yaml:"edits"`
}

// Edit is one change shown in a diff: From is replaced by To. Without From it
// is a pure addition.
type Edit struct {
	From string `yaml:"from"`
	To   string `yaml:"to"`
}

// Topic is what "run <topic>" works on. It is a folder topics/<name>/ with a
// topic.yaml (description and optional shared lists) and any number of pack
// files. A pack may leave lists out; they fall back to topic.yaml's lists.
type Topic struct {
	Name        string
	Description string
	// Packs are complete after loading: every list has at least one item.
	Packs []*Pack
}

// topicFile is the schema of topic.yaml.
type topicFile struct {
	Description string `yaml:"description"`
	Pack        `yaml:",inline"`
}

// TopicFile is the name of the file that defines a topic's folder.
const TopicFile = "topic.yaml"

// MaxPacks caps the number of pack files in one topic.
const MaxPacks = 50

// lists returns a pack's string lists in a fixed order with their YAML
// names. Edits are handled separately.
func (p *Pack) lists() []struct {
	name  string
	items *[]string
} {
	return []struct {
		name  string
		items *[]string
	}{
		{"tasks", &p.Tasks}, {"files", &p.Files}, {"modules", &p.Modules}, {"tests", &p.Tests},
		{"commands", &p.Commands}, {"logs", &p.Logs},
	}
}

// loadTopics reads every topics/<name>/ folder.
func loadTopics(fsys fs.FS, into map[string]*Topic) error {
	// fs.Glob only fails on malformed patterns; these are constant and valid.
	flat, _ := fs.Glob(fsys, "topics/*.yaml")
	if len(flat) > 0 {
		return fmt.Errorf("%s: topics must be folders, e.g. topics/<name>/%s", flat[0], TopicFile)
	}
	defs, _ := fs.Glob(fsys, "topics/*/"+TopicFile)
	for _, def := range defs {
		dir := path.Dir(def)
		t, err := loadTopic(fsys, dir)
		if err != nil {
			return err
		}
		into[t.Name] = t
	}
	return nil
}

// loadTopic reads one topic folder.
func loadTopic(fsys fs.FS, dir string) (*Topic, error) {
	name := path.Base(dir)
	if !nameRE.MatchString(name) {
		return nil, fmt.Errorf("%s: folder name must match %s", dir, nameRE)
	}
	var base topicFile
	if err := decodeFile(fsys, path.Join(dir, TopicFile), &base); err != nil {
		return nil, err
	}

	files, _ := fs.Glob(fsys, dir+"/*.yaml")
	sort.Strings(files)
	var packs []*Pack
	if len(base.Tasks) > 0 {
		base.Pack.Name = "topic"
		packs = append(packs, &base.Pack)
	}
	for _, f := range files {
		if path.Base(f) == TopicFile {
			continue
		}
		p := &Pack{Name: strings.TrimSuffix(path.Base(f), ".yaml")}
		if err := decodeFile(fsys, f, p); err != nil {
			return nil, err
		}
		packs = append(packs, p)
	}

	t := &Topic{Name: name, Description: base.Description}
	for _, p := range packs {
		t.Packs = append(t.Packs, resolve(p, &base.Pack))
	}
	if err := t.validate(); err != nil {
		return nil, fmt.Errorf("%s: %w", dir, err)
	}
	return t, nil
}

func decodeFile(fsys fs.FS, file string, v any) error {
	data, err := fs.ReadFile(fsys, file)
	if err != nil {
		return fmt.Errorf("%s: %w", file, err)
	}
	if err := decodeStrict(data, v); err != nil {
		return fmt.Errorf("%s: %w", file, err)
	}
	return nil
}

// resolve returns a copy of p with empty lists filled from base.
func resolve(p, base *Pack) *Pack {
	out := *p
	baseLists := base.lists()
	for i, l := range out.lists() {
		if len(*l.items) == 0 {
			*l.items = *baseLists[i].items
		}
	}
	if len(out.Edits) == 0 {
		out.Edits = base.Edits
	}
	return &out
}

// ParseTopic decodes and validates a single-file topic. It is a convenience
// for tests and tools; shipped topics are folders.
func ParseTopic(name string, data []byte) (*Topic, error) {
	var f topicFile
	if err := decodeStrict(data, &f); err != nil {
		return nil, err
	}
	f.Pack.Name = "topic"
	t := &Topic{Name: name, Description: f.Description, Packs: []*Pack{&f.Pack}}
	return t, t.validate()
}

func (t *Topic) validate() error {
	var c checker
	c.name(t.Name)
	c.str("description", t.Description, true)
	if len(t.Packs) == 0 {
		c.fail("no packs: add tasks to %s or add a pack file", TopicFile)
	}
	if len(t.Packs) > MaxPacks {
		c.fail("at most %d packs", MaxPacks)
	}
	for _, p := range t.Packs {
		for _, l := range p.lists() {
			c.list(p.Name+"."+l.name, *l.items, 1)
		}
		if len(p.Edits) == 0 || len(p.Edits) > MaxItems {
			c.fail("%s.edits: need 1 to %d items", p.Name, MaxItems)
		}
		for i, e := range p.Edits {
			c.str(fmt.Sprintf("%s.edits[%d].from", p.Name, i), e.From, false)
			c.str(fmt.Sprintf("%s.edits[%d].to", p.Name, i), e.To, true)
			if e.From == e.To {
				c.fail("%s.edits[%d]: from and to are the same", p.Name, i)
			}
		}
		for i, task := range p.Tasks {
			c.templates(fmt.Sprintf("%s.tasks[%d]", p.Name, i), task)
		}
		for i, log := range p.Logs {
			c.templates(fmt.Sprintf("%s.logs[%d]", p.Name, i), log)
		}
	}
	return errors.Join(c.errs...)
}

// templates checks the placeholders in a task or log line.
func (c *checker) templates(field, s string) {
	for _, m := range placeholderRE.FindAllStringSubmatch(s, -1) {
		if !placeholders[m[1]] {
			c.fail("%s: unknown placeholder {%s}", field, m[1])
		}
	}
}
