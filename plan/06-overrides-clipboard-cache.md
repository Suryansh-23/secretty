# Stage 6 - secret cache and copy-without-render

## Goals
- Implement in-memory secret cache with TTL and optional disablement.
- Add `secretty copy` to place original secret in clipboard without printing.
- Enforce strict mode policy.

## Cache design
- Map: `eventID -> SecretRecord` (original bytes, type, timestamps).
- TTL sweeper to evict expired entries.
- LRU eviction for max capacity (e.g., 64 entries).

## Pseudocode
```go
type SecretRecord struct {
  Type SecretType
  Original []byte
  CreatedAt time.Time
  ExpiresAt time.Time
}

func (c *Cache) Put(id int, r SecretRecord) { /* LRU + TTL */ }
func (c *Cache) GetLast() (SecretRecord, bool) { /* most recent */ }
```

## Clipboard integration
- macOS via `pbcopy`, Linux via `wl-copy`/`xclip`/`xsel`.
- Ensure copy operation never prints original to stdout/stderr.
- Optionally require confirmation (`require_confirm`).

## Strict mode behavior
- Strict mode must never render originals on screen.
- Config flag may disable copy-original entirely.
- In strict mode, cache can be disabled or limited.

## Acceptance criteria
- `secretty copy` copies the last secret when allowed.
- In strict mode with copy disabled, the command fails safely with a message.
- No logs contain original bytes.

## Tests
- Cache TTL expiration.
- Copy returns correct bytes.
- Strict mode policy enforcement.

## Validation plan
- Run `go test ./internal/cache ./internal/clipboard` for cache and clipboard logic.
- Manual: `go run ./cmd/secretty run -- printf \"0x<64hex>\\n\"` then `go run ./cmd/secretty copy` and verify `pbpaste` returns the original bytes.
- Manual strict-mode check: `go run ./cmd/secretty --strict copy` should fail safely if `disable_copy_original=true`.
- Confirm no stdout/stderr prints the original secret during copy.

## Risks
- Leakage through debug logs. Mitigation: sanitize log paths and enforce safe logging APIs.
