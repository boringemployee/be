package profile

import (
	"fmt"
	"regexp"
)

// ToolStyle names how tool calls are shown.
type ToolStyle string

// Supported tool styles.
const (
	ToolsBoxed  ToolStyle = "boxed"  // ● Read(file)
	ToolsInline ToolStyle = "inline" // → read file
	ToolsBullet ToolStyle = "bullet" // • Read file
	ToolsCheck  ToolStyle = "check"  // ✓ ReadFile file
)

// InputStyle names how the input line is framed.
type InputStyle string

// Supported input styles.
const (
	InputBare  InputStyle = "bare"  // marker and text only
	InputRuled InputStyle = "ruled" // a rule above and below
	InputBoxed InputStyle = "boxed" // a rounded box around it
)

// Block is one piece of a layout. Exactly one field must be set.
type Block struct {
	Header *Header   `yaml:"header"`
	Banner *Banner   `yaml:"banner"`
	Card   *[]string `yaml:"card"`
	Notice *[]string `yaml:"notice"`
	List   *List     `yaml:"list"`
	Text   *[]string `yaml:"text"`
	Rule   *Rule     `yaml:"rule"`
	Status *Status   `yaml:"status"`
	Input  *Input    `yaml:"input"`
	Footer *Footer   `yaml:"footer"`
}

// Header shows small art on the left and text lines on the right. The first
// line is bold; the others are dim.
type Header struct {
	Art   []string `yaml:"art"`
	Lines []string `yaml:"lines"`
}

// Banner shows Text in a built-in block-letter font, colored left to right
// across Gradient (or the accent color when empty).
type Banner struct {
	Text     string   `yaml:"text"`
	Gradient []string `yaml:"gradient"`
}

// List shows an optional bold title followed by bullet or numbered items.
type List struct {
	Title    string   `yaml:"title"`
	Items    []string `yaml:"items"`
	Numbered bool     `yaml:"numbered"`
}

// Rule is a horizontal line across the terminal.
type Rule struct{}

// Status is the line above the input: a spinner while working, otherwise
// Idle aligned to the right.
type Status struct {
	Idle string `yaml:"idle"`
}

// Input is the prompt line where the user types.
type Input struct {
	Style       InputStyle `yaml:"style"`
	Marker      string     `yaml:"marker"`
	Placeholder string     `yaml:"placeholder"`
}

// Footer is a line of hints. Highlight is shown first in a bright color.
type Footer struct {
	Highlight string `yaml:"highlight"`
	Left      string `yaml:"left"`
	Center    string `yaml:"center"`
	Right     string `yaml:"right"`
}

var (
	// BannerRE lists the characters the built-in banner font can draw.
	BannerRE = regexp.MustCompile(`^[A-Z0-9 .!-]{1,12}$`)
	// Placeholders allowed in layout text.
	layoutPlaceholders = map[string]bool{"title": true, "version": true, "cwd": true}
)

const maxArtWidth = 16

// layoutText checks a layout string, including its placeholders.
func (c *checker) layoutText(field, s string, required bool) {
	c.str(field, s, required)
	for _, m := range placeholderRE.FindAllStringSubmatch(s, -1) {
		if !layoutPlaceholders[m[1]] {
			c.fail("%s: unknown placeholder {%s}", field, m[1])
		}
	}
}

func (c *checker) layoutLines(field string, lines []string, min, max int) {
	if len(lines) < min {
		c.fail("%s: need at least %d item(s)", field, min)
	}
	if len(lines) > max {
		c.fail("%s: at most %d items", field, max)
	}
	for i, s := range lines {
		c.layoutText(fmt.Sprintf("%s[%d]", field, i), s, true)
	}
}

// block validates one block and returns its type name ("" if invalid).
func (c *checker) block(field string, b Block) string {
	kinds := []string{}
	if b.Header != nil {
		kinds = append(kinds, "header")
		c.listMax(field+".art", b.Header.Art, 0, 6)
		for i, l := range b.Header.Art {
			if n := len([]rune(l)); n > maxArtWidth {
				c.fail("%s.art[%d]: at most %d characters wide", field, i, maxArtWidth)
			}
		}
		c.layoutLines(field+".lines", b.Header.Lines, 1, 6)
	}
	if b.Banner != nil {
		kinds = append(kinds, "banner")
		if !BannerRE.MatchString(b.Banner.Text) {
			c.fail("%s.text: %q must match %s", field, b.Banner.Text, BannerRE)
		}
		if len(b.Banner.Gradient) > 6 {
			c.fail("%s.gradient: at most 6 colors", field)
		}
		for i, col := range b.Banner.Gradient {
			if !colorRE.MatchString(col) {
				c.fail("%s.gradient[%d]: %q must be a #rrggbb color", field, i, col)
			}
		}
	}
	if b.Card != nil {
		kinds = append(kinds, "card")
		c.layoutLines(field, *b.Card, 1, 10)
	}
	if b.Notice != nil {
		kinds = append(kinds, "notice")
		c.layoutLines(field, *b.Notice, 1, 10)
	}
	if b.List != nil {
		kinds = append(kinds, "list")
		c.layoutText(field+".title", b.List.Title, false)
		c.layoutLines(field+".items", b.List.Items, 1, 10)
	}
	if b.Text != nil {
		kinds = append(kinds, "text")
		c.layoutLines(field, *b.Text, 1, 10)
	}
	if b.Rule != nil {
		kinds = append(kinds, "rule")
	}
	if b.Status != nil {
		kinds = append(kinds, "status")
		c.layoutText(field+".idle", b.Status.Idle, false)
	}
	if b.Input != nil {
		kinds = append(kinds, "input")
		switch b.Input.Style {
		case InputBare, InputRuled, InputBoxed:
		default:
			c.fail("%s.style: %q must be bare, ruled or boxed", field, b.Input.Style)
		}
		c.str(field+".marker", b.Input.Marker, true)
		c.layoutText(field+".placeholder", b.Input.Placeholder, false)
	}
	if b.Footer != nil {
		kinds = append(kinds, "footer")
		c.layoutText(field+".highlight", b.Footer.Highlight, false)
		c.layoutText(field+".left", b.Footer.Left, false)
		c.layoutText(field+".center", b.Footer.Center, false)
		c.layoutText(field+".right", b.Footer.Right, false)
	}
	if len(kinds) != 1 {
		c.fail("%s: a block needs exactly one type, got %d %v", field, len(kinds), kinds)
		return ""
	}
	return kinds[0]
}

// Blocks that may appear in each part of a layout.
var (
	welcomeBlocks = map[string]bool{"header": true, "banner": true, "card": true, "notice": true, "list": true, "text": true, "rule": true}
	liveBlocks    = map[string]bool{"status": true, "input": true, "footer": true, "rule": true}
)

func (c *checker) layout(a *Agent) {
	if len(a.Welcome) == 0 || len(a.Welcome) > 8 {
		c.fail("welcome: need 1 to 8 blocks")
	}
	for i, b := range a.Welcome {
		field := fmt.Sprintf("welcome[%d]", i)
		if kind := c.block(field, b); kind != "" && !welcomeBlocks[kind] {
			c.fail("%s: %s is not allowed in welcome", field, kind)
		}
	}

	if len(a.Live) > 6 {
		c.fail("live: at most 6 blocks")
	}
	count := map[string]int{}
	for i, b := range a.Live {
		field := fmt.Sprintf("live[%d]", i)
		kind := c.block(field, b)
		if kind != "" && !liveBlocks[kind] {
			c.fail("%s: %s is not allowed in live", field, kind)
		}
		count[kind]++
	}
	if count["input"] != 1 || count["status"] != 1 {
		c.fail("live: need exactly one status and one input block")
	}
	if count["footer"] > 1 {
		c.fail("live: at most one footer")
	}

	switch a.Tools {
	case ToolsBoxed, ToolsInline, ToolsBullet, ToolsCheck:
	default:
		c.fail("tools: %q must be boxed, inline, bullet or check", a.Tools)
	}
}

// InputBlock returns the agent's input block. Validation guarantees one.
func (a *Agent) InputBlock() *Input {
	for _, b := range a.Live {
		if b.Input != nil {
			return b.Input
		}
	}
	return nil
}
