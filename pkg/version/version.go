package version

import (
	"fmt"
)

// commitFromGit is a constant representing the source version that
// generated this build. It should be set during build via -ldflags.
var commitFromGit string

// Info contains versioning information.
// TODO: Add []string of api versions supported? It's still unclear
// how we'll want to distribute that information.
type Info struct {
	Major     string `json:"major"`
	Minor     string `json:"minor"`
	GitCommit string `json:"gitCommit"`
}

// Get returns the overall codebase version. It's for detecting
// what code a binary was built from.
func Get() Info {
	return Info{
		Major:     "0",
		Minor:     "1",
		GitCommit: commitFromGit,
	}
}

// String returns info as a human-friendly version string.
func (info Info) String() string {
	commit := info.GitCommit
	if commit == "" {
		commit = "(unknown)"
	}
	return fmt.Sprintf("version %s.%s, build %s", info.Major, info.Minor, commit)
}
