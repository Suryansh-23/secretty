# Stage 1 - foundation and scaffolding

## Goals
- Establish repository structure, Go module, and build tooling.
- Implement core types and config schema as source of truth.
- Set up lint/format/test scaffolding (even minimal).

## Features in this stage
- Go module init and directory layout.
- Config structs mirroring YAML schema.
- Config loader with defaults and validation hooks.
- Error handling and logging primitives (sanitized by default).
- Basic CLI skeleton wiring (without PTY or redaction behavior yet).

## Architecture notes
- Keep packages per spec: `cmd/secretty`, `internal/config`, `internal/debug`, `internal/types` (or `internal/model`).
- Avoid cyclic imports: config/types used by detect/redact, not vice versa.
- All logging should support a "sanitized" mode that never prints raw secrets.

## Proposed directory skeleton
```
cmd/secretty/
internal/config/
internal/types/
internal/debug/
internal/ptywrap/
internal/ansi/
internal/detect/
internal/redact/
internal/cache/
internal/clipboard/
internal/ui/
```

## Canonical YAML (from spec)
Note: The spec uses non-ASCII glyphs for placeholders and block masking. Keep the actual defaults using:
- Placeholder brackets: U+27E6 (LEFT WHITE SQUARE BRACKET) and U+27E7 (RIGHT WHITE SQUARE BRACKET).
- Block mask char: U+2588 (FULL BLOCK).

```yaml
version: 1

mode: strict          # demo | strict | warn
strict:
  no_reveal: true
  lock_until_exit: false
  disable_copy_original: false

redaction:
  default_action: mask        # mask | placeholder
  placeholder_template: "<REDACTED:{type}>" # use U+27E6/U+27E7 in actual default
  include_event_id: false
  rolling_window_bytes: 32768
  status_line:
    enabled: true
    rate_limit_ms: 2000

masking:
  style: glow
  block_char: "U+2588" # full block in actual default
  hex_random_same_length:
    uppercase: false
  stable_hash_token:
    enabled: false
    tag_len: 8

overrides:
  copy_without_render:
    enabled: true
    ttl_seconds: 30
    require_confirm: true
    backend: pbcopy           # pbcopy (macOS) only in MVP

rulesets:
  web3:
    enabled: true
    allow_bare_64hex: false

rules:
  - name: env_private_key
    enabled: true
    type: regex              # regex | typed
    action: mask             # mask | placeholder
    severity: high
    regex:
      pattern: "(?i)\\bPRIVATE_KEY\\s*=\\s*([^\\s]+)"
      group: 1
    context_keywords: ["private_key", "secret", "sk", "--private-key"]

typed_detectors:
  - name: evm_private_key
    enabled: true
    kind: EVM_PRIVATE_KEY
    action: mask
    severity: high
    context_keywords: ["private_key", "--private-key", "secret", "sk="]

debug:
  enabled: false
  log_events: false
```

## Config struct sketch (Go)
```go
type Config struct {
  Version int `yaml:"version"`
  Mode    Mode `yaml:"mode"`
  Strict  Strict `yaml:"strict"`
  Redaction Redaction `yaml:"redaction"`
  Masking   Masking   `yaml:"masking"`
  Overrides Overrides `yaml:"overrides"`
  Rulesets  Rulesets  `yaml:"rulesets"`
  Rules     []Rule    `yaml:"rules"`
  TypedDetectors []TypedDetector `yaml:"typed_detectors"`
  Debug Debug `yaml:"debug"`
}
```

## Config loading flow
- `LoadConfig(pathOverride string) (Config, error)`
- Resolution order:
  1) `--config` if provided
  2) `~/.config/secretty/config.yaml`
  3) defaults
- If config missing: load defaults and emit a one-time hint to run `secretty init`.

## Validation checklist
- Required fields are present or have defaults.
- Enum strings for mode/action are validated.
- Rolling window and TTL are within safe bounds.

## Tests
- Unit: config parse, defaults, validation errors.
- Golden: parsing canonical YAML example.

## Validation plan
- Run `gofmt -l` on touched Go files; expect no output.
- Run `go test ./...` and confirm config tests pass.
- Run `go vet ./...` (or `golangci-lint run` if configured) to catch static issues.
- Verify config defaults in tests match the spec constants.

## Acceptance criteria
- `go test ./...` passes with config tests.
- Config defaults match spec-defined values.
- No logging path prints sensitive data by default.

## Risks and mitigations
- Risk: config drift vs spec. Mitigation: treat this package as canonical and add explicit tests that mirror spec defaults.

## Deliverables
- Go module (`go.mod`) and compileable minimal CLI.
- Config loader with full struct coverage and tests.
