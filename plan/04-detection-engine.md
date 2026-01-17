# Stage 4 - detection engine (regex + typed validators)

## Goals
- Implement typed detector for EVM private keys.
- Implement regex rule engine with capture groups.
- Add minimal context scoring and overlap resolution.

## Features in this stage
- Typed detector: `EVM_PRIVATE_KEY` (0x + 64 hex, optional bare 64 hex).
- Regex rule execution from config.
- Context keyword scoring.
- Overlap conflict resolution.

## Typed detector pseudocode
```go
func DetectEvmPrivateKey(token []byte, allowBare bool) bool {
  t := bytes.TrimPrefix(token, []byte("0x"))
  if len(t) != 64 { return false }
  return isHex(t)
}
```

## Context scoring (minimal)
- Score components:
  - +2 if typed validator passes
  - +1 if context keyword nearby
  - +1 if token starts with 0x
- Threshold >= 2
- Optional negative contexts to reduce false positives (e.g., "hash=", "tx=")

## Regex rule execution
- Compile all regex patterns at config load.
- Each rule yields spans for group `rule.Regex.Group`.

## Overlap resolution
1) Higher severity wins.
2) Typed detector wins over regex.
3) Longer span wins.
4) Earlier start index wins.

## Acceptance criteria
- Typed detector catches 0x + 64 hex values.
- Regex rules fire on `.env` style patterns.
- Overlaps are resolved deterministically.

## Tests
- Typed detector with valid/invalid cases.
- Regex rule with group extraction.
- Overlap resolution with synthetic cases.
- Context scoring with positive/negative keywords.

## Validation plan
- Run `go test ./internal/detect` and ensure all detector/unit cases pass.
- Add a regression test with a bare 64-hex token and confirm it is NOT redacted when `allow_bare_64hex=false`.
- Add a regression test with `0x` + 64 hex and confirm it is redacted.
- Benchmark or quick micro-test to ensure regex match time remains bounded for 32 KiB window.

## Risks
- False positives from bare 64 hex strings. Mitigation: default `allow_bare_64hex` false, context scoring.
- Regex performance on large buffers. Mitigation: precompile and limit window size.
