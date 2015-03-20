package cmd

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

const buildLogsLongDesc = `Retrieve logs from the containers where the build occured

NOTE: This command may be moved in the future.

Examples:

	# Stream logs from container to stdout
	$ %[1]s build-logs 566bed879d2d
`

func NewCmdBuildLogs(fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "build-logs <build>",
		Short: "Show container logs from the build container",
		Long:  fmt.Sprintf(buildLogsLongDesc, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) != 1 {
				usageError(cmd, "<build> is a required argument")
			}

			namespace, err := f.DefaultNamespace()
			checkErr(err)

			c, _, err := f.Clients()
			checkErr(err)

			readCloser, err := c.BuildLogs(namespace).Get(args[0]).Stream()
			checkErr(err)
			defer readCloser.Close()

			_, err = io.Copy(out, readCloser)
			checkErr(err)
		},
	}
	return cmd
}
