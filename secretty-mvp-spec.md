# SecreTTY — Technical MVP Spec (macOS)  
**Version:** v0.1 (MVP)  
**Last updated:** 2026-01-18  
**Primary use-case:** prevent **visual leaks** of secrets in terminal output during screen-share / live demos.

---

## 1. Scope and guarantees

### 1.1 Goals (hard requirements)
1) **Redact secrets in terminal output before they reach the user’s screen.**  
2) **Preserve TTY semantics** (colors, cursor motion, REPLs, full-screen TUIs) by running the child under a PTY. The `creack/pty` `Start` helper assigns a tty to stdin/stdout/stderr and starts the process in a new session with a controlling terminal. citeturn0search0turn0search16  
3) Be **terminal-emulator agnostic**: works with Ghostty/iTerm2/WezTerm/kitty/Terminal.app (since SecreTTY sits between the terminal and the child process).  
4) **macOS-only** for MVP.  
5) **Always-on “Strict recording” mode** inside SecreTTY sessions (redaction enabled by default).  
6) **Local-only**: no network calls; no telemetry.  
7) **Simple UX**: minimal commands; zero configuration required to get value.

### 1.2 Non-goals (explicit)
- Not a defense against malware, keyloggers, or intentional exfiltration.
- No file rewriting by default.
- No Windows support in MVP.
- No tmux compatibility guarantees (should “work often”, but not a contract).

### 1.3 Threat model (what we’re actually preventing)
- Accidental disclosure in:
  - screen-share recordings,
  - live streams,
  - in-person demos,
  - screenshots.
- Not preventing:
  - secrets read directly from disk,
  - clipboard history leaks (optional mitigations exist but are secondary),
  - remote process compromise.

---

## 2. Product shape

### 2.1 What SecreTTY is
A **PTY wrapper** that runs an interactive shell or a command inside a pseudo-terminal, proxies I/O, and **redacts sensitive tokens from the outgoing PTY stream** before writing to the user’s terminal.

### 2.2 What SecreTTY is not
- Not a secrets manager (unlike 1Password `op run` which masks values it injects by default). citeturn0search7turn0search3  
- Not a simple stdout pipe filter (like `mask`, which is designed to pipe text and replace configured strings). citeturn2search1  
- Not a log redaction tool first (though it can add a `redact` stdin filter later, like Teller supports). citeturn1search5turn1search1  

### 2.3 Prior-art UX baseline (what “good” looks like)
Warp’s Secret Redaction feature is regex-based, redacts sensitive data, and is applied to prevent secrets from being sent to servers/LLMs. citeturn0search2turn0search10turn0search6  
SecreTTY’s differentiator: **works with any terminal** (PTY wrapper), and focuses on **live output safety**.

---

## 3. User flows

### 3.1 Primary flow (MVP)
1) User installs via Homebrew.
2) User runs:  
   - `secretty` → launches a safe interactive shell session, OR  
   - `secretty run -- <cmd>` → runs a single command under protection.
3) If a secret prints, it is redacted (mask/placeholder) immediately.
4) If user needs the value, they use explicit “copy-without-render” (never shown on screen).

### 3.2 “Strict recording” flow
- User starts: `secretty --strict`  
- Secrets are redacted; **no reveal-to-screen** is possible; optionally disable “copy original” too (config-controlled).

---

## 4. CLI contract (MVP)

### 4.1 Commands
#### `secretty`
- Starts a protected interactive session (default login shell).
- Equivalent to: `secretty shell -- <default_shell>`

#### `secretty shell -- <shell>`
- Start an interactive shell under PTY.
- Example: `secretty shell -- zsh`

#### `secretty run -- <cmd...>`
- Run a command under PTY and exit with the child’s exit code.
- Example: `secretty run -- anvil`

#### `secretty init`
- First-run wizard (creates config, enables Web3 ruleset, runs self-test). Built with `huh?` forms. citeturn0search1turn0search21  
- `secretty init --default` writes the default config without prompts.

#### `secretty copy last` (MVP)
- Copies the last redacted secret’s **original value** to clipboard **without printing it**.

#### `secretty export --last <N>` (optional but recommended)
- Writes the last N lines of output as a **sanitized transcript** (placeholders preserved, originals never written).

#### `secretty doctor`
- Prints environment diagnosis: shell, TERM, whether inside tmux, PTY size, config path, enabled rules.

### 4.2 Global flags
- `--config <path>`: override config location.
- `--strict`: enable strict mode (no reveal-to-screen; policy may also disable copy-original).
- `--debug`: enable sanitized debug logs (never print originals; prints rule hits and event IDs only).
- `--no-init-hints`: don’t print “run secretty init” guidance.

### 4.3 Exit codes
- `0`: success.
- `1`: runtime error (spawn failure, IO failure, config invalid).
- child exit code forwarded for `run`.

---

## 5. UX patterns (terminal-only)

### 5.1 Redaction rendering (MVP decision)
**Chosen:** Inline replacement + optional minimal status line.

- **Inline:** replace secret span with either:
  - mask-in-place (same length), or
  - placeholder template.

- **Minimal status line (optional):** emit a short non-intrusive line **only when safe**:
  - when not in alt-screen mode, and
  - rate-limited (e.g., once per 2 seconds max).

**No overlay TUI in MVP.** (Can be added later with Bubble Tea + Lip Gloss. Bubble Tea is the underlying TUI framework; Lip Gloss handles ANSI-aware layout/styling. citeturn0search5turn0search9)

### 5.2 “Looks good” guidelines (if any UI text is shown)
- Single-line, compact, low-noise.
- No emojis by default.
- Use consistent tokens:
  - `⟦REDACTED:EVM_PK⟧`
  - `⟦REDACTED:EVM_PK#03⟧` (if event IDs are enabled)
- In strict mode, prefix status: `secretty(strict): …`

---

## 6. Redaction actions (MVP)

### 6.1 Actions supported
- **MASK** (in-place, same-length)
- **PLACEHOLDER** (replace span with template)

### 6.2 Mask strategies (enum)
**Default for MVP:** `hex_random_same_length` for hex-like secrets, else `block` (glow style by default).

#### Strategy options
1) `block`  
   - Replace all chars with `█` or `*` (configurable char).

2) `hex_random_same_length` (default for EVM PK)  
   - Replace each hex nibble with a random hex nibble, preserving case as configured.

3) `stable_hash_token`  
   - Replace full secret with deterministic token derived from `HMAC_SHA256(session_salt, secret)` truncated (e.g., 8 chars).
   - Format: `⟦MASK:{type}:{tag}⟧`
   - Useful for correlating repeated prints in a demo without leaking value.

4) `class_preserving_random` (future)
   - Preserve character classes across base58/base64/hex.

### 6.3 Placeholder template
Default: `⟦REDACTED:{type}⟧`  
Optional with counter: `⟦REDACTED:{type}#{id:02d}⟧`

---

## 7. Overrides and strictness

### 7.1 Supported overrides (MVP)
**Copy-without-render** only.

- `secretty copy last`
- Optionally `secretty copy --id <n>` if IDs are enabled.

### 7.2 Strict mode
- No reveal-to-screen.
- Config can decide whether “copy original” is allowed.

**Rationale:** 1Password exposes explicit knobs to disable masking (`--no-masking`, `OP_RUN_NO_MASKING`), reflecting users expect deliberate controls. citeturn0search7turn0search3  
SecreTTY is the inverse (masking ON), but still needs explicit policy knobs.

### 7.3 In-memory secret cache
- Originals are stored **only in memory**, never on disk.
- Each redaction event may store:
  - secret original bytes,
  - type,
  - expiry timestamp,
  - event id.
- Default TTL: 30 seconds (configurable).
- In strict mode, cache may be disabled.

---

## 8. Detection engine (MVP)

### 8.1 High-level strategy
**Chosen:** regex + typed validators + light context scoring.

#### Why this design
- Warp relies on regex patterns for redaction. citeturn0search2turn0search6  
- Generic regex-only matching for Web3 keys has high false positives (hashes, IDs). Typed validators reduce noise.

### 8.2 Streaming model (must handle chunk boundaries)
Terminal programs often write in chunks; secrets may span multiple writes.

**Approach**
- Maintain a rolling byte window (`rolling_window_bytes`, default 32 KiB).
- Append new PTY bytes.
- Run detection on the rolling buffer.
- Emit output with minimal latency while keeping enough suffix bytes to match future tokens.

**Latency target**
- Additional buffering delay ≤ 10ms under normal throughput.

### 8.3 Typed detectors (MVP)
#### `EVM_PRIVATE_KEY`
- Accept:
  - `0x` + 64 hex
- Config option:
  - `allow_bare_64hex` (default false)
- Optional context boost keywords:
  - `PRIVATE_KEY`, `--private-key`, `sk=`, `secret`, `key=`

**Validator spec**
- Strip optional `0x`.
- Check length == 64.
- Check all hex.
- Optional reject contexts like `hash=` or `tx=` (configurable, off by default).

### 8.4 Regex rules (MVP)
- `.env` style:
  - `(?i)\bPRIVATE_KEY\s*=\s*([^\s]+)` (captures value)
- Note: all regexes must be compiled once at config load.

### 8.5 Context scoring (minimal)
Score components:
- `+2` if typed validator passes
- `+1` if preceded/followed by context keyword within window
- `+1` if token begins with `0x`
Threshold default: `>=2`

---

## 9. ANSI / terminal correctness requirements

### 9.1 Non-negotiables
- Do not corrupt escape sequences.
- Preserve alt-screen behavior.
- Preserve cursor movement and color codes.
- Do not break full-screen programs in common cases.

### 9.2 ANSI-aware redaction rule
- The matcher must not scan inside escape sequences.
- Replacement must only occur on printable text spans.

**Implementation approach**
- Streaming tokenizer that splits bytes into:
  - `ESCAPE` segments (pass-through)
  - `TEXT` segments (eligible for scanning/replacement)
- Maintain tokenizer state across chunk boundaries.

---

## 10. Configuration (YAML)

### 10.1 Location
Default: `~/.config/secretty/config.yaml`

### 10.2 YAML schema (authoritative)
```yaml
version: 1

 mode: strict          # demo | strict | warn
strict:
  no_reveal: true
  lock_until_exit: false
  disable_copy_original: false

redaction:
  default_action: mask        # mask | placeholder
  placeholder_template: "⟦REDACTED:{type}⟧"
  include_event_id: false
  rolling_window_bytes: 32768
  status_line:
    enabled: true
    rate_limit_ms: 2000

masking:
  style: glow
  block_char: "█"
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

allowlist:
  enabled: false
  commands: []               # optional list of command names or globs

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
      pattern: "(?i)\bPRIVATE_KEY\s*=\s*([^\s]+)"
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

### 10.3 Config resolution
1) `--config <path>` if provided
2) `~/.config/secretty/config.yaml`
3) if missing: run with embedded defaults and print a hint to run `secretty init`

---

## 11. Onboarding (`secretty init`)

### 11.1 Wizard steps (MVP)
Implemented using `huh?` prompts (terminal forms). citeturn0search1turn0search21  

1) Detect environment:
   - shell (`$SHELL`), TERM, tmux, terminal size
2) Choose mode:
   - Strict recording (default) / Demo / Warn-only
3) Enable Web3 ruleset (default ON)
4) Self-test:
   - prints sample `0x` + 64 hex and shows redaction
5) Create alias suggestion:
   - `alias safe=secretty`
6) Configure override:
   - enable copy-without-render (default ON), TTL
7) Optional allowlist:
   - select commands that bypass redaction

### 11.2 Self-test content rules
- Self-test strings are synthetic; do not use real keys.
- Assert:
  - a sample pk is redacted in output
  - typed detector is active
  - status line works (if enabled)

---

## 12. Implementation architecture (Go)

### 12.1 Modules / packages
```
cmd/secretty/            # Cobra or stdlib flag parsing
internal/config/         # YAML load/validate/defaults
internal/ptywrap/        # PTY spawn, resize, IO proxy
internal/ansi/           # streaming ANSI tokenizer
internal/detect/         # typed detectors + regex rules
internal/redact/         # replacement engine, mask strategies
internal/cache/          # in-memory secret cache + TTL
internal/clipboard/      # pbcopy integration
internal/ui/             # status line rendering (minimal)
internal/debug/          # sanitized logging
```

### 12.2 Process model
- Parent: `secretty`
- Child: shell/command executed under PTY.

Use `creack/pty.Start(cmd)` to assign a tty for stdin/stdout/stderr and to create a new session with a controlling terminal. citeturn0search0

### 12.3 Concurrency model
- Goroutine A: `stdin → ptyMaster`
- Goroutine B: `ptyMaster → redactionPipeline → stdout`
- Goroutine C: SIGWINCH handler → resize PTY (`pty.Getsize`, `pty.Setsize` / `InheritSize` where appropriate). citeturn0search0  
- Goroutine D (optional): TTL sweeper for secret cache

Use `context.Context` for cancellation, and ensure all goroutines exit on child exit.

### 12.4 Redaction pipeline (detailed)
```
PTY bytes
  → ANSI tokenizer (ESC/TEXT segments)
    → Rolling window assembler (TEXT only)
      → Candidate finder
        - regex matches
        - typed detector matches
      → Conflict resolver (overlaps, precedence)
      → Apply replacements (mask/placeholder)
    → Recombine ESC + redacted TEXT
  → stdout writer (flush)
```
**Key invariant:** do not allow replacements to alter `ESC` segments.

### 12.5 Overlap/conflict rules
When multiple matches overlap:
1) Prefer higher severity.
2) Prefer typed detector over regex.
3) Prefer longer span.
4) Stable ordering: earliest start.

### 12.6 Secret cache design
- Map: `eventID → SecretRecord`
- `SecretRecord` contains:
  - `Type`
  - `Original []byte`
  - `CreatedAt`
  - `ExpiresAt`
- Enforce max capacity (e.g., 64 entries) with LRU eviction.

---

## 13. Typed schemas (Go)

### 13.1 Core enums
```go
type Mode string
const (
  ModeDemo   Mode = "demo"
  ModeStrict Mode = "strict"
  ModeWarn   Mode = "warn"
)

type Action string
const (
  ActionMask        Action = "mask"
  ActionPlaceholder Action = "placeholder"
  ActionWarn        Action = "warn" // future
)

type SecretType string
const (
  SecretEvmPrivateKey SecretType = "EVM_PK"
)
```

### 13.2 Config structs (canonical)
```go
type Config struct {
  Version int `yaml:"version"`

  Mode   Mode   `yaml:"mode"`
  Strict Strict `yaml:"strict"`

  Redaction Redaction `yaml:"redaction"`
  Masking   Masking   `yaml:"masking"`
  Overrides Overrides `yaml:"overrides"`

  Rulesets       Rulesets        `yaml:"rulesets"`
  Rules          []Rule          `yaml:"rules"`
  TypedDetectors []TypedDetector `yaml:"typed_detectors"`

  Debug Debug `yaml:"debug"`
}

type Debug struct {
  Enabled   bool `yaml:"enabled"`
  LogEvents bool `yaml:"log_events"`
}

type Strict struct {
  NoReveal            bool `yaml:"no_reveal"`
  LockUntilExit       bool `yaml:"lock_until_exit"`
  DisableCopyOriginal bool `yaml:"disable_copy_original"`
}

type Redaction struct {
  DefaultAction       Action `yaml:"default_action"`
  PlaceholderTemplate string `yaml:"placeholder_template"`
  IncludeEventID      bool   `yaml:"include_event_id"`
  RollingWindowBytes  int    `yaml:"rolling_window_bytes"`
  StatusLine          StatusLine `yaml:"status_line"`
}

type StatusLine struct {
  Enabled     bool `yaml:"enabled"`
  RateLimitMS int  `yaml:"rate_limit_ms"`
}

type Masking struct {
  BlockChar string `yaml:"block_char"`
  HexRandomSameLength struct {
    Uppercase bool `yaml:"uppercase"`
  } `yaml:"hex_random_same_length"`
  StableHashToken struct {
    Enabled bool `yaml:"enabled"`
    TagLen  int  `yaml:"tag_len"`
  } `yaml:"stable_hash_token"`
}

type Overrides struct {
  CopyWithoutRender CopyWithoutRender `yaml:"copy_without_render"`
}

type CopyWithoutRender struct {
  Enabled        bool   `yaml:"enabled"`
  TTLSeconds     int    `yaml:"ttl_seconds"`
  RequireConfirm bool   `yaml:"require_confirm"`
  Backend        string `yaml:"backend"` // "pbcopy"
}

type Rulesets struct {
  Web3 Web3Ruleset `yaml:"web3"`
}

type Web3Ruleset struct {
  Enabled        bool `yaml:"enabled"`
  AllowBare64Hex bool `yaml:"allow_bare_64hex"`
}
```

### 13.3 Rule / detector schemas
```go
type Rule struct {
  Name            string   `yaml:"name"`
  Enabled         bool     `yaml:"enabled"`
  Type            string   `yaml:"type"` // "regex" | "typed"
  Action          Action   `yaml:"action"`
  Severity        string   `yaml:"severity"` // "low|med|high"
  Regex           *RegexRule `yaml:"regex,omitempty"`
  ContextKeywords []string `yaml:"context_keywords,omitempty"`
}

type RegexRule struct {
  Pattern string `yaml:"pattern"`
  Group   int    `yaml:"group"`
}

type TypedDetector struct {
  Name            string   `yaml:"name"`
  Enabled         bool     `yaml:"enabled"`
  Kind            string   `yaml:"kind"` // "EVM_PRIVATE_KEY"
  Action          Action   `yaml:"action"`
  Severity        string   `yaml:"severity"`
  ContextKeywords []string `yaml:"context_keywords,omitempty"`
}
```

### 13.4 Redaction events (sanitized)
```go
type RedactionEvent struct {
  ID        int
  AtUnixMS  int64
  Type      SecretType
  RuleName  string
  Action    Action
  // No originals here:
  Tag       string // optional stable hash tag (if enabled)
  Preview   string // sanitized local context snippet with secret already redacted
}
```

---

## 14. Packaging and tooling

### 14.1 Homebrew (MVP)
- Provide a tap formula:
  - `brew install suryansh-23/tap/secretty`
- Binary installs to `/opt/homebrew/bin/secretty`

### 14.2 Build tooling
- Go modules (`go.mod`)
- `golangci-lint`
- `gofmt` enforced
- `make` targets:
  - `make build`
  - `make lint`
  - `make test` (minimal)
  - `make release`

---

## 15. Minimal testing (kept light but sufficient)

Even with low testing emphasis, these are mandatory to avoid demo disasters:

### 15.1 Smoke tests
- `secretty run -- printf "0x<64hex>"` → output redacted.
- `secretty run -- bash -lc 'echo $TERM'` → works and returns exit code.
- resize test: run `tput cols` before/after resizing terminal.

### 15.2 Golden fixtures (small)
- Sample “anvil-like” key output fixture → redaction correct.
- Fixture with ANSI colored output → ANSI preserved.

---

## 16. Explicit constraints / invariants (for the dev agent)
1) Never print or log original secret bytes (even in debug).
2) Never mutate ANSI escape sequences.
3) Redaction must handle chunk boundaries (streaming).
4) In strict mode, never render originals (and optionally disable copy-original).
5) Default config must work with zero setup.

---

## 17. Optional decisions (defaults chosen; confirm if you want changes)
These are “last knobs” that might benefit from your explicit choice:

1) **Default shell selection**
   - Default: use `$SHELL` if set, else `/bin/zsh`.
   - Options: always zsh / always bash / detect login shell only.

2) **Allow bare 64-hex private keys**
   - Default: **false** (reduces false positives).
   - Option: true (more coverage).

3) **Strict mode: allow copy-original**
   - Default: **allowed** (but no reveal-to-screen).
   - Option: disallow copy-original for maximum safety.

4) **Status line**
   - Default: enabled + rate-limited.
   - Option: inline-only (zero additional output).

5) **Stable hash token**
   - Default: off.
   - Option: on (better “repeatability” in demos).

---

## 18. References / prior art
- Warp Secret Redaction (regex-based) and privacy posture. citeturn0search2turn0search10turn0search6  
- 1Password `op run` masks stdout/stderr by default; `--no-masking` and `OP_RUN_NO_MASKING` control it. citeturn0search7turn0search3  
- Teller supports `run --redact` and `teller redact` for live redaction pipelines. citeturn1search5turn1search1  
- `mask` is a demo-focused CLI to hide sensitive info from stdout via piping. citeturn2search1  
- PTY primitives and sizing functions in `github.com/creack/pty`. citeturn0search0turn0search16  
- `huh?` for terminal wizard forms. citeturn0search1turn0search21  
- Bubble Tea + Lip Gloss ecosystem for future richer TUIs. citeturn0search5turn0search9  
