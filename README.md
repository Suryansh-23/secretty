# SecreTTY

SecreTTY is a macOS-only PTY wrapper that redacts secrets from terminal output before they reach the screen. It is designed for live demos, screen shares, and recordings where accidental secret exposure is a risk.

## Status
- MVP implementation is functional for core flows (PTY wrapper, redaction pipeline, detection, init wizard, copy last, status line, doctor).
- Strict mode and policy controls are implemented in config.
- `copy last` is currently **in-process only** (see Limitations).

## Key features
- Runs shells/commands under a PTY to preserve terminal semantics.
- Redacts secrets inline with masking or placeholders.
- ANSI-aware tokenizer (no mutation of escape sequences).
- Typed EVM private key detection + regex rules.
- Optional status line with rate limiting.
- Copy-without-render to clipboard (macOS `pbcopy`).
- Animated onboarding wizard with theme + logo.

## Install
Homebrew tap (planned):
```
brew install suryansh-23/tap/secretty
```

## Build
Requires Go 1.24+ (tested with Go 1.25).

```
make build
```

Binary output: `bin/secretty`

## Run
```
./bin/secretty
./bin/secretty shell -- zsh
./bin/secretty run -- printf "PRIVATE_KEY=0x<64hex>\n"
./bin/secretty init
./bin/secretty copy last
./bin/secretty doctor
```

## Onboarding
```
./bin/secretty init
```
The wizard shows an animated logo header and guides the user through mode, ruleset, and clipboard settings before writing `~/.config/secretty/config.yaml`.

## Configuration
Default path:
```
~/.config/secretty/config.yaml
```

Example config (ASCII placeholder form):
```yaml
version: 1

mode: demo
strict:
  no_reveal: true
  lock_until_exit: false
  disable_copy_original: false

redaction:
  default_action: mask
  placeholder_template: "<REDACTED:{type}>"
  include_event_id: false
  rolling_window_bytes: 32768
  status_line:
    enabled: true
    rate_limit_ms: 2000

masking:
  block_char: "*"
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
    backend: pbcopy

rulesets:
  web3:
    enabled: true
    allow_bare_64hex: false

rules:
  - name: env_private_key
    enabled: true
    type: regex
    action: mask
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

## Development
```
make fmt
make lint
make test
make build
make smoke
```

## Limitations
- macOS-only MVP.
- `copy last` is **in-process only** (a new `secretty` invocation cannot access secrets from a prior session).
- tmux compatibility is not guaranteed.
- Interactive shells run with unbuffered output to preserve prompt responsiveness; this can reduce cross-chunk redaction for extremely fragmented output.

## Security invariants
- Never print or log original secret bytes.
- Never mutate ANSI escape sequences.
- Redaction must handle chunk boundaries correctly.
