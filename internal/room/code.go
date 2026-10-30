package room

import (
	"math/rand"
	"strings"
)

// No ambiguous characters: no 0 O 1 I L.
const codeAlphabet = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"

func newCode(rng *rand.Rand) string {
	b := make([]byte, 7)
	for i := range b {
		if i == 3 {
			b[i] = '-'
			continue
		}
		b[i] = codeAlphabet[rng.Intn(len(codeAlphabet))]
	}
	return string(b)
}

// NormalizeCode upper-cases and re-inserts the dash ("ash7q2" -> "ASH-7Q2"). It returns "" for invalid input.
func NormalizeCode(s string) string {
	s = strings.ToUpper(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, "-", "")
	if len(s) != 6 {
		return ""
	}
	for _, c := range s {
		if !strings.ContainsRune(codeAlphabet, c) {
			return ""
		}
	}
	return s[:3] + "-" + s[3:]
}

// SanitizeName lower-cases, keeps [a-z0-9_-], truncates to 12 and requires at least 2 characters.
func SanitizeName(s string) (string, bool) {
	var b []byte
	for _, c := range strings.ToLower(strings.TrimSpace(s)) {
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_' || c == '-' {
			b = append(b, byte(c))
		}
	}
	if len(b) > 12 {
		b = b[:12]
	}
	if len(b) < 2 {
		return "", false
	}
	return string(b), true
}

// SanitizeChat strips control characters and limits the length to 160 runes.
func SanitizeChat(s string) string {
	var out []rune
	for _, r := range strings.TrimSpace(s) {
		if r < 32 || r == 127 || (r >= 0x80 && r < 0xA0) {
			continue
		}
		out = append(out, r)
		if len(out) >= 160 {
			break
		}
	}
	return strings.TrimSpace(string(out))
}
