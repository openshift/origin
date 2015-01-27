package source

import (
	"regexp"
)

var (
	argumentGit         = regexp.MustCompile("^(http://|https://|git@|git://).*\\.git(?:#([a-zA-Z0-9]*))?$")
	argumentGitProtocol = regexp.MustCompile("^git@")
)

func IsRemoteRepository(s string) bool {
	return argumentGit.MatchString(s) || argumentGitProtocol.MatchString(s)
}
