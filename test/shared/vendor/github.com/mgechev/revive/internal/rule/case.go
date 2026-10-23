package rule

import "strings"

// IsUpperCaseConst checks if a string is in constant name format like `SOME_CONST`, `SOME_CONST_2`,
// `X123_3`, `_SOME_PRIVATE_CONST`.
// See #851, #865.
func IsUpperCaseConst(s string) bool {
	if s == "" {
		return false
	}
	r := []rune(s)
	c := r[0]
	if len(r) == 1 {
		return isUpper(c)
	}
	if c != '_' && !isUpper(c) { // Must start with an uppercase letter or underscore
		return false
	}
	for i, c := range r {
		switch {
		case isUpperOrDigit(c):
			continue
		case c == '_':
			// Underscore must be followed by at least one uppercase letter or digit
			if i+1 >= len(s) || !isUpperOrDigit(r[i+1]) {
				return false
			}

		default:
			return false
		}
	}
	return true
}

// HasUpperCaseLetter checks if a string contains at least one upper case letter.
func HasUpperCaseLetter(s string) bool {
	for _, r := range s {
		if isUpper(r) {
			return true
		}
	}
	return false
}

// IsUpperUnderscore detects variable that are made from upper case letters, underscore, or digits.
//
// Short variable names are considered OK.
func IsUpperUnderscore(s string) bool {
	if !strings.Contains(s, "_") {
		return false
	}
	if len(s) <= 5 {
		// avoid false positives
		return false
	}
	for _, r := range s {
		if r == '_' || isUpperOrDigit(r) {
			continue
		}
		return false
	}
	return true
}

// isUpperOrDigit checks if a rune is an uppercase letter or digit.
func isUpperOrDigit(r rune) bool {
	return isUpper(r) || isDigit(r)
}

// isDigit checks if rune is a simple digit.
//
// We don't use [unicode.IsDigit] as it returns true for a large variety of digits that are not 0-9.
func isDigit(r rune) bool {
	return r >= '0' && r <= '9'
}

// isUpper checks if rune is ASCII upper case letter
//
// We restrict to A-Z because [unicode.IsUpper] returns true for a large variety of letters.
func isUpper(r rune) bool {
	return r >= 'A' && r <= 'Z'
}
