package serviceaccount

import "github.com/openshift/origin/pkg/api/apihelpers"

const (
	// These constants are here to create a name that is short enough to survive chopping by generate name
	maxNameLength             = 63
	randomLength              = 5
	maxSecretPrefixNameLength = maxNameLength - randomLength
)

func GetDockercfgSecretNamePrefix(serviceAccountName string) string {
	return apihelpers.GetName(serviceAccountName, "dockercfg-", maxSecretPrefixNameLength)
}

// GetTokenSecretNamePrefix creates the prefix used for the generated SA token secret. This is compatible with kube up until
// long names, at which point we hash the SA name and leave the "token-" intact.  Upstream clips the value and generates a random
// string.
// TODO fix the upstream implementation to be more like this.
func GetTokenSecretNamePrefix(serviceAccountName string) string {
	return apihelpers.GetName(serviceAccountName, "token-", maxSecretPrefixNameLength)
}
