package util

import (
	"bufio"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"
)

// case insensitively match all key=value variables containing the word "proxy"
var proxyRegex = regexp.MustCompile("(?i).*proxy.*")

// ReadEnvironmentFile reads the content for a file that contains a list of
// environment variables and values. The key-pairs are separated by a new line
// character. The file can also have comments (both '#' and '//' are supported).
func ReadEnvironmentFile(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	result := map[string]string{}

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		s := strings.TrimSpace(scanner.Text())
		// Allow for comments in environment file
		if strings.HasPrefix(s, "#") || strings.HasPrefix(s, "//") {
			continue
		}
		parts := strings.SplitN(s, "=", 2)
		if len(parts) != 2 {
			continue
		}
		result[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
	}

	return result, scanner.Err()
}

// SafeForLoggingEnv attempts to strip sensitive information from proxy
// environment variable strings in key=value form.
func SafeForLoggingEnv(env []string) []string {
	newEnv := make([]string, len(env))
	copy(newEnv, env)
	for i, entry := range newEnv {
		parts := strings.SplitN(entry, "=", 2)
		if !proxyRegex.MatchString(parts[0]) {
			continue
		}
		newVal, _ := SafeForLoggingURL(parts[1])
		newEnv[i] = fmt.Sprintf("%s=%s", parts[0], newVal)
	}
	return newEnv
}

// SafeForLoggingURL removes the user:password section of
// a url if present.  If not present or the value is unparseable,
// the value is returned unchanged.
func SafeForLoggingURL(input string) (string, error) {
	u, err := url.Parse(input)
	if err != nil {
		return input, err
	}
	if u.User == nil {
		return input, nil
	}
	if _, passwordSet := u.User.Password(); passwordSet {
		// wipe out the user info from the url.
		u.User = url.User("redacted")
	}
	return u.String(), nil
}
