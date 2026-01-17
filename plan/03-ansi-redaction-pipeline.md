# Stage 3 - ANSI tokenizer and redaction pipeline

## Goals
- Build streaming ANSI tokenizer that never mutates escape sequences.
- Implement rolling window buffering for chunk boundary detection.
- Apply replacements only on TEXT spans and reassemble output.

## Core rules
- Never scan inside ANSI escape sequences.
- Never emit altered ESC segments.
- Ensure low latency and bounded memory use.

## Redaction actions and masking strategies
- Actions: `mask` or `placeholder`.
- Mask strategies:
  - `block`: replace all chars with block char (U+2588 in spec).
  - `hex_random_same_length`: replace each hex nibble with random hex (case configurable).
  - `stable_hash_token`: deterministic token from HMAC(session_salt, secret), truncated tag.
- Placeholder template: default uses double-square brackets (U+27E6/U+27E7) and optional event id.

## Tokenizer design
- Input: raw byte stream from PTY.
- Output: stream of segments: ESCAPE or TEXT.
- Maintain state across chunks (partial escape sequences).

## Tokenizer pseudocode
```go
type Segment struct {
  Kind SegmentKind // ESCAPE or TEXT
  Bytes []byte
}

func TokenizeStream(in <-chan []byte) <-chan Segment {
  // Maintain state for escape sequences across chunks.
}
```

## Redaction pipeline pseudocode
```go
for seg := range segments {
  if seg.Kind == ESCAPE {
    write(seg.Bytes)
    continue
  }
  // TEXT segment
  window.Append(seg.Bytes)
  matches := detector.Find(window)
  redacted := redact.Apply(window, matches)
  // emit all but the tail kept for future matching
  emit, keep := splitForRollingWindow(redacted)
  write(emit)
  window.Reset(keep)
}
```

## Rolling window strategy
- Keep a suffix of TEXT bytes (default 32 KiB) to catch tokens split across chunks.
- Ensure that text emitted is never later modified.
- Match detection on current window only, replacing in the portion safe to emit.

## Conflict resolution strategy (placeholder)
- Prefer higher severity.
- Prefer typed detector over regex.
- Prefer longer match.
- Stable ordering by start index.

## Tests
- ANSI: color codes remain intact after redaction.
- Chunk boundary: split a key across writes; still detected.
- High throughput: large output stream; no visible lag.

## Validation plan
- Run `go test ./internal/ansi ./internal/redact` for tokenizer and redaction logic.
- Run golden tests covering ANSI-colored outputs and ensure byte-for-byte ESC preservation.
- Manual smoke: `timeout 10s go run ./cmd/secretty run -- printf '\\033[31mred\\033[0m 0x0123...\\n'` and confirm color is intact while the secret is redacted.

## Acceptance criteria
- ANSI escape sequences are byte-for-byte preserved.
- Redaction never occurs inside ESC sequences.
- Latency remains under the 10ms budget for typical output.

## Risks
- Incorrect tokenizer state on partial ESC sequence. Mitigation: extensive unit tests with partial sequences.
- Over-redaction across boundary. Mitigation: conservative emission of tail bytes.
