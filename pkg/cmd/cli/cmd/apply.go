package cmd

import (
	"io"

	kmeta "github.com/GoogleCloudPlatform/kubernetes/pkg/api/meta"
	kubecmd "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/resource"
	"github.com/golang/glog"
	"github.com/spf13/cobra"

	"github.com/openshift/origin/pkg/api/latest"
	"github.com/openshift/origin/pkg/config"
)

func NewCmdApply(f *Factory, out io.Writer) *cobra.Command {
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

			schema, err := f.Validator(cmd)
			checkErr(err)
			mapper, typer := f.Object(cmd)
			_, namespace, _, data := kubecmd.ResourceFromFile(cmd, filename, typer, mapper, schema)

			if len(namespace) == 0 {
				namespace = getOriginNamespace(cmd)
			} else {
				err := kubecmd.CompareNamespaceFromFile(cmd, namespace)
				checkErr(err)
			}

			result, err := config.Apply(namespace, data, func(m *kmeta.RESTMapping) (*resource.Helper, error) {
				client, kclient, err := f.Clients(cmd)
				if err != nil {
					return nil, err
				}
				if latest.OriginKind(m.Kind, m.APIVersion) {
					return resource.NewHelper(client, m), nil
				} else {
					return resource.NewHelper(kclient, m), nil
				}
			})
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
