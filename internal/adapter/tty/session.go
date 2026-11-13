package tty

import (
	"io"
	"strings"
	"time"

	"github.com/boringemployee/be/internal/engine"
	"github.com/boringemployee/be/internal/render"
)

// FrameInterval is how often the CLI advances fake time and redraws.
const FrameInterval = 100 * time.Millisecond

// Session drives one engine session on a terminal.
type Session struct {
	Engine   *engine.Engine
	Renderer *render.Renderer
	Out      io.Writer
	// Width, if set, is polled every frame so resizes take effect.
	Width func() int

	input   []rune
	drawn   bool // whether a live area is on screen
	word    string
	frame   int
	elapsed time.Duration
}

// Run starts the session and processes key chunks and frame ticks until the
// session ends or keys is closed.
func (s *Session) Run(keys <-chan []byte, ticks <-chan time.Time) error {
	if err := s.emit(s.Engine.Start()); err != nil {
		return err
	}
	for s.Engine.State() != engine.StateExited {
		select {
		case chunk, ok := <-keys:
			if !ok {
				return s.emit(s.Engine.HandleKey(engine.KeyCtrlC))
			}
			for _, in := range Parse(chunk) {
				if err := s.handle(in); err != nil {
					return err
				}
			}
		case <-ticks:
			if s.Width != nil {
				s.Renderer.SetWidth(s.Width())
			}
			s.frame++
			if s.Engine.State() == engine.StateRunning {
				s.elapsed += FrameInterval
			}
			events := s.Engine.Tick(FrameInterval)
			if len(events) == 0 && s.Engine.State() != engine.StateRunning {
				continue // nothing changed; avoid redrawing an idle prompt
			}
			if err := s.emit(events); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Session) handle(in Input) error {
	switch in.Kind {
	case InputInterrupt:
		return s.emit(s.Engine.HandleKey(engine.KeyCtrlC))
	case InputEOF:
		if len(s.input) == 0 {
			return s.emit(s.Engine.HandleKey(engine.KeyCtrlC))
		}
		return nil
	case InputEsc:
		return s.emit(s.Engine.HandleKey(engine.KeyEsc))
	case InputEnter:
		line := string(s.input)
		s.input = s.input[:0]
		return s.emit(s.Engine.HandleLine(line))
	case InputBackspace:
		if len(s.input) > 0 {
			s.input = s.input[:len(s.input)-1]
		}
	case InputClear:
		s.input = s.input[:0]
	case InputRune:
		if len(s.input) < engine.MaxInputLen {
			s.input = append(s.input, in.Rune)
		}
	}
	return s.emit(nil)
}

// emit prints the permanent output of events above the live area, then
// redraws the live area (status and input box) unless the session ended.
func (s *Session) emit(events []engine.Event) error {
	var b strings.Builder
	if s.drawn {
		b.WriteString(s.Renderer.ClearLive())
	}
	for _, ev := range events {
		switch ev.Kind {
		case engine.EventTaskStart:
			s.word, s.elapsed = "Working", 0
		case engine.EventThinking:
			s.word = ev.Text
		}
		b.WriteString(s.Renderer.Event(ev))
	}
	s.drawn = s.Engine.State() != engine.StateExited
	if s.drawn {
		b.WriteString(s.Renderer.Live(render.LiveState{
			Running: s.Engine.State() == engine.StateRunning,
			Frame:   s.frame,
			Word:    s.word,
			Elapsed: s.elapsed,
			Input:   string(s.input),
		}))
	}
	_, err := io.WriteString(s.Out, b.String())
	return err
}
