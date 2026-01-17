package ansi

import "testing"

func collect(chunks ...string) []Segment {
	t := &Tokenizer{}
	var out []Segment
	for _, chunk := range chunks {
		out = append(out, t.Push([]byte(chunk))...)
	}
	out = append(out, t.Flush()...)
	return out
}

func TestTokenizerTextOnly(t *testing.T) {
	segs := collect("hello world")
	if len(segs) != 1 {
		t.Fatalf("segments = %d", len(segs))
	}
	if segs[0].Kind != SegmentText || string(segs[0].Bytes) != "hello world" {
		t.Fatalf("unexpected segment: %#v", segs[0])
	}
}

func TestTokenizerCSISequenceAcrossChunks(t *testing.T) {
	segs := collect("hi ", "\x1b[31", "mred")
	if len(segs) != 3 {
		t.Fatalf("segments = %d", len(segs))
	}
	if segs[0].Kind != SegmentText || string(segs[0].Bytes) != "hi " {
		t.Fatalf("segment 0 unexpected: %#v", segs[0])
	}
	if segs[1].Kind != SegmentEscape || string(segs[1].Bytes) != "\x1b[31m" {
		t.Fatalf("segment 1 unexpected: %#v", segs[1])
	}
	if segs[2].Kind != SegmentText || string(segs[2].Bytes) != "red" {
		t.Fatalf("segment 2 unexpected: %#v", segs[2])
	}
}

func TestTokenizerOSCSequence(t *testing.T) {
	segs := collect("pre", "\x1b]0;title", "\x07post")
	if len(segs) != 3 {
		t.Fatalf("segments = %d", len(segs))
	}
	if segs[0].Kind != SegmentText || string(segs[0].Bytes) != "pre" {
		t.Fatalf("segment 0 unexpected: %#v", segs[0])
	}
	if segs[1].Kind != SegmentEscape || string(segs[1].Bytes) != "\x1b]0;title\x07" {
		t.Fatalf("segment 1 unexpected: %#v", segs[1])
	}
	if segs[2].Kind != SegmentText || string(segs[2].Bytes) != "post" {
		t.Fatalf("segment 2 unexpected: %#v", segs[2])
	}
}

func TestTokenizerSingleEscape(t *testing.T) {
	segs := collect("a", "\x1bc", "b")
	if len(segs) != 3 {
		t.Fatalf("segments = %d", len(segs))
	}
	if segs[1].Kind != SegmentEscape || string(segs[1].Bytes) != "\x1bc" {
		t.Fatalf("segment 1 unexpected: %#v", segs[1])
	}
}
