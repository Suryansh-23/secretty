package types

// Mode indicates the operational mode for redaction behavior.
type Mode string

const (
	ModeDemo   Mode = "demo"
	ModeStrict Mode = "strict"
	ModeWarn   Mode = "warn"
)

// Action defines how a secret match is handled.
type Action string

const (
	ActionMask        Action = "mask"
	ActionPlaceholder Action = "placeholder"
	ActionWarn        Action = "warn"
)

// SecretType labels a detected secret.
type SecretType string

const (
	SecretEvmPrivateKey SecretType = "EVM_PK"
)

// Severity labels the sensitivity of a rule/detector.
type Severity string

const (
	SeverityLow  Severity = "low"
	SeverityMed  Severity = "med"
	SeverityHigh Severity = "high"
)
