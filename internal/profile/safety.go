package profile

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Limits for content strings and lists.
const (
	MaxTextLen = 200
	MaxItems   = 100
)

var (
	urlRE    = regexp.MustCompile(`(?i)\b[a-z][a-z0-9+.-]*://|\bwww\.`)
	ipv4RE   = regexp.MustCompile(`\b\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}\b`)
	ipv6RE   = regexp.MustCompile(`(?i)\b[0-9a-f]{1,4}(:[0-9a-f]{0,4}){4,7}\b`)
	base64RE = regexp.MustCompile(`[A-Za-z0-9+/=_-]{40,}`)
	pipeRE   = regexp.MustCompile(`\|\s*(sudo\s+)?(sh|bash|zsh|fish|python[0-9.]*|perl|ruby|node|pwsh|powershell)\b`)

	// Giveaways must never appear in anything the UI shows: sessions should
	// look like real agent output.
	Giveaways = regexp.MustCompile(`(?i)\b(fake|faking|pretend\w*|parod\w*|joke|look(ing)? busy|not real|boring)\b`)

	dangerous = []string{
		"rm -rf", "rm -fr", "sudo ", "mkfs", "dd if=", ":(){", "chmod 777", "chmod -r 777",
		"curl ", "wget ", "invoke-webrequest", "iwr ", "nc -e", "/dev/tcp", "> /dev/sd",
		"format c:", "del /f", "eval(", "base64 -d", "base64 --decode",
	}
)

// CheckText reports why s is not safe to show, or nil if it is.
func CheckText(s string) error {
	if !utf8.ValidString(s) {
		return errors.New("invalid UTF-8")
	}
	if n := utf8.RuneCountInString(s); n > MaxTextLen {
		return fmt.Errorf("too long (%d > %d characters)", n, MaxTextLen)
	}
	for _, r := range s {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) || r == ' ' || r == ' ' {
			return fmt.Errorf("contains control or invisible character %U", r)
		}
	}
	switch {
	case urlRE.MatchString(s):
		return errors.New("contains a URL")
	case ipv4RE.MatchString(s), ipv6RE.MatchString(s):
		return errors.New("contains an IP address")
	case base64RE.MatchString(s):
		return errors.New("contains an encoded blob")
	case pipeRE.MatchString(strings.ToLower(s)):
		return errors.New("pipes into an interpreter")
	}
	lower := strings.ToLower(s)
	for _, d := range dangerous {
		if strings.Contains(lower, d) {
			return fmt.Errorf("contains dangerous command %q", strings.TrimSpace(d))
		}
	}
	return nil
}
