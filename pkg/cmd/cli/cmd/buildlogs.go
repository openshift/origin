package cmd

import (
	"io"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd/util"
	"github.com/spf13/cobra"

	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

func NewCmdBuildLogs(f *clientcmd.Factory, out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "build-logs <build>",
		Short: "Show container logs from the build container",
		Long: `Retrieve logs from the containers where the build occured

NOTE: This command may be moved in the future.

Examples:

	# Stream logs from container to stdout
	$ osc build-logs 566bed879d2d
`,
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) != 1 {
				usageError(cmd, "<build> is a required argument")
			}

			namespace, err := f.DefaultNamespace(cmd)
			checkErr(err)

			mapper, _ := f.Object(cmd)
			mapping, err := mapper.RESTMapping("BuildLog", util.GetFlagString(cmd, "api-version"))
			checkErr(err)
			c, _, err := f.Clients(cmd)
			checkErr(err)

			// TODO: This should be a method on the origin Client - BuildLogs(namespace).Redirect(args[0])
			request := c.Get().Namespace(namespace).Prefix("redirect").Resource(mapping.Resource).Name(args[0])

			readCloser, err := request.Stream()
			checkErr(err)
			defer readCloser.Close()

			_, err = io.Copy(out, readCloser)
			checkErr(err)
		},
	}
	return cmd
}
