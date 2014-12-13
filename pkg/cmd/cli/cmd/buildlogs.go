package cmd

import (
	"io"

	kubecmd "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd"
	"github.com/spf13/cobra"
)

func NewCmdBuildLogs(f *kubecmd.Factory, out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "build-logs <build>",
		Short: "Show container logs from the build container",
		Long: `Retrieve logs from the containers where the build occured

NOTE: This command may be moved in the future.

Examples:
$ kubectl build-logs 566bed879d2d
<stream logs from container to stdout>`,
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) != 1 {
				usageError(cmd, "<build> is a required argument")
			}

			namespace := getOriginNamespace(cmd)

			mapping, err := f.Mapper.RESTMapping(kubecmd.GetFlagString(cmd, "api-version"), "BuildLog")
			checkErr(err)
			c, err := f.Client(cmd, mapping)
			checkErr(err)

			// TODO: This should be a method on the origin Client - BuildLogs(namespace).Redirect(args[0])
			request := c.Get().Namespace(namespace).Path("redirect").Path(mapping.Resource).Path(args[0])

			readCloser, err := request.Stream()
			checkErr(err)
			defer readCloser.Close()

			_, err = io.Copy(out, readCloser)
			checkErr(err)
		},
	}
	return cmd
}
