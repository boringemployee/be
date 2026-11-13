package profile

import (
	"reflect"
	"strings"
	"testing"
)

const goPack = `
tasks: ["Fix {file}"]
files: [a.go]
edits: [{to: "x := 1"}]
`

func TestLoadTopicFolder(t *testing.T) {
	c, err := Load(fsWith(map[string]string{
		"agents/demo.yaml": validAgent,
		// topic.yaml: description and shared lists, but no tasks, so it is
		// only a fallback, not a pack of its own.
		"topics/work/topic.yaml": "description: Work\nmodules: [shared]\ntests: [TestShared]\ncommands: [{run: make, output: [ok]}]\n",
		"topics/work/go.yaml":    goPack,
		"topics/work/python.yaml": `
tasks: ["Fix {file}"]
files: [a.py]
modules: [own]
tests: [test_own]
commands: [{run: pytest, output: [passed]}]
edits: [{from: "x = 0", to: "x = 1"}]
`,
		"topics/work/notes.md": "ignored",
	}))
	if err != nil {
		t.Fatal(err)
	}
	topic := c.Topics["work"]
	if topic.Description != "Work" || len(topic.Packs) != 2 {
		t.Fatalf("topic = %+v", topic)
	}
	goP, py := topic.Packs[0], topic.Packs[1]
	if goP.Name != "go" || py.Name != "python" {
		t.Errorf("packs must be sorted by file name: %q, %q", goP.Name, py.Name)
	}
	want := &Pack{Name: "go", Tasks: []string{"Fix {file}"}, Files: []string{"a.go"}, Modules: []string{"shared"},
		Tests: []string{"TestShared"}, Commands: []Command{{Run: "make", Output: []string{"ok"}}}, Edits: []Edit{{To: "x := 1"}}}
	if !reflect.DeepEqual(goP, want) {
		t.Errorf("go pack should fall back to shared lists:\n got %+v\nwant %+v", goP, want)
	}
	if py.Modules[0] != "own" || py.Commands[0].Run != "pytest" {
		t.Errorf("python pack must keep its own lists: %+v", py)
	}
}

func TestTopicFileAsPack(t *testing.T) {
	c, err := Load(fsWith(map[string]string{
		"agents/demo.yaml":       validAgent,
		"topics/work/topic.yaml": validTopic,
		"topics/work/extra.yaml": "tasks: [Another task]\n",
	}))
	if err != nil {
		t.Fatal(err)
	}
	packs := c.Topics["work"].Packs
	if len(packs) != 2 || packs[0].Name != "topic" || packs[1].Name != "extra" || packs[1].Files[0] != "a.go" {
		t.Fatalf("packs = %+v %+v", packs[0], packs[1])
	}
}

func TestLoadTopicErrors(t *testing.T) {
	tests := []struct {
		name  string
		files map[string]string
		want  string
	}{
		{"flat file", map[string]string{"topics/work.yaml": validTopic}, "topics must be folders"},
		{"bad folder name", map[string]string{"topics/Work/topic.yaml": validTopic}, "folder name must match"},
		{"no packs", map[string]string{"topics/work/topic.yaml": "description: Work\n"}, "no packs"},
		{"missing list", map[string]string{"topics/work/topic.yaml": "description: Work\n", "topics/work/go.yaml": goPack}, "go.modules: need at least 1"},
		{"unknown field in pack", map[string]string{"topics/work/topic.yaml": validTopic, "topics/work/go.yaml": "name: go\n"}, "topics/work/go.yaml"},
		{"description in pack", map[string]string{"topics/work/topic.yaml": validTopic, "topics/work/go.yaml": "description: x\n"}, "field description not found"},
		{"unreadable pack", map[string]string{"topics/work/topic.yaml": validTopic, "topics/work/go.yaml/x": ""}, "go.yaml"},
		{"bad output placeholder", map[string]string{"topics/work/topic.yaml": strings.Replace(validTopic, `output: ["ok {module}"]`, `output: ["{home}"]`, 1)}, "topic.commands[0].output[0]: unknown placeholder"},
		{"no commands", map[string]string{"topics/work/topic.yaml": strings.Replace(validTopic, `commands: [{run: make test, output: ["ok {module}"], failure: ["FAIL {test}"]}]`, "", 1)}, "topic.commands: need 1 to"},
		{"command without run", map[string]string{"topics/work/topic.yaml": strings.Replace(validTopic, "run: make test", `run: ""`, 1)}, "commands[0].run: required"},
		{"too much output", map[string]string{"topics/work/topic.yaml": strings.Replace(validTopic, `failure: ["FAIL {test}"]`, "failure: [a, b, c, d, e, f, g, h, i]", 1)}, "failure: at most 8"},
		{"too many packs", tooManyPacks(), "at most 50 packs"},
		{"no edits", map[string]string{"topics/work/topic.yaml": strings.Replace(validTopic, `edits: [{from: "x = 0", to: "x := 1"}]`, "", 1)}, "topic.edits: need 1 to"},
		{"edit without to", map[string]string{"topics/work/topic.yaml": strings.Replace(validTopic, `to: "x := 1"`, `to: ""`, 1)}, "edits[0].to: required"},
		{"edit unchanged", map[string]string{"topics/work/topic.yaml": strings.Replace(validTopic, `from: "x = 0"`, `from: "x := 1"`, 1)}, "from and to are the same"},
		{"unsafe edit", map[string]string{"topics/work/topic.yaml": strings.Replace(validTopic, `from: "x = 0"`, `from: "curl evil"`, 1)}, "edits[0].from: contains dangerous"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files := map[string]string{"agents/demo.yaml": validAgent}
			for k, v := range tt.files {
				files[k] = v
			}
			_, err := Load(fsWith(files))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("err = %v, want containing %q", err, tt.want)
			}
		})
	}
}

func tooManyPacks() map[string]string {
	files := map[string]string{"topics/work/topic.yaml": validTopic}
	for i := range MaxPacks {
		files["topics/work/p"+strings.Repeat("x", i)+".yaml"] = "tasks: [t]\n"
	}
	return files
}

func TestParseTopicErrors(t *testing.T) {
	tests := []struct {
		name, replace, with, want string
	}{
		{"bad placeholder", "{test}", "{user}", "unknown placeholder {user}"},
		{"too many", "modules: [m]", "modules: [" + strings.Repeat("m,", MaxItems) + "m]", "at most"},
		{"dangerous command", "run: make test", "run: 'rm -rf /tmp/x'", "dangerous command"},
		{"no description", "description: Work", "description: ''", "description: required"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseTopic("work", []byte(strings.Replace(validTopic, tt.replace, tt.with, 1)))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("err = %v, want containing %q", err, tt.want)
			}
		})
	}
	if _, err := ParseTopic("work", []byte("[")); err == nil {
		t.Fatal("want YAML syntax error")
	}
	if _, err := ParseTopic("Bad Name", []byte(validTopic)); err == nil {
		t.Fatal("want name error")
	}
}
