package api

import (
	"regexp"
	"strings"
)

// sensitivePattern is a single compiled regex that matches variable key names
// containing sensitive data. We use (^|_) and ($|_) as delimiters instead of \b
// because env var names use underscores as word separators and \b treats
// underscores as word characters. This prevents false positives like "PATH"
// matching KEY or "AUTHOR" matching AUTH.
//
// APIKEY is matched without delimiters since it's a compound word that appears
// as-is in key names.
var sensitivePattern = regexp.MustCompile(`(?i)((^|_)(KEY|SECRET|PASSW(OR)?D|TOKEN|CREDENTIALS?|AUTH|PRIVATE)($|_)|APIKEY)`)

// IsSensitiveKey returns true if the variable key matches a sensitive pattern.
// Uses underscore-delimited boundary matching to avoid false positives (e.g.,
// "PATH" won't match KEY, "AUTHOR" won't match AUTH).
func IsSensitiveKey(key string) bool {
	return sensitivePattern.MatchString(key)
}

// MaskValue returns a fixed-length masked representation of a value.
// It always produces a 14-character output (first 2 runes + 12 asterisks) for
// non-empty values, regardless of the original value length. This prevents
// leaking value length or suffix. Uses rune slicing for UTF-8 safety.
func MaskValue(value string) string {
	if len(value) == 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= 2 {
		return strings.Repeat("*", 14)
	}
	return string(runes[:2]) + strings.Repeat("*", 12)
}
