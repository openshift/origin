package cmd

import (
	"errors"
	"io"
	"strings"

	cmdutil "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd/util"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/golang/glog"
	"github.com/spf13/cobra"

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

// NewCmdProcess returns a 'process' command
func NewCmdProcess(f *Factory, out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "process -f filename",
		Short: "Process template into list of resources",
		Long: `Process template into a list of resources specified in filename or stdin

JSON and YAML formats are accepted.

Examples:
  $ osc process -f template.json
  <convert template.json into resource list>

  $ cat template.json | osc process -f -
  <convert template.json into resource list>`,
		Run: func(cmd *cobra.Command, args []string) {
			filename := cmdutil.GetFlagString(cmd, "filename")
			if len(filename) == 0 {
				usageError(cmd, "Must pass a filename to update")
			}

			schema, err := f.Validator(cmd)
			checkErr(err)

			cfg, err := f.ClientConfig(cmd)
			checkErr(err)
			namespace, err := f.DefaultNamespace(cmd)
			checkErr(err)
			mapper, typer := f.Object(cmd)

			mapping, _, _, data := cmdutil.ResourceFromFile(filename, typer, mapper, schema, cfg.Version)
			obj, err := mapping.Codec.Decode(data)
			checkErr(err)

			templateObj, ok := obj.(*api.Template)
			if !ok {
				checkErr(errors.New("Unable to the convert input to the Template"))
			}

			client, _, err := f.Clients(cmd)
			checkErr(err)

			if cmd.Flag("value").Changed {
				injectUserVars(cmd, templateObj)
			}

			printer, err := f.Factory.PrinterForMapping(cmd, mapping)
			checkErr(err)

			// If 'parameters' flag is set it does not do processing but only print
			// the template parameters to console for inspection.
			if cmdutil.GetFlagBool(cmd, "parameters") == true {
				err = printer.PrintObj(templateObj, out)
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
