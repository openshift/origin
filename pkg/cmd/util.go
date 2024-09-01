package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/openshift/origin/pkg/version"
)

// PrintVersion is used as a PersistentPreRun function to ensure we always print the version.
var PrintVersion = func(cmd *cobra.Command, args []string) {
	fmt.Fprintf(os.Stdout, "openshift-tests %s\n", version.Get().GitVersion)
}

// NoPrintVersion is used as an empty PersistentPreRun function so we don't print version info
// for some commands.
var NoPrintVersion = func(cmd *cobra.Command, args []string) {
}
