package detect

import (
	"regexp"
	"sort"
	"strings"

	"github.com/suryansh-23/secretty/internal/config"
	"github.com/suryansh-23/secretty/internal/redact"
	"github.com/suryansh-23/secretty/internal/types"
)

const contextWindow = 64

type sourceKind int

const (
	sourceRegex sourceKind = iota
	sourceTyped
)

type candidate struct {
	match    redact.Match
	severity int
	source   sourceKind
	length   int
}

type compiledRule struct {
	rule     config.Rule
	re       *regexp.Regexp
	group    int
	severity int
}

type typedDetector struct {
	detector config.TypedDetector
	severity int
	keywords []string
}

// Engine detects secrets using regex rules and typed validators.
type Engine struct {
	regexRules []compiledRule
	typed      []typedDetector

	evmWithPrefix *regexp.Regexp
	evmBare       *regexp.Regexp

	allowBare64Hex bool
}

// NewEngine builds a detector engine from config.
func NewEngine(cfg config.Config) *Engine {
	engine := &Engine{
		evmWithPrefix:  regexp.MustCompile(`0x[0-9a-fA-F]{64}`),
		evmBare:        regexp.MustCompile(`\b[0-9a-fA-F]{64}\b`),
		allowBare64Hex: cfg.Rulesets.Web3.AllowBare64Hex,
	}

	for _, rule := range cfg.Rules {
		if !rule.Enabled {
			continue
		}
		if rule.Type != config.RuleTypeRegex {
			continue
		}
		if rule.Regex == nil || rule.Regex.Pattern == "" {
			continue
		}
		engine.regexRules = append(engine.regexRules, compiledRule{
			rule:     rule,
			re:       regexp.MustCompile(rule.Regex.Pattern),
			group:    rule.Regex.Group,
			severity: severityRank(rule.Severity),
		})
	}

	for _, det := range cfg.TypedDetectors {
		if !det.Enabled {
			continue
		}
		engine.typed = append(engine.typed, typedDetector{
			detector: det,
			severity: severityRank(det.Severity),
			keywords: lowerKeywords(det.ContextKeywords),
		})
	}

	return engine
}

// Find returns redaction matches within text.
func (e *Engine) Find(text []byte) []redact.Match {
	var candidates []candidate
	candidates = append(candidates, e.findRegexMatches(text)...)
	candidates = append(candidates, e.findTypedMatches(text)...)

	if len(candidates) == 0 {
		return nil
	}
	resolved := resolveOverlaps(candidates)
	matches := make([]redact.Match, 0, len(resolved))
	for _, cand := range resolved {
		matches = append(matches, cand.match)
	}
	return matches
}

func (e *Engine) findRegexMatches(text []byte) []candidate {
	if len(e.regexRules) == 0 {
		return nil
	}
	str := string(text)
	var out []candidate
	for _, rule := range e.regexRules {
		indices := rule.re.FindAllStringSubmatchIndex(str, -1)
		for _, idx := range indices {
			start, end := captureBounds(idx, rule.group)
			if start < 0 || end <= start {
				continue
			}
			out = append(out, candidate{
				match: redact.Match{
					Start:      start,
					End:        end,
					Action:     rule.rule.Action,
					SecretType: types.SecretEvmPrivateKey,
					RuleName:   rule.rule.Name,
				},
				severity: rule.severity,
				source:   sourceRegex,
				length:   end - start,
			})
		}
	}
	return out
}

func (e *Engine) findTypedMatches(text []byte) []candidate {
	if len(e.typed) == 0 {
		return nil
	}
	str := string(text)
	var out []candidate
	for _, det := range e.typed {
		if det.detector.Kind != "EVM_PRIVATE_KEY" {
			continue
		}
		for _, idx := range e.evmWithPrefix.FindAllStringIndex(str, -1) {
			out = append(out, e.buildTypedCandidate(text, idx[0], idx[1], det)...)
		}
		if e.allowBare64Hex {
			for _, idx := range e.evmBare.FindAllStringIndex(str, -1) {
				out = append(out, e.buildTypedCandidate(text, idx[0], idx[1], det)...)
			}
		}
	}
	return out
}

func (e *Engine) buildTypedCandidate(text []byte, start, end int, det typedDetector) []candidate {
	if start < 0 || end <= start || end > len(text) {
		return nil
	}
	matchBytes := text[start:end]
	score := 0
	if validateEvmPrivateKey(matchBytes, e.allowBare64Hex) {
		score += 2
	}
	if hasContextKeyword(text, start, end, det.keywords) {
		score++
	}
	if has0xPrefix(matchBytes) {
		score++
	}
	if score < 2 {
		return nil
	}
	return []candidate{{
		match: redact.Match{
			Start:      start,
			End:        end,
			Action:     det.detector.Action,
			SecretType: types.SecretEvmPrivateKey,
			RuleName:   det.detector.Name,
		},
		severity: det.severity,
		source:   sourceTyped,
		length:   end - start,
	}}
}

func validateEvmPrivateKey(token []byte, allowBare bool) bool {
	if len(token) >= 2 && token[0] == '0' && (token[1] == 'x' || token[1] == 'X') {
		return isHex(token[2:]) && len(token[2:]) == 64
	}
	if !allowBare {
		return false
	}
	return len(token) == 64 && isHex(token)
}

func isHex(token []byte) bool {
	for _, b := range token {
		switch {
		case b >= '0' && b <= '9':
		case b >= 'a' && b <= 'f':
		case b >= 'A' && b <= 'F':
		default:
			return false
		}
	}
	return true
}

func has0xPrefix(token []byte) bool {
	return len(token) >= 2 && token[0] == '0' && (token[1] == 'x' || token[1] == 'X')
}

func hasContextKeyword(text []byte, start, end int, keywords []string) bool {
	if len(keywords) == 0 {
		return false
	}
	windowStart := start - contextWindow
	if windowStart < 0 {
		windowStart = 0
	}
	windowEnd := end + contextWindow
	if windowEnd > len(text) {
		windowEnd = len(text)
	}
	chunk := strings.ToLower(string(text[windowStart:windowEnd]))
	for _, kw := range keywords {
		if strings.Contains(chunk, kw) {
			return true
		}
	}
	return false
}

func lowerKeywords(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := make([]string, 0, len(in))
	for _, kw := range in {
		if kw == "" {
			continue
		}
		out = append(out, strings.ToLower(kw))
	}
	return out
}

func captureBounds(submatches []int, group int) (int, int) {
	if group < 0 {
		return -1, -1
	}
	idx := group * 2
	if idx+1 >= len(submatches) {
		return -1, -1
	}
	return submatches[idx], submatches[idx+1]
}

func resolveOverlaps(candidates []candidate) []candidate {
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].match.Start == candidates[j].match.Start {
			return candidates[i].match.End < candidates[j].match.End
		}
		return candidates[i].match.Start < candidates[j].match.Start
	})
	var out []candidate
	for _, cand := range candidates {
		if len(out) == 0 {
			out = append(out, cand)
			continue
		}
		last := out[len(out)-1]
		if cand.match.Start >= last.match.End {
			out = append(out, cand)
			continue
		}
		if betterCandidate(cand, last) {
			out[len(out)-1] = cand
		}
	}
	return out
}

func betterCandidate(a, b candidate) bool {
	if a.severity != b.severity {
		return a.severity > b.severity
	}
	if a.source != b.source {
		return a.source == sourceTyped
	}
	if a.length != b.length {
		return a.length > b.length
	}
	return a.match.Start < b.match.Start
}

func severityRank(severity types.Severity) int {
	switch severity {
	case types.SeverityHigh:
		return 3
	case types.SeverityMed:
		return 2
	case types.SeverityLow:
		return 1
	default:
		return 0
	}
}
