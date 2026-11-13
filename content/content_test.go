package content_test

import (
	"testing"

	"github.com/boringemployee/be/content"
	"github.com/boringemployee/be/internal/profile"
)

// Every shipped agent and topic must pass the schema and safety rules.
func TestShippedContentIsValid(t *testing.T) {
	c, err := profile.Load(content.FS)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := c.Agents["claude"]; !ok {
		t.Error("missing claude agent")
	}
	if _, ok := c.Topics["engineering"]; !ok {
		t.Error("missing engineering topic")
	}
}
