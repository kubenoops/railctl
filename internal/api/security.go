package api

import (
	"regexp"
	"strings"
	"unicode"
)

// Variable-value masking policy: MASK BY DEFAULT.
//
// railctl never prints a variable value in clear unless its key is on the
// explicit safe-list below (well-known, never-secret names such as PORT).
// Everything else is masked, including any key whose name merely *looks*
// harmless. Name matching alone is not a safe gate: secret names come in every
// casing and separator style (CLERK_SECRET_KEY, Clerk__SecretKey,
// GitHubApp__PrivateKeyPem, SurrealDb__Pass, Hangfire__DashboardPassword), and
// a single missed pattern prints the secret. Sealed Railway variables are
// never readable and are shown as [SEALED] by the commands that list them.
//
// Two belt-and-braces rules apply on top of the safe-list:
//   - a key that looks sensitive (IsSensitiveKey) is masked even if listed
//     (which is why e.g. RAILWAY_PRIVATE_DOMAIN is not on it);
//   - a multi-line value (PEM keys, certificates) is always masked.
//
// `--show-values` on get/describe remains the explicit opt-out.

// safeKeys are variable names whose values are known not to be secrets and are
// useful to read in diff/get output. Matched case-insensitively on the exact
// name. Keep this list short and obviously non-secret; adding a name here is a
// reviewed decision, not a convenience.
var safeKeys = map[string]bool{
	"PORT":                   true,
	"HOST":                   true,
	"HOSTNAME":               true,
	"NODE_ENV":               true,
	"ENV":                    true,
	"ENVIRONMENT":            true,
	"LOG_LEVEL":              true,
	"TZ":                     true,
	"DOTNET_ENVIRONMENT":     true,
	"ASPNETCORE_ENVIRONMENT": true,
	"ASPNETCORE_HTTP_PORTS":  true,
	"ASPNETCORE_URLS":        true,
	"PGPORT":                 true,
	"PGDATA":                 true,
	// Railway-provided, non-secret service metadata.
	"RAILWAY_PUBLIC_DOMAIN":     true,
	"RAILWAY_STATIC_URL":        true,
	"RAILWAY_PROJECT_NAME":      true,
	"RAILWAY_PROJECT_ID":        true,
	"RAILWAY_ENVIRONMENT":       true,
	"RAILWAY_ENVIRONMENT_NAME":  true,
	"RAILWAY_ENVIRONMENT_ID":    true,
	"RAILWAY_SERVICE_NAME":      true,
	"RAILWAY_SERVICE_ID":        true,
	"RAILWAY_VOLUME_NAME":       true,
	"RAILWAY_VOLUME_MOUNT_PATH": true,
}

// IsSafeKey reports whether key is on the explicit safe-list (case-insensitive).
func IsSafeKey(key string) bool {
	return safeKeys[strings.ToUpper(key)]
}

// ShouldMask reports whether a variable's value must be masked in output.
// It returns false only for a safe-listed, non-sensitive key with a
// single-line value; every other value is masked.
func ShouldMask(key, value string) bool {
	if strings.ContainsAny(value, "\r\n") {
		return true
	}
	if IsSensitiveKey(key) {
		return true
	}
	return !IsSafeKey(key)
}

// MaskIfNeeded returns MaskValue(value) when ShouldMask(key, value), else value.
func MaskIfNeeded(key, value string) string {
	if ShouldMask(key, value) {
		return MaskValue(value)
	}
	return value
}

// sensitiveWords are matched against whole words of a key, after splitting on
// any non-alphanumeric separator ("_", "__", "-", ".") and on camelCase
// boundaries. Whole-word matching avoids false positives such as PATH (KEY)
// or AUTHOR (AUTH).
var sensitiveWords = map[string]bool{
	"KEY":         true,
	"KEYS":        true,
	"SECRET":      true,
	"SECRETS":     true,
	"PASS":        true,
	"PASSWORD":    true,
	"PASSWORDS":   true,
	"PASSWD":      true,
	"PWD":         true,
	"PASSPHRASE":  true,
	"TOKEN":       true,
	"TOKENS":      true,
	"CREDENTIAL":  true,
	"CREDENTIALS": true,
	"AUTH":        true,
	"PRIVATE":     true,
	"PEM":         true,
	"APIKEY":      true,
	"DSN":         true,
}

// sensitiveSubstrings are unambiguous fragments matched anywhere in the
// upper-cased key, catching run-together names such as "dashboardpassword"
// that word splitting cannot separate.
var sensitiveSubstrings = []string{
	"PASSWORD", "PASSWD", "PASSPHRASE", "SECRET", "TOKEN", "CREDENTIAL",
	"APIKEY", "PRIVATEKEY", "ACCESSKEY", "SIGNINGKEY",
}

var (
	camelLowerUpper = regexp.MustCompile(`([a-z0-9])([A-Z])`)
	camelAcronym    = regexp.MustCompile(`([A-Z]+)([A-Z][a-z])`)
	nonAlnum        = regexp.MustCompile(`[^A-Za-z0-9]+`)
)

// keyWords splits a variable key into upper-cased words on separators and
// camelCase boundaries: "GitHubApp__PrivateKeyPem" → [GIT HUB APP PRIVATE KEY PEM].
func keyWords(key string) []string {
	s := camelAcronym.ReplaceAllString(key, "${1}_${2}")
	s = camelLowerUpper.ReplaceAllString(s, "${1}_${2}")
	var words []string
	for _, w := range nonAlnum.Split(s, -1) {
		if w != "" {
			words = append(words, strings.ToUpper(w))
		}
	}
	return words
}

// IsSensitiveKey reports whether a variable key name looks like it holds a
// secret, in any casing or separator style (SCREAMING_SNAKE, .NET-style
// Section__Key, camelCase, lowercase). It is a defence-in-depth signal only:
// whether output is masked is decided by ShouldMask, which masks by default.
func IsSensitiveKey(key string) bool {
	upper := strings.ToUpper(key)
	for _, s := range sensitiveSubstrings {
		if strings.Contains(upper, s) {
			return true
		}
	}
	// Separator-only words catch arbitrary casing ("sUrReAlDb__PaSs") that the
	// camelCase split would cut mid-word; camelCase words catch "SecretKey".
	for _, w := range nonAlnum.Split(upper, -1) {
		if sensitiveWords[w] {
			return true
		}
	}
	for _, w := range keyWords(key) {
		if sensitiveWords[w] {
			return true
		}
	}
	return false
}

// MaskValue returns a fixed-length masked representation of a value.
// It always produces a 14-character output (first 2 runes + 12 asterisks) for
// non-empty values, regardless of the original value length. This prevents
// leaking value length or suffix. Uses rune slicing for UTF-8 safety. If either
// leading rune is whitespace or a control character (e.g. a value starting
// with a newline), the output is all asterisks so it stays on one line.
func MaskValue(value string) string {
	if len(value) == 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= 2 {
		return strings.Repeat("*", 14)
	}
	for _, r := range runes[:2] {
		if unicode.IsSpace(r) || unicode.IsControl(r) {
			return strings.Repeat("*", 14)
		}
	}
	return string(runes[:2]) + strings.Repeat("*", 12)
}
