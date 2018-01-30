package version

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

// VersionInfo provides semantic version information
// in a human-friendly format
// TODO: may be expanded for various short and formatting options if necessary.
type VersionInfo interface {
	String() string
}

// NewCmdVersion provides a shim around version for
// non-client packages that require version information
func NewCmdVersion(fullName string, versionInfo VersionInfo, out io.Writer) *cobra.Command {
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
