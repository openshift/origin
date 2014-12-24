package cmd

import (
	"io"
	"os"

	kubecmd "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd"
	"github.com/golang/glog"
	"github.com/openshift/origin/pkg/cmd/client"
	"github.com/spf13/cobra"
)

func NewCmdProcess(f *kubecmd.Factory, out io.Writer) *cobra.Command {
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

			schema, err := f.Validator(cmd)
			checkErr(err)
			_, namespace, _, data := kubecmd.ResourceFromFile(filename, f.Typer, f.Mapper, schema)
			if len(namespace) == 0 {
				namespace = getOriginNamespace(cmd)
			} else {
				err := kubecmd.CompareNamespaceFromFile(cmd, namespace)
				checkErr(err)
			}

			mapping, err := f.Mapper.RESTMapping(kubecmd.GetFlagString(cmd, "api-version"), "TemplateConfig")
			checkErr(err)
			c, err := f.Client(cmd, mapping)
			checkErr(err)

			request := c.Post().Namespace(namespace).Path(mapping.Resource).Body(data)
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
