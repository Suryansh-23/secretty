package redact

import (
	"bytes"
	"io"
	"time"

	"github.com/suryansh-23/secretty/internal/ansi"
	"github.com/suryansh-23/secretty/internal/cache"
	"github.com/suryansh-23/secretty/internal/config"
	"github.com/suryansh-23/secretty/internal/debug"
	"github.com/suryansh-23/secretty/internal/types"
	"github.com/suryansh-23/secretty/internal/ui"
)

// Stream applies redaction to a byte stream and writes to an output.
type Stream struct {
	out        io.Writer
	tokenizer  *ansi.Tokenizer
	detector   Detector
	redactor   *Redactor
	windowSize int
	buffer     []byte
	cache      *cache.Cache
	nextID     int
	cacheOn    bool
	includeID  bool
	strictMode bool
	logger     *debug.Logger

	statusEnabled   bool
	statusRateLimit time.Duration
	lastStatus      time.Time
	altScreen       bool
}

// NewStream returns a streaming redactor writer.
func NewStream(out io.Writer, cfg config.Config, detector Detector, secretCache *cache.Cache, logger *debug.Logger) *Stream {
	if detector == nil {
		detector = NoopDetector{}
	}
	windowSize := cfg.Redaction.RollingWindowBytes
	if windowSize < 0 {
		windowSize = 32768
	}
	cacheOn := cfg.Overrides.CopyWithoutRender.Enabled
	if cfg.Mode == types.ModeStrict && cfg.Strict.DisableCopyOriginal {
		cacheOn = false
	}
	statusEnabled := cfg.Redaction.StatusLine.Enabled
	statusRateLimit := time.Duration(cfg.Redaction.StatusLine.RateLimitMS) * time.Millisecond
	return &Stream{
		out:             out,
		tokenizer:       &ansi.Tokenizer{},
		detector:        detector,
		redactor:        NewRedactor(cfg),
		windowSize:      windowSize,
		cache:           secretCache,
		cacheOn:         cacheOn,
		includeID:       cfg.Redaction.IncludeEventID,
		strictMode:      cfg.Mode == types.ModeStrict,
		logger:          logger,
		statusEnabled:   statusEnabled,
		statusRateLimit: statusRateLimit,
	}
}

// Write processes input bytes and writes redacted output.
func (s *Stream) Write(p []byte) (int, error) {
	segments := s.tokenizer.Push(p)
	for _, seg := range segments {
		if seg.Kind == ansi.SegmentEscape {
			s.updateAltScreen(seg.Bytes)
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
	matches = s.assignIDs(matches)
	s.storeMatches(s.buffer, matches)
	redacted, err := s.redactor.Apply(s.buffer, matches)
	if err != nil {
		return err
	}
	if _, err := s.out.Write(redacted); err != nil {
		return err
	}
	s.logMatches(matches)
	s.maybeEmitStatus(matches, redacted)
	s.buffer = nil
	return nil
}

func (s *Stream) processText(text []byte) error {
	if s.windowSize == 0 {
		s.buffer = append(s.buffer, text...)
		matches := s.detector.Find(s.buffer)
		matches = s.assignIDs(matches)
		s.storeMatches(s.buffer, matches)
		redacted, err := s.redactor.Apply(s.buffer, matches)
		if err != nil {
			return err
		}
		if _, err := s.out.Write(redacted); err != nil {
			return err
		}
		s.logMatches(matches)
		s.maybeEmitStatus(matches, redacted)
		s.buffer = nil
		return nil
	}

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
	emitMatches = s.assignIDs(emitMatches)
	s.storeMatches(emitBuf, emitMatches)
	redacted, err := s.redactor.Apply(emitBuf, emitMatches)
	if err != nil {
		return err
	}
	if _, err := s.out.Write(redacted); err != nil {
		return err
	}
	s.logMatches(emitMatches)
	s.maybeEmitStatus(emitMatches, redacted)
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

func (s *Stream) updateAltScreen(esc []byte) {
	if bytes.Contains(esc, []byte("[?1049h")) || bytes.Contains(esc, []byte("[?47h")) || bytes.Contains(esc, []byte("[?1047h")) {
		s.altScreen = true
		return
	}
	if bytes.Contains(esc, []byte("[?1049l")) || bytes.Contains(esc, []byte("[?47l")) || bytes.Contains(esc, []byte("[?1047l")) {
		s.altScreen = false
	}
}

func (s *Stream) assignIDs(matches []Match) []Match {
	if len(matches) == 0 {
		return matches
	}
	if !s.includeID && s.cache == nil {
		return matches
	}
	out := append([]Match(nil), matches...)
	for i := range out {
		if out[i].ID == 0 {
			s.nextID++
			out[i].ID = s.nextID
		}
	}
	return out
}

func (s *Stream) storeMatches(text []byte, matches []Match) {
	if s.cache == nil || !s.cacheOn || len(matches) == 0 {
		return
	}
	for _, m := range matches {
		if m.Start < 0 || m.End > len(text) || m.End <= m.Start {
			continue
		}
		s.cache.Put(cache.SecretRecord{
			ID:       m.ID,
			Type:     m.SecretType,
			RuleName: m.RuleName,
			Original: append([]byte(nil), text[m.Start:m.End]...),
		})
	}
}

func (s *Stream) logMatches(matches []Match) {
	if s.logger == nil || len(matches) == 0 {
		return
	}
	for _, m := range matches {
		s.logger.Infof("redact event id=%d type=%s rule=%s action=%s", m.ID, m.SecretType, m.RuleName, m.Action)
	}
}

func (s *Stream) maybeEmitStatus(matches []Match, redacted []byte) {
	if !s.statusEnabled || len(matches) == 0 || s.altScreen {
		return
	}
	if s.statusRateLimit > 0 && time.Since(s.lastStatus) < s.statusRateLimit {
		return
	}
	if len(redacted) == 0 || redacted[len(redacted)-1] != '\n' {
		return
	}
	first := matches[0]
	line := ui.StatusLine(len(matches), s.strictMode, s.includeID, first.SecretType, first.ID)
	if line == "" {
		return
	}
	if _, err := s.out.Write([]byte(line + "\n")); err != nil {
		return
	}
	s.lastStatus = time.Now()
}
