package util

// Includes determines if the given string is in the provided slice of strings.
func Includes(arr []string, str string) bool {
	for _, s := range arr {
		if s == str {
			return true
		}
	}
	return false
}

// FirstNonEmpty returns the first non-empty string in the provided list of strings.
func FirstNonEmpty(args ...string) string {
	for _, value := range args {
		if len(value) > 0 {
			return value
		}
	}
	return ""
}
