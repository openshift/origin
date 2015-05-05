package cmd

import (
	"fmt"
	"io"
	"reflect"
	"strings"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/meta"
	ctl "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl"
	kcmdutil "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd/util"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/resource"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/golang/glog"
	"github.com/spf13/cobra"

	"github.com/openshift/origin/pkg/cmd/cli/describe"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	configapi "github.com/openshift/origin/pkg/config/api"
	"github.com/openshift/origin/pkg/template"
	"github.com/openshift/origin/pkg/template/api"
)

// injectUserVars injects user specified variables into the Template
func injectUserVars(cmd *cobra.Command, t *api.Template) {
	values := util.StringList{}
	values.Set(kcmdutil.GetFlagString(cmd, "value"))
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

const (
	process_long = `Process template into a list of resources specified in filename or stdin

JSON and YAML formats are accepted.`

	process_example = `  // Convert template.json file into resource list
  $ %[1]s process -f template.json

  // Process template while passing a user-defined label
  $ %[1]s process -f template.json -l name=mytemplate

  // Convert stored template into resource list
  $ %[1]s process foo

  // Convert template.json into resource list
  $ cat template.json | %[1]s process -f -`
)

// NewCmdProcess implements the OpenShift cli process command
func NewCmdProcess(fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "process (TEMPLATE | -f FILENAME) [-v=KEY=VALUE]",
		Short:   "Process template into list of resources",
		Long:    process_long,
		Example: fmt.Sprintf(process_example, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			err := RunProcess(f, out, cmd, args)
			kcmdutil.CheckErr(err)
		},
	}

	kcmdutil.AddPrinterFlags(cmd)

	cmd.Flags().StringP("filename", "f", "", "Filename or URL to file to use to update the resource")
	cmd.Flags().StringP("value", "v", "", "Specify a list of key-value pairs (eg. -v FOO=BAR,BAR=FOO) to set/override parameter values")
	cmd.Flags().BoolP("parameters", "", false, "Do not process but only print available parameters")
	cmd.Flags().StringP("labels", "l", "", "Label to set in all resources for this template")
	return cmd
}

// RunProject contains all the necessary functionality for the OpenShift cli process command
func RunProcess(f *clientcmd.Factory, out io.Writer, cmd *cobra.Command, args []string) error {
	storedTemplate := ""
	if len(args) > 0 {
		storedTemplate = args[0]
	}

	filename := kcmdutil.GetFlagString(cmd, "filename")
	if len(storedTemplate) == 0 && len(filename) == 0 {
		return kcmdutil.UsageError(cmd, "Must pass a filename or name of stored template")
	}

	namespace, err := f.DefaultNamespace()
	if err != nil {
		return err
	}

	mapper, typer := f.Object()

	client, _, err := f.Clients()
	if err != nil {
		return err
	}

	var (
		templateObj *api.Template
		mapping     *meta.RESTMapping
	)

	if len(storedTemplate) > 0 {
		templateObj, err = client.Templates(namespace).Get(storedTemplate)
		if err != nil {
			if errors.IsNotFound(err) {
				return fmt.Errorf("template %q could not be found", storedTemplate)
			}
			return err
		}

		version, kind, err := mapper.VersionAndKindForResource("template")
		if mapping, err = mapper.RESTMapping(kind, version); err != nil {
			if err != nil {
				return err
			}
		}
	} else {
		obj, err := resource.NewBuilder(mapper, typer, f.ClientMapperForCommand()).
			NamespaceParam(namespace).RequireNamespace().
			FilenameParam(filename).
			Do().
			Object()

		if err != nil {
			return err
		}

		var ok bool
		templateObj, ok = obj.(*api.Template)
		if !ok {
			return fmt.Errorf("cannot convert input to Template: ", reflect.TypeOf(obj))
		}

		version, kind, err := kapi.Scheme.ObjectVersionAndKind(templateObj)
		if err != nil {
			return err
		}
		if mapping, err = mapper.RESTMapping(kind, version); err != nil {
			if err != nil {
				return err
			}
		}
	}

	if cmd.Flag("value").Changed {
		injectUserVars(cmd, templateObj)
	}

	printer, err := f.Factory.PrinterForMapping(cmd, mapping)
	if err != nil {
		return err
	}

	// If 'parameters' flag is set it does not do processing but only print
	// the template parameters to console for inspection.
	if kcmdutil.GetFlagBool(cmd, "parameters") == true {
		err = describe.PrintTemplateParameters(templateObj.Parameters, out)
		if err != nil {
			return err
		}
		return nil
	}

	label := kcmdutil.GetFlagString(cmd, "labels")
	if len(label) != 0 {
		lbl := ctl.ParseLabels(label)
		for key, value := range lbl {
			templateObj.ObjectLabels[key] = value
		}
	}

	// TODO: use AsVersionedObjects to generate the runtime.Objects, because
	// some objects may not exist in the destination version but they should
	// still be transformed.
	obj, err := client.TemplateConfigs(namespace).Create(templateObj)
	if err != nil {
		return err
	}
	var result runtime.Object = obj
	// legacy support - when using older api versions, output a Config
	if kapi.PreV1Beta3(mapping.APIVersion) {
		result = &configapi.Config{
			ListMeta: kapi.ListMeta{},
			Items:    obj.Objects,
		}
	} else {
		result = &kapi.List{
			ListMeta: kapi.ListMeta{},
			Items:    obj.Objects,
		}
	}

	// We need to override the default output format to JSON so we can return
	// processed template. Users might still be able to change the output
	// format using the 'output' flag.
	if !cmd.Flag("output").Changed {
		cmd.Flags().Set("output", "json")
		printer, _ = f.PrinterForMapping(cmd, mapping)
	}
	return printer.PrintObj(result, out)
}
