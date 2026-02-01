package redact

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash/fnv"
	"io"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/suryansh-23/secretty/internal/config"
	"github.com/suryansh-23/secretty/internal/types"
)

// Redactor applies redaction actions to matched spans.
type Redactor struct {
	cfg            config.Config
	rng            io.Reader
	salt           []byte
	lastGlowIndex  int
	lastGlowBand   int
	hasGlowHistory bool
}

// NewRedactor returns a redactor using config defaults.
func NewRedactor(cfg config.Config) *Redactor {
	r := &Redactor{cfg: cfg, rng: rand.Reader}
	if cfg.Masking.StableHashToken.Enabled {
		r.salt = make([]byte, 32)
		_, _ = io.ReadFull(r.rng, r.salt)
	}
	return r
}

// Apply replaces matches inside text and returns redacted output.
func (r *Redactor) Apply(text []byte, matches []Match) ([]byte, error) {
	if len(matches) == 0 {
		return text, nil
	}
	local := append([]Match(nil), matches...)
	sort.Slice(local, func(i, j int) bool { return local[i].Start < local[j].Start })

	var out bytes.Buffer
	cursor := 0
	for _, m := range local {
		if m.Start < cursor {
			continue
		}
		if m.Start < 0 || m.End > len(text) || m.End <= m.Start {
			continue
		}
		out.Write(text[cursor:m.Start])
		replacement := r.replacement(text[m.Start:m.End], m)
		out.Write(replacement)
		cursor = m.End
	}
	out.Write(text[cursor:])
	return out.Bytes(), nil
}

func (r *Redactor) replacement(original []byte, match Match) []byte {
	action := match.Action
	if action == "" {
		action = r.cfg.Redaction.DefaultAction
	}
	switch action {
	case types.ActionMask:
		return r.maskBytes(original, match)
	case types.ActionPlaceholder:
		return r.placeholder(match)
	default:
		return original
	}
}

func (r *Redactor) maskBytes(original []byte, match Match) []byte {
	if r.cfg.Masking.StableHashToken.Enabled {
		return r.stableHashToken(match)
	}
	style := r.cfg.Masking.Style
	if style == "" {
		style = types.MaskStyleBlock
	}
	switch style {
	case types.MaskStyleGlow:
		startIndex, bandSize := r.glowParams(original)
		return maskGlow(original, r.cfg.Masking.BlockChar, startIndex, bandSize)
	case types.MaskStyleMorse:
		return maskMorse(original, r.cfg.Masking.MorseMessage)
	default:
		if match.SecretType == types.SecretEvmPrivateKey || looksHex(original) {
			return r.hexRandomSameLength(original, r.cfg.Masking.HexRandomSameLength.Uppercase)
		}
		return maskBlock(original, r.cfg.Masking.BlockChar)
	}
}

func (r *Redactor) placeholder(match Match) []byte {
	template := r.cfg.Redaction.PlaceholderTemplate
	if template == "" {
		template = "\u27e6REDACTED:{type}\u27e7"
	}
	repl := strings.ReplaceAll(template, "{type}", string(match.SecretType))
	repl = strings.ReplaceAll(repl, "{id:02d}", fmt.Sprintf("%02d", match.ID))
	repl = strings.ReplaceAll(repl, "{id}", strconv.Itoa(match.ID))
	return []byte(repl)
}

func (r *Redactor) stableHashToken(match Match) []byte {
	if len(r.salt) == 0 {
		r.salt = make([]byte, 32)
		_, _ = io.ReadFull(r.rng, r.salt)
	}
	h := hmac.New(sha256.New, r.salt)
	_, _ = h.Write([]byte(match.RuleName))
	_, _ = h.Write([]byte("|"))
	_, _ = h.Write([]byte(match.SecretType))
	tag := hex.EncodeToString(h.Sum(nil))
	if r.cfg.Masking.StableHashToken.TagLen > 0 && r.cfg.Masking.StableHashToken.TagLen < len(tag) {
		tag = tag[:r.cfg.Masking.StableHashToken.TagLen]
	}
	return []byte(fmt.Sprintf("\u27e6MASK:%s:%s\u27e7", match.SecretType, tag))
}

func (r *Redactor) hexRandomSameLength(original []byte, uppercase bool) []byte {
	prefix := []byte{}
	body := original
	if len(original) >= 2 && original[0] == '0' && (original[1] == 'x' || original[1] == 'X') {
		prefix = original[:2]
		body = original[2:]
	}
	out := make([]byte, len(prefix)+len(body))
	copy(out, prefix)
	for i := 0; i < len(body); i++ {
		out[len(prefix)+i] = randomHexNibble(r.rng, uppercase)
	}
	return out
}

func maskBlock(original []byte, blockChar string) []byte {
	runes := utf8.RuneCount(original)
	if runes <= 0 {
		return nil
	}
	return []byte(strings.Repeat(blockChar, runes))
}

func looksHex(b []byte) bool {
	if len(b) >= 2 && b[0] == '0' && (b[1] == 'x' || b[1] == 'X') {
		b = b[2:]
	}
	if len(b) == 0 {
		return false
	}
	for _, c := range b {
		if !isHexDigit(c) {
			return false
		}
	}
	return true
}

func isHexDigit(b byte) bool {
	switch {
	case b >= '0' && b <= '9':
		return true
	case b >= 'a' && b <= 'f':
		return true
	case b >= 'A' && b <= 'F':
		return true
	default:
		return false
	}
}

func randomHexNibble(rng io.Reader, uppercase bool) byte {
	buf := []byte{0}
	_, _ = io.ReadFull(rng, buf)
	idx := int(buf[0]) % 16
	var digits string
	if uppercase {
		digits = "0123456789ABCDEF"
	} else {
		digits = "0123456789abcdef"
	}
	return digits[idx]
}

type rgb struct {
	r int
	g int
	b int
}

var glowPalette = []rgb{
	{r: 45, g: 212, b: 191},
	{r: 34, g: 211, b: 238},
	{r: 56, g: 189, b: 248},
	{r: 96, g: 165, b: 250},
	{r: 129, g: 140, b: 248},
	{r: 167, g: 139, b: 250},
	{r: 192, g: 132, b: 252},
	{r: 244, g: 114, b: 182},
	{r: 251, g: 113, b: 133},
}

func maskGlow(original []byte, blockChar string, startIndex int, bandSize int) []byte {
	runes := utf8.RuneCount(original)
	if runes <= 0 {
		return nil
	}
	if blockChar == "" {
		blockChar = "\u2588"
	}
	var out bytes.Buffer
	if startIndex < 0 {
		startIndex = 0
	}
	if bandSize <= 0 {
		bandSize = 1
	}
	for i := 0; i < runes; i++ {
		color := glowPalette[(startIndex+(i/bandSize))%len(glowPalette)]
		out.WriteString(fmt.Sprintf("\x1b[38;2;%d;%d;%dm", color.r, color.g, color.b))
		out.WriteString(blockChar)
	}
	out.WriteString("\x1b[0m")
	return out.Bytes()
}

func (r *Redactor) glowParams(original []byte) (int, int) {
	if len(glowPalette) == 0 {
		return 0, 1
	}
	hasher := fnv.New32a()
	_, _ = hasher.Write(original)
	sum := hasher.Sum32()
	idx := int(sum % uint32(len(glowPalette)))
	bandSize := int((sum>>8)%4) + 2
	if bandSize < 2 {
		bandSize = 2
	}
	if r.hasGlowHistory && idx == r.lastGlowIndex && bandSize == r.lastGlowBand && len(glowPalette) > 1 {
		idx = (idx + 1) % len(glowPalette)
	}
	r.lastGlowIndex = idx
	r.lastGlowBand = bandSize
	r.hasGlowHistory = true
	return idx, bandSize
}

var morseAlphabet = map[rune]string{
	'A': ".-",
	'B': "-...",
	'C': "-.-.",
	'D': "-..",
	'E': ".",
	'F': "..-.",
	'G': "--.",
	'H': "....",
	'I': "..",
	'J': ".---",
	'K': "-.-",
	'L': ".-..",
	'M': "--",
	'N': "-.",
	'O': "---",
	'P': ".--.",
	'Q': "--.-",
	'R': ".-.",
	'S': "...",
	'T': "-",
	'U': "..-",
	'V': "...-",
	'W': ".--",
	'X': "-..-",
	'Y': "-.--",
	'Z': "--..",
	'0': "-----",
	'1': ".----",
	'2': "..---",
	'3': "...--",
	'4': "....-",
	'5': ".....",
	'6': "-....",
	'7': "--...",
	'8': "---..",
	'9': "----.",
}

func maskMorse(original []byte, message string) []byte {
	runes := utf8.RuneCount(original)
	if runes <= 0 {
		return nil
	}
	pattern := strings.TrimSpace(morsePattern(message))
	if pattern == "" {
		pattern = "... --- ..."
	}
	out := pattern
	for len(out) < runes {
		out = out + " " + pattern
	}
	return []byte(out[:runes])
}

func morsePattern(message string) string {
	msg := strings.ToUpper(strings.TrimSpace(message))
	if msg == "" {
		msg = "SECRETTY"
	}
	var parts []string
	for _, r := range msg {
		if r == ' ' {
			parts = append(parts, "/")
			continue
		}
		if code, ok := morseAlphabet[r]; ok {
			parts = append(parts, code)
		}
	}
	return strings.Join(parts, " ")
}
