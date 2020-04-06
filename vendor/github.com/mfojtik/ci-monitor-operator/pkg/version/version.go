package version

import (
	"k8s.io/component-base/metrics"
	"k8s.io/component-base/metrics/legacyregistry"

	"k8s.io/apimachinery/pkg/version"
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

// Get returns the overall codebase version. It's for detecting
// what code a binary was built from.
func Get() version.Info {
	return version.Info{
		Major:      majorFromGit,
		Minor:      minorFromGit,
		GitCommit:  commitFromGit,
		GitVersion: versionFromGit,
		BuildDate:  buildDate,
	}
}

func init() {
	buildInfo := metrics.NewGaugeVec(
		&metrics.GaugeOpts{
			Name: "openshift_cluster_kube_apiserver_operator_build_info",
			Help: "A metric with a constant '1' value labeled by major, minor, git commit & git version from which OpenShift Cluster Kube-API-Server Operator was built.",
		},
		[]string{"major", "minor", "gitCommit", "gitVersion"},
	)
	buildInfo.WithLabelValues(majorFromGit, minorFromGit, commitFromGit, versionFromGit).Set(1)

	legacyregistry.MustRegister(buildInfo)
}
