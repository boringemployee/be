// Package tty connects the engine to a real terminal in raw mode.
package tty

import "unicode/utf8"

// InputKind is the type of a parsed key press.
type InputKind int

// Parsed input kinds.
const (
	InputRune      InputKind = iota // a printable character
	InputEnter                      // submit the line
	InputBackspace                  // delete one character
	InputClear                      // Ctrl+U: clear the line
	InputEsc                        // stop the current task
	InputInterrupt                  // Ctrl+C: exit
	InputEOF                        // Ctrl+D: exit when the line is empty
)

// Input is one key press.
type Input struct {
	Kind InputKind
	Rune rune
}

// Parse splits a chunk of raw terminal bytes into key presses. Escape
// sequences (arrows, function keys) are ignored; a lone ESC byte is Esc.
func Parse(chunk []byte) []Input {
	var out []Input
	for i := 0; i < len(chunk); {
		c := chunk[i]
		switch {
		case c == 0x1b:
			if i+1 == len(chunk) {
				out = append(out, Input{Kind: InputEsc})
				return out
			}
			i += escapeLen(chunk[i:])
			continue
		case c == '\r' || c == '\n':
			out = append(out, Input{Kind: InputEnter})
		case c == 0x7f || c == 0x08:
			out = append(out, Input{Kind: InputBackspace})
		case c == 0x15:
			out = append(out, Input{Kind: InputClear})
		case c == 0x03:
			out = append(out, Input{Kind: InputInterrupt})
		case c == 0x04:
			out = append(out, Input{Kind: InputEOF})
		case c < 0x20:
			// other control keys are ignored
		default:
			r, size := utf8.DecodeRune(chunk[i:])
			if r != utf8.RuneError {
				out = append(out, Input{Kind: InputRune, Rune: r})
			}
			i += size
			continue
		}
		i++
	}
	return out
}

// escapeLen returns the length of the escape sequence at the start of b.
func escapeLen(b []byte) int {
	switch b[1] {
	case '[': // CSI: ESC [ params final(0x40-0x7e)
		for j := 2; j < len(b); j++ {
			if b[j] >= 0x40 && b[j] <= 0x7e {
				return j + 1
			}
		}
		return len(b)
	case 'O': // SS3: ESC O x
		return min(3, len(b))
	default: // Alt+key: ESC x
		return 2
	}
}
