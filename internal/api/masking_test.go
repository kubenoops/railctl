package api

import (
	"strings"
	"testing"
)

// All values in this file are FAKE fixtures. Never paste real secrets here.

// fakePEM is a syntactically PEM-shaped, entirely fake multi-line value.
const fakePEM = "-----BEGIN RSA PRIVATE KEY-----\nFAKEFAKEFAKEFAKEFAKEFAKEFAKEFAKE\nNOTAREALKEYNOTAREALKEYNOTAREALKEY\n-----END RSA PRIVATE KEY-----"

// secretKeyShapes are the real-world name shapes that leaked through the old
// underscore-delimited regex (.NET-style Section__Key, camelCase, bare "Pass"),
// plus the SCREAMING_SNAKE shape that was already caught.
var secretKeyShapes = []string{
	"CLERK_SECRET_KEY",
	"Clerk__SecretKey",
	"GitHubApp__PrivateKeyPem",
	"SurrealDb__Pass",
	"Hangfire__DashboardPassword",
}

// caseVariants returns the key as written, lower-cased, upper-cased and in an
// alternating mixed case.
func caseVariants(key string) []string {
	var mixed strings.Builder
	for i, r := range key {
		if i%2 == 0 {
			mixed.WriteString(strings.ToLower(string(r)))
		} else {
			mixed.WriteString(strings.ToUpper(string(r)))
		}
	}
	return []string{key, strings.ToLower(key), strings.ToUpper(key), mixed.String()}
}

func TestIsSensitiveKey_AllShapesAndCases(t *testing.T) {
	for _, shape := range secretKeyShapes {
		for _, k := range caseVariants(shape) {
			if !IsSensitiveKey(k) {
				t.Errorf("IsSensitiveKey(%q) = false, want true", k)
			}
		}
	}
	more := []string{
		"apiKey", "ApiKey", "stripeApiKey", "JWT_SIGNING_KEY", "db.password",
		"my-secret", "SENTRY_DSN", "Auth__ClientSecret", "OAuthToken",
		"PrivateKey", "tls_pem", "ADMIN_PWD", "Gpg__Passphrase",
	}
	for _, k := range more {
		if !IsSensitiveKey(k) {
			t.Errorf("IsSensitiveKey(%q) = false, want true", k)
		}
	}
}

func TestIsSensitiveKey_NoFalsePositivesOnNeutralNames(t *testing.T) {
	// Name matching is defence-in-depth only; these are still masked by
	// ShouldMask unless safe-listed. This guards the word-boundary logic.
	for _, k := range []string{
		"PORT", "PATH", "AUTHOR", "MY_PATH_CONFIG", "HOSTNAME", "DATABASE_URL",
		"LOG_LEVEL", "Clerk__AuthorizedParties__0", "Cors__AllowedOrigins__0",
		"KEYBOARD_LAYOUT", "PASSAGE_ID", "MONKEY_COUNT",
	} {
		if IsSensitiveKey(k) {
			t.Errorf("IsSensitiveKey(%q) = true, want false", k)
		}
	}
}

func TestShouldMask_SecretShapesAlwaysMasked(t *testing.T) {
	for _, shape := range secretKeyShapes {
		for _, k := range caseVariants(shape) {
			if !ShouldMask(k, "fake-value-0000") {
				t.Errorf("ShouldMask(%q, single-line) = false, want true", k)
			}
			if got := MaskIfNeeded(k, "fake-value-0000"); strings.Contains(got, "fake-value-0000") {
				t.Errorf("MaskIfNeeded(%q) leaked the value: %q", k, got)
			}
		}
	}
}

func TestShouldMask_MaskByDefault(t *testing.T) {
	// Keys that look harmless but are not on the safe-list are masked.
	for _, k := range []string{"NORMAL_VAR", "DATABASE_URL", "Clerk__AuthorizedParties__0", "SurrealDb__User", "anything"} {
		if !ShouldMask(k, "fake-value") {
			t.Errorf("ShouldMask(%q) = false, want true (mask by default)", k)
		}
	}
}

func TestShouldMask_SafeListShownInAnyCase(t *testing.T) {
	for _, k := range []string{"PORT", "port", "Port", "NODE_ENV", "node_env", "HOSTNAME", "DOTNET_ENVIRONMENT", "RAILWAY_PUBLIC_DOMAIN"} {
		if ShouldMask(k, "3000") {
			t.Errorf("ShouldMask(%q, %q) = true, want false (safe-listed)", k, "3000")
		}
		if got := MaskIfNeeded(k, "3000"); got != "3000" {
			t.Errorf("MaskIfNeeded(%q) = %q, want %q", k, got, "3000")
		}
	}
}

func TestShouldMask_MultiLineAlwaysMasked(t *testing.T) {
	// Even a safe-listed key never prints a multi-line value.
	for _, k := range []string{"PORT", "HOSTNAME", "NORMAL_VAR", "GitHubApp__PrivateKeyPem", "githubapp__privatekeypem"} {
		if !ShouldMask(k, fakePEM) {
			t.Errorf("ShouldMask(%q, PEM) = false, want true", k)
		}
	}
	if !ShouldMask("PORT", "80\r\n") {
		t.Error("ShouldMask(PORT, CRLF value) = false, want true")
	}
}

func TestMaskValue_PEM(t *testing.T) {
	got := MaskIfNeeded("GitHubApp__PrivateKeyPem", fakePEM)
	if strings.ContainsAny(got, "\r\n") {
		t.Errorf("masked PEM spans lines: %q", got)
	}
	if strings.Contains(got, "FAKEFAKE") || strings.Contains(got, "BEGIN") {
		t.Errorf("masked PEM leaked content: %q", got)
	}
	if len(got) != 14 {
		t.Errorf("masked PEM length = %d, want 14", len(got))
	}
}

func TestMaskValue_LeadingNewlineStaysOneLine(t *testing.T) {
	got := MaskValue("\n" + fakePEM)
	if got != strings.Repeat("*", 14) {
		t.Errorf("MaskValue(leading newline) = %q, want 14 asterisks", got)
	}
}

func TestRedactResponseBody_VariablesNeverEchoed(t *testing.T) {
	body := []byte(`{"data":{"variables":{"Clerk__SecretKey":"fake-clerk-0000","PORT":"3000","GitHubApp__PrivateKeyPem":` +
		jsonString(fakePEM) + `},"environmentPatches":{"edges":[{"node":{"patch":{"services":{"svc":{"variables":{"SurrealDb__Pass":{"value":"fake-surreal-0000"}}}}}}}]},"project":{"name":"demo"}}}`)
	got := redactResponseBody(body)
	for _, leak := range []string{"fake-clerk-0000", "fake-surreal-0000", "FAKEFAKE", "3000"} {
		if strings.Contains(got, leak) {
			t.Errorf("debug body leaked %q:\n%s", leak, got)
		}
	}
	if !strings.Contains(got, `"demo"`) {
		t.Errorf("non-variable fields should still be shown:\n%s", got)
	}
	if got := redactResponseBody([]byte("not json fake-secret")); strings.Contains(got, "fake-secret") {
		t.Errorf("non-JSON body echoed: %q", got)
	}
}

func jsonString(s string) string {
	r := strings.NewReplacer("\\", "\\\\", "\"", "\\\"", "\n", "\\n", "\r", "\\r")
	return `"` + r.Replace(s) + `"`
}
