package cmd

import (
	"fmt"
	"io"
	"math/rand"
	"reflect"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"k8s.io/client-go/rest"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/kubectl"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"
	oldresource "k8s.io/kubernetes/pkg/kubectl/resource"
	kprinters "k8s.io/kubernetes/pkg/printers"

	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/oc/cli/describe"
	"github.com/openshift/origin/pkg/oc/cli/util/clientcmd"
	"github.com/openshift/origin/pkg/oc/generate/app"
	templateapi "github.com/openshift/origin/pkg/template/apis/template"
	templatevalidation "github.com/openshift/origin/pkg/template/apis/template/validation"
	templateinternalclient "github.com/openshift/origin/pkg/template/client/internalversion"
	templateclientinternal "github.com/openshift/origin/pkg/template/generated/internalclientset"
	templateclient "github.com/openshift/origin/pkg/template/generated/internalclientset/typed/template/internalversion"
	"github.com/openshift/origin/pkg/template/generator"
	"github.com/openshift/origin/pkg/template/templateprocessing"
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

type ProcessOptions struct {
	PrintFlags *genericclioptions.PrintFlags
	genericclioptions.IOStreams

	usageErrorFn func(string, ...interface{}) error

	outputFormat        string
	labels              string
	filename            string
	local               bool
	raw                 bool
	parameters          bool
	ignoreUnknownParams bool
	templateName        string
	paramFile           []string
	templateParams      []string
	namespace           string
	explicitNamespace   bool

	builderFn      func() *oldresource.Builder
	clientConfigFn func() (*rest.Config, error)

	mapper meta.RESTMapper

	printObj func(*meta.RESTMapping, runtime.Object, io.Writer) error
}

func NewProcessOptions(streams genericclioptions.IOStreams) *ProcessOptions {
	return &ProcessOptions{
		PrintFlags: genericclioptions.NewPrintFlags("").WithDefaultOutput("json"),
		IOStreams:  streams,
	}
}

// NewCmdProcess implements the OpenShift cli process command
func NewCmdProcess(fullName string, f *clientcmd.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewProcessOptions(streams)

	cmd := &cobra.Command{
		Use:     "process (TEMPLATE | -f FILENAME) [-p=KEY=VALUE]",
		Short:   "Process a template into list of resources",
		Long:    processLong,
		Example: fmt.Sprintf(processExample, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(o.Complete(f, cmd, args))
			kcmdutil.CheckErr(o.RunProcess(f))
		},
	}

	o.PrintFlags.AddFlags(cmd)

	cmd.Flags().StringVarP(&o.filename, "filename", "f", o.filename, "Filename or URL to file to read a template")
	cmd.MarkFlagFilename("filename", "yaml", "yml", "json")
	cmd.Flags().StringArrayP("param", "p", nil, "Specify a key-value pair (eg. -p FOO=BAR) to set/override a parameter value in the template.")
	cmd.Flags().StringArrayVar(&o.paramFile, "param-file", o.paramFile, "File containing template parameter values to set/override in the template.")
	cmd.MarkFlagFilename("param-file")
	cmd.Flags().BoolVar(&o.ignoreUnknownParams, "ignore-unknown-parameters", o.ignoreUnknownParams, "If true, will not stop processing if a provided parameter does not exist in the template.")
	cmd.Flags().BoolVarP(&o.local, "local", "", o.local, "If true process the template locally instead of contacting the server.")
	cmd.Flags().BoolVarP(&o.parameters, "parameters", "", o.parameters, "If true, do not process but only print available parameters")
	cmd.Flags().StringVarP(&o.labels, "labels", "l", o.labels, "Label to set in all resources for this template")

	cmd.Flags().BoolVar(&o.raw, "raw", o.raw, "If true, output the processed template instead of the template's objects. Implied by -o describe")

	// FIXME-REBASE: remove these humanreadable-specific flags once we are able to use o.PrintFlags.ToPrinter()
	// we must bind them for now, while we are dependent on cmdutil.PrinterForOptions
	cmd.Flags().BoolP("show-all", "a", false, "When printing, show all resources (default hide terminated pods.)")
	cmd.Flags().Bool("show-labels", false, "When printing, show all labels as the last column (default hide labels column)")
	cmd.Flags().Bool("no-headers", false, "When using the default output, don't print headers.")
	cmd.Flags().MarkHidden("no-headers")
	cmd.Flags().String("sort-by", "", "If non-empty, sort list types using this field specification.  The field specification is expressed as a JSONPath expression (e.g. 'ObjectMeta.Name'). The field in the API resource specified by this JSONPath expression must be an integer or a string.")
	cmd.Flags().MarkHidden("sort-by")

	// REBASE-FIXME: we need to wire a custom printer for this command
	// modify description on "output" flag
	//outputFlag := cmd.Flags().Lookup("output")
	//if outputFlag != nil {
	//	outputFlag.Usage = "Output format. One of: describe|json|yaml|name|go-template=...|go-template-file=...|jsonpath=...|jsonpath-file=..."
	//}
	return cmd
}

func (o *ProcessOptions) Complete(f *clientcmd.Factory, cmd *cobra.Command, args []string) error {
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

	if cmd.Flag("param").Changed {
		flagValues := getFlagStringArray(cmd, "param")
		cmdutil.WarnAboutCommaSeparation(o.ErrOut, flagValues, "--param")
		templateParams = append(templateParams, flagValues...)
	}

	o.templateParams = templateParams
	o.templateName = templateName

	o.builderFn = f.NewBuilder
	o.clientConfigFn = f.ClientConfig

	if o.local {
		// TODO: Change f.Object() so that it can fall back to local RESTMapper safely (currently glog.Fatals)
		o.mapper = legacyscheme.Registry.RESTMapper()
	} else {
		o.mapper, _ = f.Object()
	}

	var err error
	o.namespace, o.explicitNamespace, err = f.DefaultNamespace()
	// we only need to fail on namespace acquisition if we're actually taking action.
	// Otherwise the namespace can be enforced later.
	if err != nil && !o.local {
		return err
	}

	// FIXME-REBASE
	//printer, err := o.PrintFlags.ToPrinter()
	//if err != nil {
	//	return err
	//}
	o.printObj = func(mapping *meta.RESTMapping, obj runtime.Object, out io.Writer) error {
		version := mapping.GroupVersionKind.GroupVersion()
		version.Group = kapi.GroupName

		p, err := kcmdutil.PrinterForOptions(kcmdutil.ExtractCmdPrintOptions(cmd, false))
		if err != nil {
			return err
		}

		// Prefer the Kubernetes core group for the List over the template.openshift.io
		p = kprinters.NewVersionedPrinter(p, legacyscheme.Scheme, legacyscheme.Scheme, version)
		return p.PrintObj(obj, out)
	}
	o.outputFormat = kcmdutil.GetFlagString(cmd, "output")

	o.usageErrorFn = func(format string, args ...interface{}) error {
		return kcmdutil.UsageErrorf(cmd, format, args)
	}
	return nil
}

func (o *ProcessOptions) Validate(cmd *cobra.Command) error {
	if o.parameters {
		for _, flag := range []string{"param", "labels", "output", "output-version", "raw", "template"} {
			if f := cmd.Flags().Lookup(flag); f != nil && f.Changed {
				return kcmdutil.UsageErrorf(cmd, "The --parameters flag does not process the template, can't be used with --%v", flag)
			}
		}
	}

	return nil
}

// RunProcess contains all the necessary functionality for the OpenShift cli process command
func (o *ProcessOptions) RunProcess(g *clientcmd.Factory) error {

	duplicatedKeys := sets.NewString()
	params, paramErr := app.ParseAndCombineEnvironment(o.templateParams, o.paramFile, o.In, func(key, file string) error {
		if file == "" {
			duplicatedKeys.Insert(key)
		} else {
			fmt.Fprintf(o.ErrOut, "warning: Template parameter %q already defined, ignoring value from file %q", key, file)
		}
		return nil
	})
	if len(duplicatedKeys) != 0 {
		return o.usageErrorFn(fmt.Sprintf("The following parameters were provided more than once: %s", strings.Join(duplicatedKeys.List(), ", ")))
	}

	if len(o.templateName) == 0 && len(o.filename) == 0 {
		return o.usageErrorFn("Must pass a filename or name of stored template")
	}

	var (
		err     error
		objects []runtime.Object
		infos   []*oldresource.Info

		client templateclient.TemplateInterface
	)

	if !o.local {
		clientConfig, err := o.clientConfigFn()
		if err != nil {
			return err
		}
		templateClient, err := templateclientinternal.NewForConfig(clientConfig)
		if err != nil {
			return err
		}
		client = templateClient.Template()
	}
	// client is deliberately left nil on local

	// When templateName is not empty, then we fetch the template from the
	// server, otherwise we require to set the `-f` parameter.
	if len(o.templateName) > 0 {
		var (
			storedTemplate, rs string
			sourceNamespace    string
			ok                 bool
		)
		sourceNamespace, rs, storedTemplate, ok = parseNamespaceResourceName(o.templateName, o.namespace)
		if !ok {
			return fmt.Errorf("invalid argument %q", o.templateName)
		}
		if len(rs) > 0 && (rs != "template" && rs != "templates") {
			return fmt.Errorf("unable to process invalid resource %q", rs)
		}
		if len(storedTemplate) == 0 {
			return fmt.Errorf("invalid value syntax %q", o.templateName)
		}

		templateObj, err := client.Templates(sourceNamespace).Get(storedTemplate, metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				return fmt.Errorf("template %q could not be found", storedTemplate)
			}
			return err
		}
		templateObj.CreationTimestamp = metav1.Now()
		infos = append(infos, &oldresource.Info{Object: templateObj})
	} else {
		infos, err = o.builderFn().
			Internal().
			LocalParam(o.local).
			FilenameParam(o.explicitNamespace, &oldresource.FilenameOptions{Recursive: false, Filenames: []string{o.filename}}).
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
		fmt.Fprintf(o.Out, "%d input templates found, but only the first will be processed", len(infos))
	}

	obj, ok := infos[0].Object.(*templateapi.Template)
	if !ok {
		sourceName := o.filename
		if len(o.templateName) > 0 {
			sourceName = o.namespace + "/" + o.templateName
		}
		return fmt.Errorf("unable to parse %q, not a valid Template but %s\n", sourceName, reflect.TypeOf(infos[0].Object))
	}

	// If 'parameters' flag is set it does not do processing but only print
	// the template parameters to console for inspection.
	if o.parameters {
		return describe.PrintTemplateParameters(obj.Parameters, o.Out)
	}

	if label := o.labels; len(label) > 0 {
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
	if errs := injectUserVars(params, obj, o.ignoreUnknownParams); errs != nil {
		return kerrors.NewAggregate(errs)
	}

	resultObj := obj
	if o.local {
		if err := processTemplateLocally(obj); err != nil {
			return err
		}
	} else {
		processor := templateinternalclient.NewTemplateProcessorClient(client.RESTClient(), o.namespace)
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

	mapping, err := o.mapper.RESTMapping(templateapi.Kind("Template"))
	if err != nil {
		return err
	}

	if o.outputFormat == "describe" {
		if s, err := (&describe.TemplateDescriber{
			MetadataAccessor: meta.NewAccessor(),
			ObjectTyper:      legacyscheme.Scheme,
			ObjectDescriber:  nil,
		}).DescribeTemplate(resultObj); err != nil {
			return fmt.Errorf("error describing %q: %v\n", obj.Name, err)
		} else {
			_, err := fmt.Fprintf(o.Out, s)
			return err
		}
	}
	objects = append(objects, resultObj.Objects...)

	// use generic output
	if o.raw {
		for i := range objects {
			o.printObj(mapping, objects[i], o.Out)
		}
		return nil
	}

	return o.printObj(mapping, &kapi.List{
		ListMeta: metav1.ListMeta{},
		Items:    objects,
	}, o.Out)
}

// injectUserVars injects user specified variables into the Template
func injectUserVars(values app.Environment, t *templateapi.Template, ignoreUnknownParameters bool) []error {
	var errors []error
	for param, val := range values {
		v := templateprocessing.GetParameterByName(t, param)
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
	processor := templateprocessing.NewProcessor(map[string]generator.Generator{
		"expression": generator.NewExpressionValueGenerator(rand.New(rand.NewSource(time.Now().UnixNano()))),
	})
	if errs := processor.Process(tpl); len(errs) > 0 {
		return errors.NewInvalid(templateapi.Kind("Template"), tpl.Name, errs)
	}
	return nil
}
