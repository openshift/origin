package version

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"k8s.io/apimachinery/pkg/version"
)

// NewCmdVersion provides a shim around version for
// non-client packages that require version information
func NewCmdVersion(fullName string, versionInfo version.Info, out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Display version",
		Long:  "Display version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintf(out, "%s %v\n", fullName, versionInfo)
		},
	}

	return cmd
}
