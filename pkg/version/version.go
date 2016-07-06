package version

import (
	"fmt"
	"regexp"
	"strings"

	etcdversion "github.com/coreos/etcd/version"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/spf13/cobra"

	kubeversion "k8s.io/kubernetes/pkg/version"

	"github.com/openshift/origin/pkg/cmd/util/tokencmd"
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
	// build date in ISO8601 format, output of $(date -u +'%Y-%m-%dT%H:%M:%SZ')
	buildDate string
)

// Info contains versioning information.
// TODO: Add []string of api versions supported? It's still unclear
// how we'll want to distribute that information.
type Info struct {
	Major      string `json:"major"`
	Minor      string `json:"minor"`
	GitCommit  string `json:"gitCommit"`
	GitVersion string `json:"gitVersion"`
	BuildDate  string `json:"buildDate"`
}

// Get returns the overall codebase version. It's for detecting
// what code a binary was built from.
func Get() Info {
	return Info{
		Major:      majorFromGit,
		Minor:      minorFromGit,
		GitCommit:  commitFromGit,
		GitVersion: versionFromGit,
		BuildDate:  buildDate,
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

var (
	reCommitSegment   = regexp.MustCompile(`\+[0-9a-f]{6,14}$`)
	reCommitIncrement = regexp.MustCompile(`^[0-9a-f]+$`)
)

// LastSemanticVersion attempts to return a semantic version from the GitVersion - which
// is either <semver>+<commit> or <semver> on release boundaries.
func (info Info) LastSemanticVersion() string {
	version := info.GitVersion
	parts := strings.Split(version, "-")
	// strip the modifier
	if len(parts) > 1 && parts[len(parts)-1] == "dirty" {
		parts = parts[:len(parts)-1]
	}
	// strip the Git commit
	if len(parts) > 1 && reCommitSegment.MatchString(parts[len(parts)-1]) {
		parts[len(parts)-1] = reCommitSegment.ReplaceAllString(parts[len(parts)-1], "")
		if len(parts[len(parts)-1]) == 0 {
			parts = parts[:len(parts)-1]
		}
		// strip a version increment, but only if we found the commit
		if len(parts) > 1 && reCommitIncrement.MatchString(parts[len(parts)-1]) {
			parts = parts[:len(parts)-1]
		}
	}

	return strings.Join(parts, "-")
}

type Options struct {
	PrintEtcdVersion    bool
	PrintClientFeatures bool
}

// NewVersionCommand creates a command for displaying the version of this binary
func NewVersionCommand(basename string, options Options) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Display version",
		Run: func(c *cobra.Command, args []string) {
			fmt.Printf("%s %v\n", basename, Get())
			fmt.Printf("kubernetes %v\n", kubeversion.Get())
			if options.PrintEtcdVersion {
				fmt.Printf("etcd %v\n", etcdversion.Version)
			}
			if options.PrintClientFeatures {
				features := []string{}
				if tokencmd.BasicEnabled() {
					features = append(features, "Basic-Auth")
				}
				if tokencmd.GSSAPIEnabled() {
					features = append(features, "GSSAPI")
					features = append(features, "Kerberos") // GSSAPI or SSPI
					features = append(features, "SPNEGO")   // GSSAPI or SSPI
				}
				fmt.Printf("features: %s\n", strings.Join(features, " "))
			}
		},
	}
}

func init() {
	buildInfo := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "openshift_build_info",
			Help: "A metric with a constant '1' value labeled by major, minor, git commit & git version from which OpenShift was built.",
		},
		[]string{"major", "minor", "gitCommit", "gitVersion"},
	)
	buildInfo.WithLabelValues(majorFromGit, minorFromGit, commitFromGit, versionFromGit).Set(1)

	prometheus.MustRegister(buildInfo)
}
