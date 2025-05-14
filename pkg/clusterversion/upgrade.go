package clusterversion

import (
	"strings"

	configv1 "github.com/openshift/api/config/v1"
	"golang.org/x/mod/semver"
)

// IsUpgradedFromMinorVersion returns true if the cluster has been upgraded from or through the given version.
// This will only check for X.Y version upgrades - it will ignore patch/z-stream versions.
// Returns false if the input version is not a semver.
func IsUpgradedFromMinorVersion(version string, cv *configv1.ClusterVersion) bool {
	fromMajorMinor := majorMinorVersion(version)
	if !semver.IsValid(fromMajorMinor) {
		return false
	}

	beforeOrAtVersionFound := false
	atOrLaterVersionFound := false

	// History is always ordered from most recent to oldest.
	for _, history := range cv.Status.History {
		historyMajorMinor := majorMinorVersion(history.Version)
		// Version in history can be empty or not a semver. Skip in this case.
		if !semver.IsValid(historyMajorMinor) {
			continue
		}
		if semver.Compare(historyMajorMinor, fromMajorMinor) >= 0 {
			atOrLaterVersionFound = true
		}
		if semver.Compare(historyMajorMinor, fromMajorMinor) <= 0 {
			beforeOrAtVersionFound = true
		}
	}
	return beforeOrAtVersionFound && atOrLaterVersionFound
}

func majorMinorVersion(version string) string {
	if !strings.HasPrefix(version, "v") {
		version = "v" + version
	}
	return semver.MajorMinor(version)
}
