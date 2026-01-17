package ansi

// SegmentKind identifies tokenizer output types.
type SegmentKind int

const (
	SegmentText SegmentKind = iota
	SegmentEscape
)

// Segment holds a classified byte sequence.
type Segment struct {
	Kind  SegmentKind
	Bytes []byte
}

type escState int

const (
	stateText escState = iota
	stateEscStart
	stateCSI
	stateOSC
	stateDCS
	stateSOS
	statePM
	stateAPC
)

// Tokenizer splits ANSI escape sequences from text in a streaming-safe way.
type Tokenizer struct {
	state       escState
	escBuf      []byte
	escInString bool
}

// Push processes a chunk of bytes and returns completed segments.
func (t *Tokenizer) Push(data []byte) []Segment {
	var segments []Segment
	var textBuf []byte

	flushText := func() {
		if len(textBuf) == 0 {
			return
		}
		segments = append(segments, Segment{Kind: SegmentText, Bytes: append([]byte(nil), textBuf...)})
		textBuf = textBuf[:0]
	}
	flushEsc := func() {
		if len(t.escBuf) == 0 {
			return
		}
		segments = append(segments, Segment{Kind: SegmentEscape, Bytes: append([]byte(nil), t.escBuf...)})
		t.escBuf = t.escBuf[:0]
		t.escInString = false
		t.state = stateText
	}

	for _, b := range data {
		switch t.state {
		case stateText:
			if b == 0x1b { // ESC
				flushText()
				t.escBuf = append(t.escBuf, b)
				t.state = stateEscStart
				continue
			}
			textBuf = append(textBuf, b)
		case stateEscStart:
			t.escBuf = append(t.escBuf, b)
			switch b {
			case '[':
				t.state = stateCSI
			case ']':
				t.state = stateOSC
			case 'P':
				t.state = stateDCS
			case 'X':
				t.state = stateSOS
			case '^':
				t.state = statePM
			case '_':
				t.state = stateAPC
			default:
				flushEsc()
			}
		case stateCSI:
			t.escBuf = append(t.escBuf, b)
			if b >= 0x40 && b <= 0x7e {
				flushEsc()
			}
		case stateOSC, stateDCS, stateSOS, statePM, stateAPC:
			t.escBuf = append(t.escBuf, b)
			if t.state == stateOSC && b == 0x07 { // BEL terminator
				flushEsc()
				continue
			}
			if t.escInString {
				if b == '\\' { // ST sequence ESC \
					flushEsc()
					continue
				}
				t.escInString = false
				continue
			}
			if b == 0x1b {
				t.escInString = true
			}
		}
	}

	if t.state == stateText {
		flushText()
	}
	return segments
}

// Flush emits any pending bytes as an escape segment.
func (t *Tokenizer) Flush() []Segment {
	if t.state == stateText {
		return nil
	}
	seg := Segment{Kind: SegmentEscape, Bytes: append([]byte(nil), t.escBuf...)}
	t.escBuf = nil
	t.state = stateText
	t.escInString = false
	return []Segment{seg}
}
