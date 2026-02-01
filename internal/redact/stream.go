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
	if s.windowSize == 0 {
		if err := s.writeInteractiveSegments(segments); err != nil {
			return 0, err
		}
		return len(p), nil
	}
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

type segmentInfo struct {
	index int
	start int
	end   int
}

func (s *Stream) writeInteractiveSegments(segments []ansi.Segment) error {
	var plain []byte
	infos := make([]segmentInfo, 0, len(segments))
	for i, seg := range segments {
		if seg.Kind != ansi.SegmentText {
			continue
		}
		start := len(plain)
		plain = append(plain, seg.Bytes...)
		infos = append(infos, segmentInfo{index: i, start: start, end: len(plain)})
	}

	var matches []Match
	var matchesBySeg map[int][]Match
	if len(plain) > 0 {
		matches = s.detector.Find(plain)
		matches = s.assignIDs(matches)
		s.storeMatches(plain, matches)
		matchesBySeg = splitMatchesBySegment(matches, infos)
		s.logMatches(matches)
	}

	infoIdx := 0
	for _, seg := range segments {
		if seg.Kind == ansi.SegmentEscape {
			s.updateAltScreen(seg.Bytes)
			if _, err := s.out.Write(seg.Bytes); err != nil {
				return err
			}
			continue
		}
		var chunk []byte
		if infoIdx < len(infos) {
			infoIdx++
			segMatches := matchesBySeg[infoIdx-1]
			if len(segMatches) == 0 {
				chunk = seg.Bytes
			} else {
				redacted, err := s.redactor.Apply(seg.Bytes, segMatches)
				if err != nil {
					return err
				}
				chunk = redacted
			}
		} else {
			chunk = seg.Bytes
		}
		if len(chunk) == 0 {
			continue
		}
		if _, err := s.out.Write(chunk); err != nil {
			return err
		}
		segMatches := matchesBySeg[infoIdx-1]
		s.maybeEmitStatus(segMatches, chunk)
	}
	return nil
}

func splitMatchesBySegment(matches []Match, infos []segmentInfo) map[int][]Match {
	if len(matches) == 0 || len(infos) == 0 {
		return nil
	}
	out := make(map[int][]Match)
	for _, m := range matches {
		for i, info := range infos {
			if m.End <= info.start || m.Start >= info.end {
				continue
			}
			start := max(m.Start, info.start)
			end := min(m.End, info.end)
			if end <= start {
				continue
			}
			out[i] = append(out[i], Match{
				Start:      start - info.start,
				End:        end - info.start,
				Action:     m.Action,
				SecretType: m.SecretType,
				RuleName:   m.RuleName,
				ID:         m.ID,
			})
		}
	}
	return out
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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
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
