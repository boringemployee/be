package guard

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

// workflow is the subset of a GitHub Actions workflow the rules look at.
type workflow struct {
	On          any               `yaml:"on"`
	Permissions map[string]string `yaml:"permissions"`
	Jobs        map[string]struct {
		Environment any `yaml:"environment"`
		Steps       []struct {
			Uses string            `yaml:"uses"`
			Env  map[string]string `yaml:"env"`
			With map[string]any    `yaml:"with"`
		} `yaml:"steps"`
	} `yaml:"jobs"`
}

var pinnedRE = regexp.MustCompile(`^[\w.-]+/[\w./-]+@[0-9a-f]{40}$`)

// TestWorkflowRules enforces the CI rules in AGENTS.md: actions pinned by
// commit SHA, read-only default permissions, no pull_request_target, and
// secrets only in jobs that run behind a protected environment.
func TestWorkflowRules(t *testing.T) {
	files, _ := filepath.Glob(filepath.Join("..", "..", ".github", "workflows", "*.yml"))
	if len(files) == 0 {
		t.Fatal("no workflows found")
	}
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		var w workflow
		if err := yaml.Unmarshal(data, &w); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		name := filepath.Base(f)
		if strings.Contains(string(data), "pull_request_target") {
			t.Errorf("%s: pull_request_target is not allowed", name)
		}
		if len(w.Permissions) != 1 || w.Permissions["contents"] != "read" {
			t.Errorf("%s: default permissions must be exactly {contents: read}, got %v", name, w.Permissions)
		}
		for jobName, job := range w.Jobs {
			for _, step := range job.Steps {
				if step.Uses != "" && !pinnedRE.MatchString(step.Uses) {
					t.Errorf("%s/%s: %q must be pinned to a full commit SHA", name, jobName, step.Uses)
				}
				usesSecret := false
				for _, v := range step.Env {
					usesSecret = usesSecret || strings.Contains(v, "secrets.")
				}
				for _, v := range step.With {
					if s, ok := v.(string); ok && strings.Contains(s, "secrets.") {
						usesSecret = true
					}
				}
				if usesSecret && job.Environment == nil {
					t.Errorf("%s/%s: secrets may only be used in jobs with a protected environment", name, jobName)
				}
			}
		}
	}
}
