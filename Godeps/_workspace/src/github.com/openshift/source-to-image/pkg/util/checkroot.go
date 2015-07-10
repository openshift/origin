package util

import (
	"regexp"
	"strconv"
	"strings"
)

// IsPotentialRootUser will return true if the passed in
// user is blank, non-numeric, OR it's numeric and == 0
func IsPotentialRootUser(user string) bool {
	uid, err := strconv.Atoi(user)
	return err != nil || uid == 0
}

var dockerLineDelim = regexp.MustCompile(`[\t\v\f\r ]+`)

// IncludesRootUserDirective takes a list of Dockerfile instructions
// and returns true if they include one with a "USER" directive
// and the specified user is a potential root user
func IncludesRootUserDirective(directives []string) bool {
	for _, line := range directives {
		parts := dockerLineDelim.Split(line, 2)
		if strings.ToLower(parts[0]) != "user" {
			continue
		}
		if IsPotentialRootUser(parts[1]) {
			return true
		}
	}
	return false
}
