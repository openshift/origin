package version

import (
	"fmt"

	"github.com/spf13/cobra"
	kubeversion "k8s.io/kubernetes/pkg/version"
)

var (
	// commitFromGit is a constant representing the source version that
	// generated this build. It should be set during build via -ldflags.
	commitFromGit string
	// versionFromGit is a constant representing the version tag that
	// generated this build. It should be set during build via -ldflags.
	versionFromGit string
	// major version
	majorFromGit string
	// minor version
	minorFromGit string
)

// Info contains versioning information.
// TODO: Add []string of api versions supported? It's still unclear
// how we'll want to distribute that information.
type Info struct {
	Major      string `json:"major"`
	Minor      string `json:"minor"`
	GitCommit  string `json:"gitCommit"`
	GitVersion string `json:"gitVersion"`
}

// Get returns the overall codebase version. It's for detecting
// what code a binary was built from.
func Get() Info {
	return Info{
		Major:      majorFromGit,
		Minor:      minorFromGit,
		GitCommit:  commitFromGit,
		GitVersion: versionFromGit,
	}
}

// String returns info as a human-friendly version string.
func (info Info) String() string {
	version := info.GitVersion
	if version == "" {
		version = "unknown"
	}
	return version
}

// NewVersionCommand creates a command for displaying the version of this binary
func NewVersionCommand(basename string) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Display version",
		Run: func(c *cobra.Command, args []string) {
			fmt.Printf("%s %v\n", basename, Get())
			fmt.Printf("kubernetes %v\n", kubeversion.Get())
		},
	}
}
