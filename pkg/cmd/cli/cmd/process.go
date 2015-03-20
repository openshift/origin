package cmd

import (
	"fmt"
	"io"
	"strings"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/meta"
	cmdutil "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd/util"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/golang/glog"
	"github.com/spf13/cobra"

	"github.com/openshift/origin/pkg/cmd/cli/describe"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	"github.com/openshift/origin/pkg/template"
	"github.com/openshift/origin/pkg/template/api"
)

// injectUserVars injects user specified variables into the Template
func injectUserVars(cmd *cobra.Command, t *api.Template) {
	values := util.StringList{}
	values.Set(cmdutil.GetFlagString(cmd, "value"))
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
}

const processLongDesc = `Process template into a list of resources specified in filename or stdin

JSON and YAML formats are accepted.

Examples:

	# Convert template.json file into resource list
	$ %[1]s process -f template.json

	# Convert stored template into resource list
	$ %[1]s process foo

	# Convert template.json into resource list
	$ cat template.json | %[1]s process -f -
`

// NewCmdProcess returns a 'process' command
func NewCmdProcess(fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "process -f filename",
		Short: "Process template into list of resources",
		Long:  fmt.Sprintf(processLongDesc, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			storedTemplate := ""
			if len(args) > 0 {
				storedTemplate = args[0]
			}

			filename := cmdutil.GetFlagString(cmd, "filename")
			if len(storedTemplate) == 0 && len(filename) == 0 {
				usageError(cmd, "Must pass a filename or name of stored template")
			}

			namespace, err := f.DefaultNamespace()
			checkErr(err)

			mapper, typer := f.Object()

			client, _, err := f.Clients()
			checkErr(err)

			var (
				templateObj *api.Template
				mapping     *meta.RESTMapping
			)

			if len(storedTemplate) > 0 {
				templateObj, err = client.Templates(namespace).Get(storedTemplate)
				if err != nil {
					checkErr(fmt.Errorf("Error retrieving template \"%s/%s\", please confirm it exists", namespace, storedTemplate))
				}

				version, kind, err := mapper.VersionAndKindForResource("template")
				if mapping, err = mapper.RESTMapping(kind, version); err != nil {
					checkErr(err)
				}
			} else {
				schema, err := f.Validator()
				checkErr(err)
				cfg, err := f.ClientConfig()
				checkErr(err)
				var (
					ok   bool
					data []byte
				)
				mapping, _, _, data, err = cmdutil.ResourceFromFile(filename, typer, mapper, schema, cfg.Version)
				checkErr(err)
				obj, err := mapping.Codec.Decode(data)
				checkErr(err)
				templateObj, ok = obj.(*api.Template)
				if !ok {
					checkErr(fmt.Errorf("cannot convert input to Template"))
				}
			}

			if cmd.Flag("value").Changed {
				injectUserVars(cmd, templateObj)
			}

			printer, err := f.Factory.PrinterForMapping(cmd, mapping)
			checkErr(err)

			// If 'parameters' flag is set it does not do processing but only print
			// the template parameters to console for inspection.
			if cmdutil.GetFlagBool(cmd, "parameters") == true {
				err = describe.PrintTemplateParameters(templateObj.Parameters, out)
				checkErr(err)
				return
			}

			result, err := client.TemplateConfigs(namespace).Create(templateObj)
			checkErr(err)

			// We need to override the default output format to JSON so we can return
			// processed template. Users might still be able to change the output
			// format using the 'output' flag.
			if !cmd.Flag("output").Changed {
				cmd.Flags().Set("output", "json")
				printer, _ = f.PrinterForMapping(cmd, mapping)
			}
			printer.PrintObj(result, out)
		},
	}

	cmdutil.AddPrinterFlags(cmd)

	cmd.Flags().StringP("filename", "f", "", "Filename or URL to file to use to update the resource")
	cmd.Flags().StringP("value", "v", "", "Specify a list of key-value pairs (eg. -v FOO=BAR,BAR=FOO) to set/override parameter values")
	cmd.Flags().BoolP("parameters", "", false, "Do not process but only print available parameters")
	return cmd
}
