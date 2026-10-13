package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	"github.com/openshift/origin/pkg/version"
)

// RequireClusterAccess can be used as a PreRunE to ensure there's a valid kubeconfig available. It emits
// a user-friendly error message, since the upstream kube one is confusing.  e2e.LoadConfig falls back to
// trying to find in-cluster service account creds, but this isn't even a way we support running origin. Give
// the user a nicer error telling them we expect to find a kubeconfig.
func RequireClusterAccess(_ *cobra.Command, _ []string) error {
	if _, err := e2e.LoadConfig(true); err != nil {
		return fmt.Errorf("failed to find cluster config: ensure KUBECONFIG is set")
	}

	return nil
}

// PrintVersion is used as a PersistentPreRun function to ensure we always print the version.
func PrintVersion(_ *cobra.Command, _ []string) {
	fmt.Fprintf(os.Stdout, "openshift-tests %s\n", version.Get().GitVersion)
}

// NoPrintVersion is used as an empty PersistentPreRun function so we don't print version info
// for some commands.
func NoPrintVersion(_ *cobra.Command, _ []string) {
}
