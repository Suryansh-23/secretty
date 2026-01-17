package redact

import (
	"io"

	"github.com/suryansh-23/secretty/internal/ansi"
	"github.com/suryansh-23/secretty/internal/config"
)

// Stream applies redaction to a byte stream and writes to an output.
type Stream struct {
	out        io.Writer
	tokenizer  *ansi.Tokenizer
	detector   Detector
	redactor   *Redactor
	windowSize int
	buffer     []byte
}

// NewStream returns a streaming redactor writer.
func NewStream(out io.Writer, cfg config.Config, detector Detector) *Stream {
	if detector == nil {
		detector = NoopDetector{}
	}
	windowSize := cfg.Redaction.RollingWindowBytes
	if windowSize <= 0 {
		windowSize = 32768
	}
	return &Stream{
		out:        out,
		tokenizer:  &ansi.Tokenizer{},
		detector:   detector,
		redactor:   NewRedactor(cfg),
		windowSize: windowSize,
	}
}

// Write processes input bytes and writes redacted output.
func (s *Stream) Write(p []byte) (int, error) {
	segments := s.tokenizer.Push(p)
	for _, seg := range segments {
		if seg.Kind == ansi.SegmentEscape {
			if _, err := s.out.Write(seg.Bytes); err != nil {
				return 0, err
			}
			continue
		}
		if err := s.processText(seg.Bytes); err != nil {
			return 0, err
		}
	}
	return len(p), nil
}

// Close flushes any pending data.
func (s *Stream) Close() error {
	return s.Flush()
}

// Flush drains tokenizer and rolling buffer.
func (s *Stream) Flush() error {
	segments := s.tokenizer.Flush()
	for _, seg := range segments {
		if _, err := s.out.Write(seg.Bytes); err != nil {
			return err
		}
	}
	if len(s.buffer) == 0 {
		return nil
	}
	matches := s.detector.Find(s.buffer)
	redacted, err := s.redactor.Apply(s.buffer, matches)
	if err != nil {
		return err
	}
	if _, err := s.out.Write(redacted); err != nil {
		return err
	}
	s.buffer = nil
	return nil
}

func (s *Stream) processText(text []byte) error {
	s.buffer = append(s.buffer, text...)
	emitLen := 0
	if len(s.buffer) > s.windowSize {
		emitLen = len(s.buffer) - s.windowSize
	}
	if emitLen == 0 {
		return nil
	}
	matches := s.detector.Find(s.buffer)
	emitLen = safeEmitLen(emitLen, matches)
	if emitLen == 0 {
		return nil
	}
	emitBuf := s.buffer[:emitLen]
	keepBuf := s.buffer[emitLen:]
	emitMatches := filterMatches(matches, emitLen)
	redacted, err := s.redactor.Apply(emitBuf, emitMatches)
	if err != nil {
		return err
	}
	if _, err := s.out.Write(redacted); err != nil {
		return err
	}
	s.buffer = append([]byte(nil), keepBuf...)
	return nil
}

func safeEmitLen(emitLen int, matches []Match) int {
	if emitLen <= 0 {
		return 0
	}
	for {
		changed := false
		for _, m := range matches {
			if m.Start < emitLen && m.End > emitLen {
				emitLen = m.Start
				changed = true
			}
		}
		if !changed {
			break
		}
		if emitLen <= 0 {
			return 0
		}
	}
	return emitLen
}

func filterMatches(matches []Match, emitLen int) []Match {
	if len(matches) == 0 {
		return nil
	}
	out := make([]Match, 0, len(matches))
	for _, m := range matches {
		if m.Start >= 0 && m.End <= emitLen {
			out = append(out, m)
		}
	}
	return out
}
