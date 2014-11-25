package cmd

import (
	"io"
	"os"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kubecmd "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd"
	"github.com/golang/glog"
	"github.com/openshift/origin/pkg/cmd/client"
	"github.com/spf13/cobra"
)

func (f *OriginFactory) NewCmdProcess(out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "process -f filename",
		Short: "Process template into config",
		Long: `Process template into a config specified in filename or stdin

JSON and YAML formats are accepted.

Examples:
  $ kubectl process -f template.json
  <convert template.json into Config>

  $ cat template.json | kubectl process -f -
  <convert template.json into Config>`,
		Run: func(cmd *cobra.Command, args []string) {
			filename := kubecmd.GetFlagString(cmd, "filename")
			if len(filename) == 0 {
				usageError(cmd, "Must pass a filename to update")
			}

			data, err := kubecmd.ReadConfigData(filename)
			checkErr(err)

			// TODO: Wouldn't be necessary, in upstream it is builtin.
			namespace := api.NamespaceDefault
			if ns := kubecmd.GetFlagString(cmd, "namespace"); len(ns) > 0 {
				namespace = ns
			}

			c, err := f.OriginClientFunc(cmd, nil)
			checkErr(err)

			request := c.Post().Namespace(namespace).Path("/templateConfigs").Body(data)
			result := request.Do()
			body, err := result.Raw()
			checkErr(err)

			printer := client.JSONPrinter{}
			if err := printer.Print(body, os.Stdout); err != nil {
				glog.Fatalf("unable to pretty print config JSON: %v [%s]", err, string(body))
			}

		},
	}
	cmd.Flags().StringP("filename", "f", "", "Filename or URL to file to use to update the resource")
	return cmd
}
