package config

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/suryansh-23/secretty/internal/types"
	"gopkg.in/yaml.v3"
)

const (
	DefaultConfigVersion       = 1
	defaultConfigRelPath       = "secretty/config.yaml"
	defaultPlaceholderTemplate = "\u27e6REDACTED:{type}\u27e7"
	defaultBlockChar           = "\u2588"
)

var ErrInvalidConfig = errors.New("invalid config")

// Config is the top-level configuration schema.
type Config struct {
	Version int `yaml:"version"`

	Mode   types.Mode `yaml:"mode"`
	Strict Strict     `yaml:"strict"`

	Redaction Redaction `yaml:"redaction"`
	Masking   Masking   `yaml:"masking"`
	Overrides Overrides `yaml:"overrides"`
	Allowlist Allowlist `yaml:"allowlist"`

	Rulesets       Rulesets        `yaml:"rulesets"`
	Rules          []Rule          `yaml:"rules"`
	TypedDetectors []TypedDetector `yaml:"typed_detectors"`

	Debug Debug `yaml:"debug"`
}

// Debug controls sanitized logging.
type Debug struct {
	Enabled   bool `yaml:"enabled"`
	LogEvents bool `yaml:"log_events"`
}

// Strict controls strict-mode behavior.
type Strict struct {
	NoReveal            bool `yaml:"no_reveal"`
	LockUntilExit       bool `yaml:"lock_until_exit"`
	DisableCopyOriginal bool `yaml:"disable_copy_original"`
}

// Redaction configures redaction behavior.
type Redaction struct {
	DefaultAction       types.Action `yaml:"default_action"`
	PlaceholderTemplate string       `yaml:"placeholder_template"`
	IncludeEventID      bool         `yaml:"include_event_id"`
	RollingWindowBytes  int          `yaml:"rolling_window_bytes"`
	StatusLine          StatusLine   `yaml:"status_line"`
}

// StatusLine controls optional UI hints.
type StatusLine struct {
	Enabled     bool `yaml:"enabled"`
	RateLimitMS int  `yaml:"rate_limit_ms"`
}

// Masking configures masking strategies.
type Masking struct {
	Style               types.MaskStyle `yaml:"style"`
	BlockChar           string          `yaml:"block_char"`
	HexRandomSameLength struct {
		Uppercase bool `yaml:"uppercase"`
	} `yaml:"hex_random_same_length"`
	StableHashToken struct {
		Enabled bool `yaml:"enabled"`
		TagLen  int  `yaml:"tag_len"`
	} `yaml:"stable_hash_token"`
	MorseMessage string `yaml:"morse_message"`
}

// Overrides configures opt-in behavior.
type Overrides struct {
	CopyWithoutRender CopyWithoutRender `yaml:"copy_without_render"`
}

// Allowlist configures redaction bypass for selected commands.
type Allowlist struct {
	Enabled  bool     `yaml:"enabled"`
	Commands []string `yaml:"commands,omitempty"`
}

// CopyWithoutRender configures clipboard behavior.
type CopyWithoutRender struct {
	Enabled        bool   `yaml:"enabled"`
	TTLSeconds     int    `yaml:"ttl_seconds"`
	RequireConfirm bool   `yaml:"require_confirm"`
	Backend        string `yaml:"backend"`
}

// Rulesets enables higher-level rulesets.
type Rulesets struct {
	Web3       Web3Ruleset    `yaml:"web3"`
	APIKeys    GenericRuleset `yaml:"api_keys"`
	AuthTokens GenericRuleset `yaml:"auth_tokens"`
	Cloud      GenericRuleset `yaml:"cloud"`
	Passwords  GenericRuleset `yaml:"passwords"`
}

// Web3Ruleset enables Web3-specific detection.
type Web3Ruleset struct {
	Enabled        bool `yaml:"enabled"`
	AllowBare64Hex bool `yaml:"allow_bare_64hex"`
}

// GenericRuleset toggles a ruleset group.
type GenericRuleset struct {
	Enabled bool `yaml:"enabled"`
}

// RuleType indicates how a rule is evaluated.
type RuleType string

const (
	RuleTypeRegex RuleType = "regex"
	RuleTypeTyped RuleType = "typed"
)

// Rule represents a detection rule.
type Rule struct {
	Name            string           `yaml:"name"`
	Enabled         bool             `yaml:"enabled"`
	Type            RuleType         `yaml:"type"`
	Action          types.Action     `yaml:"action"`
	Severity        types.Severity   `yaml:"severity"`
	SecretType      types.SecretType `yaml:"secret_type"`
	Ruleset         string           `yaml:"ruleset,omitempty"`
	Regex           *RegexRule       `yaml:"regex,omitempty"`
	ContextKeywords []string         `yaml:"context_keywords,omitempty"`
}

// RegexRule configures regex-based detection.
type RegexRule struct {
	Pattern string `yaml:"pattern"`
	Group   int    `yaml:"group"`
}

// TypedDetector configures typed detection.
type TypedDetector struct {
	Name            string           `yaml:"name"`
	Enabled         bool             `yaml:"enabled"`
	Kind            string           `yaml:"kind"`
	Action          types.Action     `yaml:"action"`
	Severity        types.Severity   `yaml:"severity"`
	SecretType      types.SecretType `yaml:"secret_type"`
	Ruleset         string           `yaml:"ruleset,omitempty"`
	ContextKeywords []string         `yaml:"context_keywords,omitempty"`
}

// DefaultConfig returns the canonical default configuration.
func DefaultConfig() Config {
	return Config{
		Version: DefaultConfigVersion,
		Mode:    types.ModeStrict,
		Strict: Strict{
			NoReveal:            true,
			LockUntilExit:       false,
			DisableCopyOriginal: false,
		},
		Redaction: Redaction{
			DefaultAction:       types.ActionMask,
			PlaceholderTemplate: defaultPlaceholderTemplate,
			IncludeEventID:      false,
			RollingWindowBytes:  32768,
			StatusLine: StatusLine{
				Enabled:     true,
				RateLimitMS: 2000,
			},
		},
		Masking: Masking{
			Style:     types.MaskStyleGlow,
			BlockChar: defaultBlockChar,
			HexRandomSameLength: struct {
				Uppercase bool `yaml:"uppercase"`
			}{
				Uppercase: false,
			},
			StableHashToken: struct {
				Enabled bool `yaml:"enabled"`
				TagLen  int  `yaml:"tag_len"`
			}{
				Enabled: false,
				TagLen:  8,
			},
			MorseMessage: "SECRETTY",
		},
		Overrides: Overrides{
			CopyWithoutRender: CopyWithoutRender{
				Enabled:        true,
				TTLSeconds:     30,
				RequireConfirm: true,
				Backend:        "auto",
			},
		},
		Allowlist: Allowlist{
			Enabled:  false,
			Commands: nil,
		},
		Rulesets: Rulesets{
			Web3: Web3Ruleset{
				Enabled:        true,
				AllowBare64Hex: false,
			},
			APIKeys: GenericRuleset{
				Enabled: false,
			},
			AuthTokens: GenericRuleset{
				Enabled: false,
			},
			Cloud: GenericRuleset{
				Enabled: false,
			},
			Passwords: GenericRuleset{
				Enabled: false,
			},
		},
		Rules: []Rule{
			{
				Name:       "env_private_key",
				Enabled:    true,
				Type:       RuleTypeRegex,
				Action:     types.ActionMask,
				Severity:   types.SeverityHigh,
				SecretType: types.SecretEvmPrivateKey,
				Ruleset:    "web3",
				Regex: &RegexRule{
					Pattern: "(?i)\\bPRIVATE_KEY\\s*=\\s*([^\\s]+)",
					Group:   1,
				},
				ContextKeywords: []string{"private_key", "secret", "sk", "--private-key"},
			},
			{
				Name:       "api_key_label",
				Enabled:    true,
				Type:       RuleTypeRegex,
				Action:     types.ActionMask,
				Severity:   types.SeverityHigh,
				SecretType: types.SecretAPIKey,
				Ruleset:    "api_keys",
				Regex: &RegexRule{
					Pattern: "(?i)\\b([A-Z0-9_]*API[_-]?KEY|x-api-key|client[_-]?secret|secret[_-]?key)\\b\\s*[:=]\\s*([A-Za-z0-9_\\-]{16,})",
					Group:   2,
				},
				ContextKeywords: []string{"api_key", "x-api-key", "client_secret", "secret_key"},
			},
			{
				Name:       "stripe_key",
				Enabled:    true,
				Type:       RuleTypeRegex,
				Action:     types.ActionMask,
				Severity:   types.SeverityHigh,
				SecretType: types.SecretAPIKey,
				Ruleset:    "api_keys",
				Regex: &RegexRule{
					Pattern: "\\b(sk_(live|test)_[0-9a-zA-Z]{16,})\\b",
					Group:   1,
				},
			},
			{
				Name:       "github_pat",
				Enabled:    true,
				Type:       RuleTypeRegex,
				Action:     types.ActionMask,
				Severity:   types.SeverityHigh,
				SecretType: types.SecretAPIKey,
				Ruleset:    "api_keys",
				Regex: &RegexRule{
					Pattern: "\\bghp_[A-Za-z0-9]{36}\\b",
					Group:   0,
				},
			},
			{
				Name:       "bearer_token",
				Enabled:    true,
				Type:       RuleTypeRegex,
				Action:     types.ActionMask,
				Severity:   types.SeverityHigh,
				SecretType: types.SecretAuthToken,
				Ruleset:    "auth_tokens",
				Regex: &RegexRule{
					Pattern: "(?i)\\bBearer\\s+([A-Za-z0-9\\-._~+/]{20,}={0,2})",
					Group:   1,
				},
			},
			{
				Name:       "auth_token_label",
				Enabled:    true,
				Type:       RuleTypeRegex,
				Action:     types.ActionMask,
				Severity:   types.SeverityHigh,
				SecretType: types.SecretAuthToken,
				Ruleset:    "auth_tokens",
				Regex: &RegexRule{
					Pattern: "(?i)\\b(access|refresh|auth)[_-]?token\\b\\s*[:=]\\s*([^\\s]+)",
					Group:   2,
				},
				ContextKeywords: []string{"token", "auth", "access", "refresh"},
			},
			{
				Name:       "jwt_token",
				Enabled:    true,
				Type:       RuleTypeRegex,
				Action:     types.ActionMask,
				Severity:   types.SeverityHigh,
				SecretType: types.SecretJWT,
				Ruleset:    "auth_tokens",
				Regex: &RegexRule{
					Pattern: "\\b(eyJ[A-Za-z0-9_-]+\\.[A-Za-z0-9_-]+\\.[A-Za-z0-9_-]+)\\b",
					Group:   1,
				},
			},
			{
				Name:       "aws_access_key_id",
				Enabled:    true,
				Type:       RuleTypeRegex,
				Action:     types.ActionMask,
				Severity:   types.SeverityHigh,
				SecretType: types.SecretCloudCred,
				Ruleset:    "cloud",
				Regex: &RegexRule{
					Pattern: "\\bAKIA[0-9A-Z]{16}\\b",
					Group:   0,
				},
			},
			{
				Name:       "aws_secret_access_key",
				Enabled:    true,
				Type:       RuleTypeRegex,
				Action:     types.ActionMask,
				Severity:   types.SeverityHigh,
				SecretType: types.SecretCloudCred,
				Ruleset:    "cloud",
				Regex: &RegexRule{
					Pattern: "(?i)\\baws_secret_access_key\\b\\s*[:=]\\s*([A-Za-z0-9/+=]{40})",
					Group:   1,
				},
				ContextKeywords: []string{"aws", "secret_access_key"},
			},
			{
				Name:       "password_label",
				Enabled:    true,
				Type:       RuleTypeRegex,
				Action:     types.ActionMask,
				Severity:   types.SeverityHigh,
				SecretType: types.SecretPassword,
				Ruleset:    "passwords",
				Regex: &RegexRule{
					Pattern: "(?i)\\b(password|passwd|pwd|passphrase)\\b\\s*[:=]\\s*([^\\s]+)",
					Group:   2,
				},
				ContextKeywords: []string{"password", "pwd", "passphrase"},
			},
		},
		TypedDetectors: []TypedDetector{
			{
				Name:            "evm_private_key",
				Enabled:         true,
				Kind:            "EVM_PRIVATE_KEY",
				Action:          types.ActionMask,
				Severity:        types.SeverityHigh,
				SecretType:      types.SecretEvmPrivateKey,
				Ruleset:         "web3",
				ContextKeywords: []string{"private_key", "--private-key", "secret", "sk="},
			},
		},
		Debug: Debug{
			Enabled:   false,
			LogEvents: false,
		},
	}
}

// RulesetEnabled reports whether a ruleset is active.
func RulesetEnabled(name string, sets Rulesets) bool {
	switch name {
	case "":
		return true
	case "web3":
		return sets.Web3.Enabled
	case "api_keys":
		return sets.APIKeys.Enabled
	case "auth_tokens":
		return sets.AuthTokens.Enabled
	case "cloud":
		return sets.Cloud.Enabled
	case "passwords":
		return sets.Passwords.Enabled
	default:
		return false
	}
}

// DefaultPath returns the default config path.
func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}
	if xdg := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME")); xdg != "" {
		return filepath.Join(xdg, defaultConfigRelPath), nil
	}
	return filepath.Join(home, ".config", defaultConfigRelPath), nil
}

// Parse parses YAML config content, applying defaults.
func Parse(data []byte) (Config, error) {
	cfg := DefaultConfig()
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// Load reads config from disk, applying defaults when missing.
// The boolean return indicates whether a config file was found.
func Load(pathOverride string) (Config, bool, error) {
	path := strings.TrimSpace(pathOverride)
	if path == "" {
		var err error
		path, err = DefaultPath()
		if err != nil {
			return Config{}, false, err
		}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			cfg := DefaultConfig()
			if err := cfg.Validate(); err != nil {
				return Config{}, false, err
			}
			return cfg, false, nil
		}
		return Config{}, false, fmt.Errorf("read config: %w", err)
	}
	cfg, err := Parse(data)
	if err != nil {
		return Config{}, true, err
	}
	return cfg, true, nil
}

// Validate enforces the supported configuration schema.
func (c Config) Validate() error {
	var errs []string
	if c.Version != DefaultConfigVersion {
		errs = append(errs, fmt.Sprintf("version must be %d", DefaultConfigVersion))
	}
	if !validMode(c.Mode) {
		errs = append(errs, fmt.Sprintf("mode must be one of: %s", strings.Join(validModes(), ", ")))
	}
	if !validAction(c.Redaction.DefaultAction) {
		errs = append(errs, "redaction.default_action must be mask or placeholder")
	}
	if c.Redaction.PlaceholderTemplate == "" {
		errs = append(errs, "redaction.placeholder_template is required")
	}
	if c.Redaction.RollingWindowBytes < 0 {
		errs = append(errs, "redaction.rolling_window_bytes must be >= 0")
	}
	if c.Redaction.StatusLine.RateLimitMS < 0 {
		errs = append(errs, "redaction.status_line.rate_limit_ms must be >= 0")
	}
	if c.Masking.BlockChar == "" {
		errs = append(errs, "masking.block_char is required")
	}
	if !validMaskStyle(c.Masking.Style) {
		errs = append(errs, "masking.style must be block|glow|morse")
	}
	if c.Masking.StableHashToken.TagLen < 0 {
		errs = append(errs, "masking.stable_hash_token.tag_len must be >= 0")
	}
	if c.Overrides.CopyWithoutRender.TTLSeconds < 0 {
		errs = append(errs, "overrides.copy_without_render.ttl_seconds must be >= 0")
	}
	if c.Overrides.CopyWithoutRender.Backend == "" {
		errs = append(errs, "overrides.copy_without_render.backend is required")
	} else if !validClipboardBackend(c.Overrides.CopyWithoutRender.Backend) {
		errs = append(errs, "overrides.copy_without_render.backend must be one of: auto, pbcopy, wl-copy, xclip, xsel, none")
	}
	for i, entry := range c.Allowlist.Commands {
		trimmed := strings.TrimSpace(entry)
		if trimmed == "" {
			errs = append(errs, fmt.Sprintf("allowlist.commands[%d] must not be empty", i))
			continue
		}
		if _, err := path.Match(trimmed, "dummy"); err != nil {
			errs = append(errs, fmt.Sprintf("allowlist.commands[%d] has invalid pattern: %v", i, err))
		}
	}
	for i, rule := range c.Rules {
		if rule.Name == "" {
			errs = append(errs, fmt.Sprintf("rules[%d].name is required", i))
		}
		if !validRuleType(rule.Type) {
			errs = append(errs, fmt.Sprintf("rules[%d].type must be regex or typed", i))
		}
		if !validAction(rule.Action) {
			errs = append(errs, fmt.Sprintf("rules[%d].action must be mask or placeholder", i))
		}
		if !validSeverity(rule.Severity) {
			errs = append(errs, fmt.Sprintf("rules[%d].severity must be low|med|high", i))
		}
		if rule.Type == RuleTypeRegex {
			if rule.Regex == nil {
				errs = append(errs, fmt.Sprintf("rules[%d].regex is required for regex rule", i))
			} else {
				if rule.Regex.Pattern == "" {
					errs = append(errs, fmt.Sprintf("rules[%d].regex.pattern is required", i))
				}
				if rule.Regex.Group < 0 {
					errs = append(errs, fmt.Sprintf("rules[%d].regex.group must be >= 0", i))
				}
			}
		}
		if rule.Ruleset != "" && !validRuleset(rule.Ruleset) {
			errs = append(errs, fmt.Sprintf("rules[%d].ruleset is not supported", i))
		}
	}
	for i, det := range c.TypedDetectors {
		if det.Name == "" {
			errs = append(errs, fmt.Sprintf("typed_detectors[%d].name is required", i))
		}
		if det.Kind == "" {
			errs = append(errs, fmt.Sprintf("typed_detectors[%d].kind is required", i))
		}
		if !validAction(det.Action) {
			errs = append(errs, fmt.Sprintf("typed_detectors[%d].action must be mask or placeholder", i))
		}
		if !validSeverity(det.Severity) {
			errs = append(errs, fmt.Sprintf("typed_detectors[%d].severity must be low|med|high", i))
		}
		if det.Ruleset != "" && !validRuleset(det.Ruleset) {
			errs = append(errs, fmt.Sprintf("typed_detectors[%d].ruleset is not supported", i))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("%w: %s", ErrInvalidConfig, strings.Join(errs, "; "))
	}
	return nil
}

func validMode(mode types.Mode) bool {
	switch mode {
	case types.ModeDemo, types.ModeStrict, types.ModeWarn:
		return true
	default:
		return false
	}
}

func validModes() []string {
	return []string{string(types.ModeDemo), string(types.ModeStrict), string(types.ModeWarn)}
}

func validAction(action types.Action) bool {
	switch action {
	case types.ActionMask, types.ActionPlaceholder:
		return true
	default:
		return false
	}
}

func validMaskStyle(style types.MaskStyle) bool {
	switch style {
	case types.MaskStyleBlock, types.MaskStyleGlow, types.MaskStyleMorse:
		return true
	default:
		return false
	}
}

func validSeverity(severity types.Severity) bool {
	switch severity {
	case types.SeverityLow, types.SeverityMed, types.SeverityHigh:
		return true
	default:
		return false
	}
}

func validRuleType(ruleType RuleType) bool {
	switch ruleType {
	case RuleTypeRegex, RuleTypeTyped:
		return true
	default:
		return false
	}
}

func validRuleset(name string) bool {
	switch name {
	case "web3", "api_keys", "auth_tokens", "cloud", "passwords":
		return true
	default:
		return false
	}
}

func validClipboardBackend(backend string) bool {
	switch strings.ToLower(strings.TrimSpace(backend)) {
	case "auto", "pbcopy", "wl-copy", "xclip", "xsel", "none":
		return true
	default:
		return false
	}
}
