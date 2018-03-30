package cmd

import (
	"fmt"
	"io"
	"math/rand"
	"reflect"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/kubectl"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/resource"
	kprinters "k8s.io/kubernetes/pkg/printers"

	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/oc/cli/describe"
	"github.com/openshift/origin/pkg/oc/cli/util/clientcmd"
	"github.com/openshift/origin/pkg/oc/generate/app"
	"github.com/openshift/origin/pkg/template"
	templateapi "github.com/openshift/origin/pkg/template/apis/template"
	templatevalidation "github.com/openshift/origin/pkg/template/apis/template/validation"
	templateinternalclient "github.com/openshift/origin/pkg/template/client/internalversion"
	templateclient "github.com/openshift/origin/pkg/template/generated/internalclientset/typed/template/internalversion"
	"github.com/openshift/origin/pkg/template/generator"
)

var (
	processLong = templates.LongDesc(`
		Process template into a list of resources specified in filename or stdin

		Templates allow parameterization of resources prior to being sent to the server for creation or
		update. Templates have "parameters", which may either be generated on creation or set by the user,
		as well as metadata describing the template.

		The output of the process command is always a list of one or more resources. You may pipe the
		output to the create command over STDIN (using the '-f -' option) or redirect it to a file.

		Process resolves the template on the server, but you may pass --local to parameterize the template
		locally. When running locally be aware that the version of your client tools will determine what
		template transformations are supported, rather than the server.`)

	processExample = templates.Examples(`
		# Convert template.json file into resource list and pass to create
	  %[1]s process -f template.json | %[1]s create -f -

	  # Process a file locally instead of contacting the server
	  %[1]s process -f template.json --local -o yaml

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
		Use:     "process (TEMPLATE | -f FILENAME) [-p=KEY=VALUE]",
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
	cmd.Flags().StringArrayP("param", "p", nil, "Specify a key-value pair (eg. -p FOO=BAR) to set/override a parameter value in the template.")
	cmd.Flags().StringArray("param-file", []string{}, "File containing template parameter values to set/override in the template.")
	cmd.MarkFlagFilename("param-file")
	cmd.Flags().Bool("ignore-unknown-parameters", false, "If true, will not stop processing if a provided parameter does not exist in the template.")
	cmd.Flags().BoolP("local", "", false, "If true process the template locally instead of contacting the server.")
	cmd.Flags().BoolP("parameters", "", false, "If true, do not process but only print available parameters")
	cmd.Flags().StringP("labels", "l", "", "Label to set in all resources for this template")

	cmd.Flags().StringP("output", "o", "json", "Output format. One of: describe|json|yaml|name|go-template=...|go-template-file=...|jsonpath=...|jsonpath-file=...")
	cmd.Flags().Bool("raw", false, "If true, output the processed template instead of the template's objects. Implied by -o describe")
	cmd.Flags().String("output-version", "", "Output the formatted object with the given version (default api-version).")
	cmd.Flags().StringP("template", "t", "", "Template string or path to template file to use when -o=go-template, -o=go-templatefile.  The template format is golang templates [http://golang.org/pkg/text/template/#pkg-overview]")

	// kcmdutil.PrinterForCommand needs these flags, however they are useless
	// here because oc process returns list of heterogeneous objects that is
	// not suitable for formatting as a table.
	cmd.Flags().BoolP("show-all", "a", false, "When printing, show all resources (default hide terminated pods.)")
	cmd.Flags().Bool("show-labels", false, "When printing, show all labels as the last column (default hide labels column)")
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
			return kcmdutil.UsageErrorf(cmd, "template name must be specified only once: %s", s)
		}
	}

	local := kcmdutil.GetFlagBool(cmd, "local")
	if cmd.Flag("param").Changed {
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
		return kcmdutil.UsageErrorf(cmd, fmt.Sprintf("The following parameters were provided more than once: %s", strings.Join(duplicatedKeys.List(), ", ")))
	}

	filename := kcmdutil.GetFlagString(cmd, "filename")
	if len(templateName) == 0 && len(filename) == 0 {
		return kcmdutil.UsageErrorf(cmd, "Must pass a filename or name of stored template")
	}

	if kcmdutil.GetFlagBool(cmd, "parameters") {
		for _, flag := range []string{"param", "labels", "output", "output-version", "raw", "template"} {
			if f := cmd.Flags().Lookup(flag); f != nil && f.Changed {
				return kcmdutil.UsageErrorf(cmd, "The --parameters flag does not process the template, can't be used with --%v", flag)
			}
		}
	}

	namespace, explicit, err := f.DefaultNamespace()
	if err != nil {
		return err
	}

	var (
		objects []runtime.Object
		infos   []*resource.Info

		mapper meta.RESTMapper
		client templateclient.TemplateInterface
	)

	if local {
		// TODO: Change f.Object() so that it can fall back to local RESTMapper safely (currently glog.Fatals)
		mapper = legacyscheme.Registry.RESTMapper()
		// client is deliberately left nil
	} else {
		templateClient, err := f.OpenshiftInternalTemplateClient()
		if err != nil {
			return err
		}
		client = templateClient.Template()
		mapper, _ = f.Object()
	}
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

		templateObj, err := client.Templates(sourceNamespace).Get(storedTemplate, metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				return fmt.Errorf("template %q could not be found", storedTemplate)
			}
			return err
		}
		templateObj.CreationTimestamp = metav1.Now()
		infos = append(infos, &resource.Info{Object: templateObj})
	} else {
		infos, err = f.NewBuilder().
			Internal().
			LocalParam(local).
			FilenameParam(explicit, &resource.FilenameOptions{Recursive: false, Filenames: []string{filename}}).
			Do().
			Infos()
		if err != nil {
			return fmt.Errorf("failed to read input object (not a Template?): %v", err)
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
	if errs := injectUserVars(params, obj, kcmdutil.GetFlagBool(cmd, "ignore-unknown-parameters")); errs != nil {
		return kerrors.NewAggregate(errs)
	}

	resultObj := obj
	if local {
		if err := processTemplateLocally(obj); err != nil {
			return err
		}
	} else {
		processor := templateinternalclient.NewTemplateProcessorClient(client.RESTClient(), namespace)
		resultObj, err = processor.Process(obj)
		if err != nil {
			if err, ok := err.(*errors.StatusError); ok && err.ErrStatus.Details != nil {
				errstr := "unable to process template\n"
				for _, cause := range err.ErrStatus.Details.Causes {
					errstr += fmt.Sprintf("  %s\n", cause.Message)
				}

				// if no error causes found, fallback to returning original
				// error message received from the server
				if len(err.ErrStatus.Details.Causes) == 0 {
					errstr += fmt.Sprintf("  %v\n", err)
				}

				return fmt.Errorf(errstr)
			}

			return fmt.Errorf("unable to process template: %v\n", err)
		}
	}

	outputFormat := kcmdutil.GetFlagString(cmd, "output")
	if outputFormat == "describe" {
		if s, err := (&describe.TemplateDescriber{
			MetadataAccessor: meta.NewAccessor(),
			ObjectTyper:      legacyscheme.Scheme,
			ObjectDescriber:  nil,
		}).DescribeTemplate(resultObj); err != nil {
			return fmt.Errorf("error describing %q: %v\n", obj.Name, err)
		} else {
			_, err := fmt.Fprintf(out, s)
			return err
		}
	}
	objects = append(objects, resultObj.Objects...)

	p, err := f.PrinterForOptions(kcmdutil.ExtractCmdPrintOptions(cmd, false))
	if err != nil {
		return err
	}
	var version schema.GroupVersion
	outputVersionString := kcmdutil.GetFlagString(cmd, "output-version")
	if len(outputVersionString) == 0 {
		version = mapping.GroupVersionKind.GroupVersion()
	} else {
		version, err = schema.ParseGroupVersion(outputVersionString)
		if err != nil {
			return err
		}
	}
	// Prefer the Kubernetes core group for the List over the template.openshift.io
	version.Group = kapi.GroupName
	p = kprinters.NewVersionedPrinter(p, legacyscheme.Scheme, version)

	// use generic output
	if kcmdutil.GetFlagBool(cmd, "raw") {
		for i := range objects {
			p.PrintObj(objects[i], out)
		}
		return nil
	}

	return p.PrintObj(&kapi.List{
		ListMeta: metav1.ListMeta{},
		Items:    objects,
	}, out)
}

// injectUserVars injects user specified variables into the Template
func injectUserVars(values app.Environment, t *templateapi.Template, ignoreUnknownParameters bool) []error {
	var errors []error
	for param, val := range values {
		v := template.GetParameterByName(t, param)
		if v != nil {
			v.Value = val
			v.Generate = ""
		} else if !ignoreUnknownParameters {
			errors = append(errors, fmt.Errorf("unknown parameter name %q\n", param))
		}
	}
	return errors
}

// processTemplateLocally applies the same logic that a remote call would make but makes no
// connection to the server.
func processTemplateLocally(tpl *templateapi.Template) error {
	if errs := templatevalidation.ValidateProcessedTemplate(tpl); len(errs) > 0 {
		return errors.NewInvalid(templateapi.Kind("Template"), tpl.Name, errs)
	}
	processor := template.NewProcessor(map[string]generator.Generator{
		"expression": generator.NewExpressionValueGenerator(rand.New(rand.NewSource(time.Now().UnixNano()))),
	})
	if errs := processor.Process(tpl); len(errs) > 0 {
		return errors.NewInvalid(templateapi.Kind("Template"), tpl.Name, errs)
	}
	return nil
}
