package cmd

import (
	"io"
	"os"
	"strings"

	kubecmd "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/golang/glog"
	"github.com/openshift/origin/pkg/cmd/client"
	"github.com/openshift/origin/pkg/template"
	"github.com/openshift/origin/pkg/template/api"
	"github.com/spf13/cobra"
)

func NewCmdProcess(f *Factory, out io.Writer) *cobra.Command {
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
			mapper, typer := f.Object(cmd)
			mappings, namespace, _, data := kubecmd.ResourceFromFile(cmd, filename, typer, mapper, schema)
			if len(namespace) == 0 {
				namespace = getOriginNamespace(cmd)
			} else {
				err := kubecmd.CompareNamespaceFromFile(cmd, namespace)
				checkErr(err)
			}

			mapping, err := mapper.RESTMapping("TemplateConfig", kubecmd.GetFlagString(cmd, "api-version"))
			checkErr(err)
			c, _, err := f.Clients(cmd)
			checkErr(err)

			// User can override Template parameters by using --value(-v) option with
			// list of key-value pairs.
			// TODO: This should be done on server-side to make other clients life
			//			 easier.
			if cmd.Flag("value").Changed {
				values := util.StringList{}
				values.Set(kubecmd.GetFlagString(cmd, "value"))
				templateObj, err := mappings.Codec.Decode(data)
				checkErr(err)
				t := templateObj.(*api.Template)
				for _, keypair := range values {
					p := strings.SplitN(keypair, "=", 2)
					if len(p) != 2 {
						glog.Errorf("Invalid parameter assignment '%s'", keypair)
						continue
					}
					if v := template.GetParameterByName(t, p[0]); v != nil {
						v.Value = p[1]
						v.Generate = ""
						template.AddParameter(t, *v)
					} else {
						glog.Errorf("Unknown parameter name '%s'", p[0])
					}
				}
				data, err = mapping.Codec.Encode(t)
				checkErr(err)
			}

			// Print template parameters will cause template stop processing.
			// Users can see what parameters can be overriden and will be set in the
			// template.
			if kubecmd.GetFlagBool(cmd, "parameters") {
				obj, err := mapping.Codec.Decode(data)
				checkErr(err)
				printer, err := f.Printer(cmd, mapping, kubecmd.GetFlagBool(cmd, "no-headers"))
				checkErr(err)
				if t, ok := obj.(*api.Template); ok {
					err = printer.PrintObj(t, out)
					checkErr(err)
				}
				return
			}

			request := c.Post().Namespace(namespace).Resource(mapping.Resource).Body(data)
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
	cmd.Flags().StringP("value", "v", "", "Specify a list of key-value pairs (eg. -v FOO=BAR,BAR=FOO) to set/override parameter values")
	cmd.Flags().BoolP("parameters", "", false, "Do not process but only print available parameters")
	cmd.Flags().Bool("no-headers", false, "When using the default output, don't print headers")
	return cmd
}
