package ptywrap

import "time"

const responseDrainWindow = 1500 * time.Millisecond

type responseFilter struct {
	deadline time.Time
	buffer   []byte
}

func newResponseFilter(window time.Duration) *responseFilter {
	return &responseFilter{deadline: time.Now().Add(window)}
}

func (f *responseFilter) active() bool {
	return time.Now().Before(f.deadline)
}

func (f *responseFilter) Flush() []byte {
	if len(f.buffer) == 0 {
		return nil
	}
	out := append([]byte(nil), f.buffer...)
	f.buffer = f.buffer[:0]
	return out
}

func (f *responseFilter) Filter(in []byte) []byte {
	f.buffer = append(f.buffer, in...)
	var out []byte
	for len(f.buffer) > 0 {
		if !f.active() {
			out = append(out, f.buffer...)
			f.buffer = f.buffer[:0]
			break
		}
		if f.buffer[0] != 0x1b {
			out = append(out, f.buffer[0])
			f.buffer = f.buffer[1:]
			continue
		}
		if len(f.buffer) < 2 {
			break
		}
		if f.buffer[1] == ']' {
			if seqLen, ok := osc11ResponseLen(f.buffer); ok {
				f.buffer = f.buffer[seqLen:]
				continue
			}
		}
		if f.buffer[1] == '[' {
			if seqLen, ok := dsrResponseLen(f.buffer); ok {
				f.buffer = f.buffer[seqLen:]
				continue
			}
		}
		out = append(out, f.buffer[0])
		f.buffer = f.buffer[1:]
	}
	return out
}

func osc11ResponseLen(buf []byte) (int, bool) {
	if len(buf) < 5 {
		return 0, false
	}
	if buf[0] != 0x1b || buf[1] != ']' || buf[2] != '1' || buf[3] != '1' {
		return 0, false
	}
	start := 4
	if buf[start] == ';' {
		start++
	}
	for i := start; i < len(buf); i++ {
		if buf[i] == 0x07 { // BEL
			return i + 1, true
		}
		if buf[i] == 0x1b && i+1 < len(buf) && buf[i+1] == '\\' { // ST
			return i + 2, true
		}
	}
	return 0, false
}

func dsrResponseLen(buf []byte) (int, bool) {
	if len(buf) < 4 {
		return 0, false
	}
	if buf[0] != 0x1b || buf[1] != '[' {
		return 0, false
	}
	i := 2
	seenDigit := false
	for i < len(buf) {
		b := buf[i]
		if b >= '0' && b <= '9' {
			seenDigit = true
			i++
			continue
		}
		if b == ';' {
			i++
			continue
		}
		break
	}
	if !seenDigit || i >= len(buf) {
		return 0, false
	}
	if buf[i] == 'R' {
		return i + 1, true
	}
	return 0, false
}
