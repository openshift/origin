package cmd

import (
	"fmt"
	"io"
	"strings"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/meta"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl"
	kcmdutil "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd/util"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/resource"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
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
	processLong = `Process template into a list of resources specified in filename or stdin

JSON and YAML formats are accepted.`

	processExample = `  // Convert template.json file into resource list
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
		Short:   "Process a template into list of resources",
		Long:    processLong,
		Example: fmt.Sprintf(processExample, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			err := RunProcess(f, out, cmd, args)
			kcmdutil.CheckErr(err)
		},
	}

	cmd.Flags().StringP("filename", "f", "", "Filename or URL to file to read a template")
	cmd.Flags().StringP("value", "v", "", "Specify a list of key-value pairs (eg. -v FOO=BAR,BAR=FOO) to set/override parameter values")
	cmd.Flags().BoolP("parameters", "", false, "Do not process but only print available parameters")
	cmd.Flags().StringP("labels", "l", "", "Label to set in all resources for this template")

	cmd.Flags().StringP("output", "o", "", "Output format. One of: describe|json|yaml|template|templatefile.")
	cmd.Flags().Bool("raw", false, "If true output the processed template instead of the template's objects. Implied by -o describe")
	cmd.Flags().String("output-version", "", "Output the formatted object with the given version (default api-version).")
	cmd.Flags().StringP("template", "t", "", "Template string or path to template file to use when -o=template or -o=templatefile.  The template format is golang templates [http://golang.org/pkg/text/template/#pkg-overview]")
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
			return err
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

		// FIXME: For now, only the first Template item is processed.
		// When the input is provided in STDIN then all Template objects are loaded
		// into api.List items, which assumes that we will process them
		// sequentially. However, this is not (yet) supported as we will have to
		// deal with multiple parameters set by different templates.
		switch t := obj.(type) {
		case *kapi.List:
			if len(t.Items) == 0 {
				return fmt.Errorf("no valid Template items found in the input")
			}
			if len(t.Items) > 1 {
				return fmt.Errorf("you can pass only one Template using standard input")
			}
			var ok bool
			if templateObj, ok = t.Items[0].(*api.Template); !ok {
				return fmt.Errorf("cannot convert input to Template: %v", t)
			}
		case *api.Template:
			templateObj = t
		default:
			return fmt.Errorf("cannot convert input to Template: %v", t)
		}

		templateObj.CreationTimestamp = util.Now()
		version, kind, err := kapi.Scheme.ObjectVersionAndKind(templateObj)
		if err != nil {
			return err
		}
		if mapping, err = mapper.RESTMapping(kind, version); err != nil {
			return err
		}
	}

	if cmd.Flag("value").Changed {
		injectUserVars(cmd, templateObj)
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
		lbl, err := kubectl.ParseLabels(label)
		if err != nil {
			return err
		}
		if templateObj.ObjectLabels == nil {
			templateObj.ObjectLabels = make(map[string]string)
		}
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

	outputVersion := kcmdutil.OutputVersion(cmd, mapping.APIVersion)
	raw := kcmdutil.GetFlagBool(cmd, "raw")
	outputFormat := kcmdutil.GetFlagString(cmd, "output")
	if len(outputFormat) == 0 {
		outputFormat = "json"
	}

	if outputFormat == "describe" {
		s, err := (&describe.TemplateDescriber{
			MetadataAccessor: meta.NewAccessor(),
			ObjectTyper:      kapi.Scheme,
			ObjectDescriber:  nil,
		}).DescribeTemplate(obj)
		if err != nil {
			return err
		}
		fmt.Fprintf(out, s)
		return nil
	}

	// use generic output
	var result runtime.Object
	switch {
	case raw:
		result = obj
	// display the processed template instead of the objects
	default:
		result = &kapi.List{
			ListMeta: kapi.ListMeta{},
			Items:    obj.Objects,
		}
	}
	p, _, err := kubectl.GetPrinter(outputFormat, "")
	if err != nil {
		return err
	}
	p = kubectl.NewVersionedPrinter(p, kapi.Scheme, outputVersion)
	return p.PrintObj(result, out)
}
