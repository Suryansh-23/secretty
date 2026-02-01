package redact

import (
	"bytes"
	"io"
	"regexp"
	"time"
	"unicode/utf8"

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
		emitBuf, tail := splitUTF8Tail(s.buffer)
		if len(emitBuf) == 0 {
			s.buffer = tail
			return nil
		}
		if err := s.writeInteractive(emitBuf); err != nil {
			return err
		}
		s.buffer = tail
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
	emitLen = utf8SafePrefixLen(s.buffer, emitLen)
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

type textRun struct {
	control bool
	bytes   []byte
}

func (s *Stream) writeInteractive(buf []byte) error {
	for _, run := range splitControlRuns(buf) {
		if run.control {
			if _, err := s.out.Write(run.bytes); err != nil {
				return err
			}
			continue
		}
		matches := s.detector.Find(run.bytes)
		matches = s.assignIDs(matches)
		s.storeMatches(run.bytes, matches)
		redacted, err := s.redactor.Apply(run.bytes, matches)
		if err != nil {
			return err
		}
		if _, err := s.out.Write(redacted); err != nil {
			return err
		}
		s.logMatches(matches)
		s.maybeEmitStatus(matches, redacted)
	}
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

func splitUTF8Tail(buf []byte) ([]byte, []byte) {
	if len(buf) == 0 {
		return nil, nil
	}
	start := len(buf) - 1
	for start >= 0 && !utf8.RuneStart(buf[start]) {
		start--
	}
	if start < 0 {
		return nil, buf
	}
	if utf8.FullRune(buf[start:]) {
		return buf, nil
	}
	return buf[:start], buf[start:]
}

func utf8SafePrefixLen(buf []byte, max int) int {
	if max <= 0 {
		return 0
	}
	if max > len(buf) {
		max = len(buf)
	}
	head, _ := splitUTF8Tail(buf[:max])
	return len(head)
}

func splitControlRuns(buf []byte) []textRun {
	if len(buf) == 0 {
		return nil
	}
	var runs []textRun
	start := 0
	currControl := isControlByte(buf[0])
	for i := 1; i < len(buf); i++ {
		nextControl := isControlByte(buf[i])
		if nextControl == currControl {
			continue
		}
		runs = append(runs, textRun{control: currControl, bytes: buf[start:i]})
		start = i
		currControl = nextControl
	}
	runs = append(runs, textRun{control: currControl, bytes: buf[start:]})
	return runs
}

func isControlByte(b byte) bool {
	if b == '\n' || b == '\t' {
		return false
	}
	return b < 0x20 || b == 0x7f
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
		label := extractLabel(text, m)
		s.cache.Put(cache.SecretRecord{
			ID:       m.ID,
			Type:     m.SecretType,
			RuleName: m.RuleName,
			Label:    label,
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

var labelRegex = regexp.MustCompile(`^\s*([A-Za-z_][A-Za-z0-9_-]{0,63})\s*[:=]`)

func extractLabel(text []byte, match Match) string {
	if match.Start < 0 || match.Start > len(text) {
		return ""
	}
	lineStart := bytes.LastIndexByte(text[:match.Start], '\n')
	if lineStart == -1 {
		lineStart = 0
	} else {
		lineStart++
	}
	lineEnd := bytes.IndexByte(text[match.Start:], '\n')
	if lineEnd == -1 {
		lineEnd = len(text)
	} else {
		lineEnd += match.Start
	}
	line := text[lineStart:lineEnd]
	matches := labelRegex.FindSubmatch(line)
	if len(matches) < 2 {
		return ""
	}
	return string(matches[1])
}
