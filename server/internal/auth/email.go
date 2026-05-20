package auth

import (
	"strings"

	"golang.org/x/net/idna"
	"golang.org/x/text/unicode/norm"
)

// CanonicalizeEmail normalizes an email address for storage and comparison.
//
// Steps:
//  1. Trim whitespace
//  2. NFKC normalization (Unicode equivalence — canonically equivalent
//     sequences map to the same string, e.g. ﬁ → fi, ℌ → H)
//  3. Domain part → Punycode (IDNA ASCII) for internationalized domains
//  4. Both parts lowercased (RFC 5321 local-part is technically
//     case-sensitive, but virtually all providers treat it case-insensitively)
//
// Examples:
//
//	" Test@Example.com "           → "test@example.com"
//	"Straße@ExAmPlE.cOm"           → "straße@example.com"
//	"test@münchen.de"              → "test@xn--mnchen-3ya.de"
//	"  USER@example.com  "         → "user@example.com"
//	"\uFEFFtest@example.com"       → "test@example.com"  (BOM stripped)
func CanonicalizeEmail(email string) string {
	email = strings.TrimSpace(email)
	// NFKC normalization: decomposes + recomposes canonically.
	// Handles ligatures, half-width/full-width forms, and other Unicode
	// equivalence issues. Also normalizes Turkish İ (U+0130) correctly.
	email = norm.NFKC.String(email)

	// Strip BOM (U+FEFF) which can appear in copy-pasted text.
	// NFKC preserves U+FEFF as a valid character, but it has no place in an email.
	email = strings.ReplaceAll(email, "\uFEFF", "")

	at := strings.LastIndex(email, "@")
	if at == -1 {
		// No @ — not a valid email, just lowercase and return
		return strings.ToLower(email)
	}

	local := email[:at]
	domain := email[at+1:]

	// Convert domain to Punycode (IDNA ASCII)
	// idna.ToASCII handles the full IDNA2008 algorithm:
	//   "münchen.de" → "xn--mnchen-3ya.de"
	//   "example.com" → "example.com" (no change)
	// On error (rare — malformed domain), fall through to lowercase.
	if asciiDomain, err := idna.ToASCII(domain); err == nil && asciiDomain != "" {
		domain = asciiDomain
	}
	domain = strings.ToLower(domain)

	// Lowercase local part. Go's strings.ToLower uses Unicode default
	// case folding (not locale-specific), so Turkish İ → i correctly.
	local = strings.ToLower(local)

	return local + "@" + domain
}
