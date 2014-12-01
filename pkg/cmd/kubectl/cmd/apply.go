package cmd

import (
	"io"

	kubecmd "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd"
	"github.com/golang/glog"
	"github.com/openshift/origin/pkg/config"
	"github.com/spf13/cobra"
)

func (f *OriginFactory) NewCmdApply(out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "apply -f filename",
		Short: "Perform bulk create operation on set of resources",
		Long: `Create all resources contained in JSON file specified in filename or stdin

NOTE: This command will be obsoleted and it is just temporary.

JSON and YAML formats are accepted.

Examples:
  $ kubectl apply -f config.json
  <creates all resources listed in config.json>

  $ cat config.json | kubectl apply -f -
  <creates all resources listed in config.json>`,
		Run: func(cmd *cobra.Command, args []string) {
			filename := kubecmd.GetFlagString(cmd, "filename")
			if len(filename) == 0 {
				usageError(cmd, "Must pass a filename to update")
			}

			_, namespace, _, data := kubecmd.ResourceFromFile(filename, f.Typer, f.Mapper)

			if len(namespace) == 0 {
				namespace = getOriginNamespace(cmd)
			} else {
				err := kubecmd.CompareNamespaceFromFile(cmd, namespace)
				checkErr(err)
			}

			result, err := config.Apply(namespace, data, f.RESTHelper(cmd))
			checkErr(err)

			for _, itemResult := range result {
				if len(itemResult.Errors) == 0 {
					glog.Infof(itemResult.Message)
					continue
				}
				for _, itemError := range itemResult.Errors {
					glog.Errorf("%v", itemError)
				}
			}
		},
	}
	cmd.Flags().StringP("filename", "f", "", "Filename or URL to file to use to update the resource")
	return cmd
}
