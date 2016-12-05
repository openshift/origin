package cmd

import (
	"fmt"
	"io"
	"reflect"
	"strings"

	"github.com/spf13/cobra"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/api/meta"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/kubectl"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/resource"
	"k8s.io/kubernetes/pkg/runtime"
	kerrors "k8s.io/kubernetes/pkg/util/errors"
	"k8s.io/kubernetes/pkg/util/sets"

	"github.com/openshift/origin/pkg/cmd/cli/describe"
	"github.com/openshift/origin/pkg/cmd/templates"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	"github.com/openshift/origin/pkg/generate/app"
	"github.com/openshift/origin/pkg/template"
	templateapi "github.com/openshift/origin/pkg/template/api"
)

var (
	processLong = templates.LongDesc(`
		Process template into a list of resources specified in filename or stdin

		Templates allow parameterization of resources prior to being sent to the server for creation or
		update. Templates have "parameters", which may either be generated on creation or set by the user,
		as well as metadata describing the template.

		The output of the process command is always a list of one or more resources. You may pipe the
		output to the create command over STDIN (using the '-f -' option) or redirect it to a file.`)

	processExample = templates.Examples(`
		# Convert template.json file into resource list and pass to create
	  %[1]s process -f template.json | %[1]s create -f -

	  # Process template while passing a user-defined label
	  %[1]s process -f template.json -l name=mytemplate

	  # Convert stored template into resource list
	  %[1]s process foo

	  # Convert stored template into resource list by setting/overriding parameter values
	  %[1]s process foo PARM1=VALUE1 PARM2=VALUE2

	  # Convert template stored in different namespace into a resource list
	  %[1]s process openshift//foo

	  # Convert template.json into resource list
	  cat template.json | %[1]s process -f -`)
)

// NewCmdProcess implements the OpenShift cli process command
func NewCmdProcess(fullName string, f *clientcmd.Factory, in io.Reader, out, errout io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "process (TEMPLATE | -f FILENAME) [-v=KEY=VALUE]",
		Short:   "Process a template into list of resources",
		Long:    processLong,
		Example: fmt.Sprintf(processExample, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			err := RunProcess(f, in, out, errout, cmd, args)
			kcmdutil.CheckErr(err)
		},
	}
	cmd.Flags().StringP("filename", "f", "", "Filename or URL to file to read a template")
	cmd.MarkFlagFilename("filename", "yaml", "yml", "json")
	params := cmd.Flags().StringArrayP("value", "v", nil, "Specify a key-value pair (eg. -v FOO=BAR) to set/override a parameter value in the template.")
	cmd.Flags().MarkDeprecated("value", "Use -p, --param instead.")
	cmd.Flags().MarkHidden("value")
	cmd.Flags().StringArrayVarP(params, "param", "p", nil, "Specify a key-value pair (eg. -p FOO=BAR) to set/override a parameter value in the template.")
	cmd.Flags().StringArray("param-file", []string{}, "File containing template parameter values to set/override in the template.")
	cmd.MarkFlagFilename("param-file")
	cmd.Flags().BoolP("parameters", "", false, "Do not process but only print available parameters")
	cmd.Flags().StringP("labels", "l", "", "Label to set in all resources for this template")

	cmd.Flags().StringP("output", "o", "json", "Output format. One of: describe|json|yaml|name|go-template=...|go-template-file=...|jsonpath=...|jsonpath-file=...")
	cmd.Flags().Bool("raw", false, "If true output the processed template instead of the template's objects. Implied by -o describe")
	cmd.Flags().String("output-version", "", "Output the formatted object with the given version (default api-version).")
	cmd.Flags().StringP("template", "t", "", "Template string or path to template file to use when -o=go-template, -o=go-templatefile.  The template format is golang templates [http://golang.org/pkg/text/template/#pkg-overview]")

	// kcmdutil.PrinterForCommand needs these flags, however they are useless
	// here because oc process returns list of heterogeneous objects that is
	// not suitable for formatting as a table.
	cmd.Flags().Bool("no-headers", false, "When using the default output, don't print headers.")
	cmd.Flags().MarkHidden("no-headers")
	cmd.Flags().String("sort-by", "", "If non-empty, sort list types using this field specification.  The field specification is expressed as a JSONPath expression (e.g. 'ObjectMeta.Name'). The field in the API resource specified by this JSONPath expression must be an integer or a string.")
	cmd.Flags().MarkHidden("sort-by")

	return cmd
}

// RunProcess contains all the necessary functionality for the OpenShift cli process command
func RunProcess(f *clientcmd.Factory, in io.Reader, out, errout io.Writer, cmd *cobra.Command, args []string) error {
	templateName, templateParams := "", []string{}
	for _, s := range args {
		isValue := strings.Contains(s, "=")
		switch {
		case isValue:
			templateParams = append(templateParams, s)
		case !isValue && len(templateName) == 0:
			templateName = s
		case !isValue && len(templateName) > 0:
			return kcmdutil.UsageError(cmd, "template name must be specified only once: %s", s)
		}
	}

	if cmd.Flag("value").Changed || cmd.Flag("param").Changed {
		flagValues := getFlagStringArray(cmd, "param")
		cmdutil.WarnAboutCommaSeparation(errout, flagValues, "--param")
		templateParams = append(templateParams, flagValues...)
	}

	duplicatedKeys := sets.NewString()
	params, paramErr := app.ParseAndCombineEnvironment(templateParams, getFlagStringArray(cmd, "param-file"), in, func(key, file string) error {
		if file == "" {
			duplicatedKeys.Insert(key)
		} else {
			fmt.Fprintf(errout, "warning: Template parameter %q already defined, ignoring value from file %q", key, file)
		}
		return nil
	})
	if len(duplicatedKeys) != 0 {
		return kcmdutil.UsageError(cmd, fmt.Sprintf("The following parameters were provided more than once: %s", strings.Join(duplicatedKeys.List(), ", ")))
	}

	filename := kcmdutil.GetFlagString(cmd, "filename")
	if len(templateName) == 0 && len(filename) == 0 {
		return kcmdutil.UsageError(cmd, "Must pass a filename or name of stored template")
	}

	if kcmdutil.GetFlagBool(cmd, "parameters") {
		for _, flag := range []string{"value", "param", "labels", "output", "output-version", "raw", "template"} {
			if f := cmd.Flags().Lookup(flag); f != nil && f.Changed {
				return kcmdutil.UsageError(cmd, "The --parameters flag does not process the template, can't be used with --%v", flag)
			}
		}
	}

	namespace, explicit, err := f.DefaultNamespace()
	if err != nil {
		return err
	}

	mapper, typer := f.Object(false)

	client, _, _, err := f.Clients()
	if err != nil {
		return err
	}

	var (
		objects []runtime.Object
		infos   []*resource.Info
	)

	mapping, err := mapper.RESTMapping(templateapi.Kind("Template"))
	if err != nil {
		return err
	}

	// When templateName is not empty, then we fetch the template from the
	// server, otherwise we require to set the `-f` parameter.
	if len(templateName) > 0 {
		var (
			storedTemplate, rs string
			sourceNamespace    string
			ok                 bool
		)
		sourceNamespace, rs, storedTemplate, ok = parseNamespaceResourceName(templateName, namespace)
		if !ok {
			return fmt.Errorf("invalid argument %q", templateName)
		}
		if len(rs) > 0 && (rs != "template" && rs != "templates") {
			return fmt.Errorf("unable to process invalid resource %q", rs)
		}
		if len(storedTemplate) == 0 {
			return fmt.Errorf("invalid value syntax %q", templateName)
		}
		templateObj, err := client.Templates(sourceNamespace).Get(storedTemplate)
		if err != nil {
			if errors.IsNotFound(err) {
				return fmt.Errorf("template %q could not be found", storedTemplate)
			}
			return err
		}
		templateObj.CreationTimestamp = unversioned.Now()
		infos = append(infos, &resource.Info{Object: templateObj})
	} else {
		infos, err = resource.NewBuilder(mapper, typer, resource.ClientMapperFunc(f.ClientForMapping), kapi.Codecs.UniversalDecoder()).
			NamespaceParam(namespace).RequireNamespace().
			FilenameParam(explicit, false, filename).
			Do().
			Infos()
		if err != nil {
			return err
		}
	}

	if len(infos) > 1 {
		// in order to run validation on the input given to us by a user, we only support the processing
		// of one template in a list. For instance, we want to be able to fail when a user does not give
		// a parameter that the template wants or when they give a parameter the template doesn't need,
		// as this may indicate that they have mis-used `oc process`. This is much less complicated when
		// we process at most one template.
		fmt.Fprintf(out, "%d input templates found, but only the first will be processed", len(infos))
	}

	obj, ok := infos[0].Object.(*templateapi.Template)
	if !ok {
		sourceName := filename
		if len(templateName) > 0 {
			sourceName = namespace + "/" + templateName
		}
		return fmt.Errorf("unable to parse %q, not a valid Template but %s\n", sourceName, reflect.TypeOf(infos[0].Object))
	}

	// If 'parameters' flag is set it does not do processing but only print
	// the template parameters to console for inspection.
	if kcmdutil.GetFlagBool(cmd, "parameters") {
		return describe.PrintTemplateParameters(obj.Parameters, out)
	}

	if label := kcmdutil.GetFlagString(cmd, "labels"); len(label) > 0 {
		lbl, err := kubectl.ParseLabels(label)
		if err != nil {
			return fmt.Errorf("error parsing labels: %v\n", err)
		}
		if obj.ObjectLabels == nil {
			obj.ObjectLabels = make(map[string]string)
		}
		for key, value := range lbl {
			obj.ObjectLabels[key] = value
		}
	}

	// Raise parameter parsing errors here after we had chance to return UsageErrors first
	if paramErr != nil {
		return paramErr
	}
	if errs := injectUserVars(params, obj); errs != nil {
		return kerrors.NewAggregate(errs)
	}

	resultObj, err := client.TemplateConfigs(namespace).Create(obj)
	if err != nil {
		return fmt.Errorf("error processing the template %q: %v\n", obj.Name, err)
	}

	outputFormat := kcmdutil.GetFlagString(cmd, "output")
	if outputFormat == "describe" {
		if s, err := (&describe.TemplateDescriber{
			MetadataAccessor: meta.NewAccessor(),
			ObjectTyper:      kapi.Scheme,
			ObjectDescriber:  nil,
		}).DescribeTemplate(resultObj); err != nil {
			return fmt.Errorf("error describing %q: %v\n", obj.Name, err)
		} else {
			_, err := fmt.Fprintf(out, s)
			return err
		}
	}
	objects = append(objects, resultObj.Objects...)

	p, _, err := kcmdutil.PrinterForCommand(cmd)
	if err != nil {
		return err
	}
	gv := mapping.GroupVersionKind.GroupVersion()
	version, err := kcmdutil.OutputVersion(cmd, &gv)
	if err != nil {
		return err
	}
	p = kubectl.NewVersionedPrinter(p, kapi.Scheme, version)

	// use generic output
	if kcmdutil.GetFlagBool(cmd, "raw") {
		for i := range objects {
			p.PrintObj(objects[i], out)
		}
		return nil
	}

	return p.PrintObj(&kapi.List{
		ListMeta: unversioned.ListMeta{},
		Items:    objects,
	}, out)
}

// injectUserVars injects user specified variables into the Template
func injectUserVars(values app.Environment, t *templateapi.Template) []error {
	var errors []error
	for param, val := range values {
		if v := template.GetParameterByName(t, param); v != nil {
			v.Value = val
			v.Generate = ""
		} else {
			errors = append(errors, fmt.Errorf("unknown parameter name %q\n", param))
		}
	}
	return errors
}
