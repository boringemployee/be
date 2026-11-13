package engine

// Kind is the type of an Event.
type Kind int

// Event kinds. Renderers decide how each one looks.
const (
	EventWelcome   Kind = iota // Text: title, Lines: banner then greeting
	EventPrompt                // Text: prompt; the session is waiting for input
	EventEcho                  // Text: the line the user typed
	EventNotice                // Lines: informational output (help, topics, hints)
	EventError                 // Text: error message
	EventTaskStart             // Text: task title
	EventThinking              // Text: thinking word; show a spinner until the next event
	EventRead                  // Text: file path
	EventEdit                  // Text: file path, Lines: diff lines
	EventCommand               // Text: command, Lines: output, OK: success
	EventTaskDone              // Text: completion message
	EventStopped               // Text: message shown after Esc
	EventExit                  // The session ended
)

var kindNames = [...]string{
	"welcome", "prompt", "echo", "notice", "error", "task_start", "thinking",
	"read", "edit", "command", "task_done", "stopped", "exit",
}

// String returns a stable lowercase name, used in golden files.
func (k Kind) String() string {
	if k < 0 || int(k) >= len(kindNames) {
		return "unknown"
	}
	return kindNames[k]
}

// Event is one thing for a frontend to display.
type Event struct {
	Kind  Kind
	Text  string
	Lines []string
	OK    bool
}

// Key is a special key press.
type Key int

// Keys the engine reacts to.
const (
	KeyEsc Key = iota + 1
	KeyCtrlC
)

// State is the session state.
type State int

// Session states.
const (
	StateIdle State = iota
	StateRunning
	StateExited
)
