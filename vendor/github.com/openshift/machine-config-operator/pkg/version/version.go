package version

import (
	"fmt"
	"os"
)

const (
	releaseVersionEnv = "RELEASE_VERSION"
)

var (
	// ReleaseVersion is the version of the openshift release.
	// This will be injected by the payload build process.
	ReleaseVersion = "0.0.1-snapshot"

	// OperatorImage is the image pullspec for the current machine-config operator.
	// This will be injected by the payload build process.
	OperatorImage = "placeholder.url.oc.will.replace.this.org/placeholdernamespace:was-not-built-properly"

	// Raw is the string representation of the version. This will be replaced
	// with the calculated version at build time.
	Raw = "v0.0.0-was-not-built-properly"

	// Hash is the git hash we've built the MCO with
	Hash = "was-not-built-properly"

	// String is the human-friendly representation of the version.
	String = fmt.Sprintf("MachineConfigOperator %s", Raw)

	// FCOS is a setting to enable Fedora CoreOS-only modifications
	FCOS = false

	// SCOS is a setting to enable CentOS Stream CoreOS-only modifications
	SCOS = false
)

// IsFCOS returns true if Fedora CoreOS-only modifications are enabled
func IsFCOS() bool {
	return FCOS
}

// IsSCOS returns true if CentOS Stream CoreOS-only modifications are enabled
func IsSCOS() bool {
	return SCOS
}

func init() {
	// TODO: Remove the following env var override to deprecated RELEASE_VERSION.
	// This is only here for backwards compatibility with the old build process.
	// For now, it will prepopulate the ReleaseVersion with the value of the env var
	// prior to flag parsing. This will allow the flag to override the env var.
	// In the future, we should remove this and only use the flag.
	rv := os.Getenv(releaseVersionEnv)
	if rv != "" {
		ReleaseVersion = rv
	}
}
