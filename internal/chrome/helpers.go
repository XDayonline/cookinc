package chrome

import "strings"

// domainMatches checks if cookieDomain matches any domain in the allowlist.
func domainMatches(cookieDomain string, allowlist []string) bool {
	cd := strings.ToLower(cookieDomain)
	for _, allowed := range allowlist {
		a := strings.ToLower(allowed)
		if cd == a || cd == "."+a || strings.HasSuffix(cd, "."+a) {
			return true
		}
	}
	return false
}

// sameSiteToInt converts a Chrome SameSite string to the protocol integer.
func sameSiteToInt(s string) int {
	switch strings.ToLower(s) {
	case "lax":
		return 1
	case "strict":
		return 2
	case "none":
		return 3
	default:
		return 0
	}
}
