package appjson

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions/printers"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions/resource"
	"k8s.io/kubernetes/pkg/kubectl/scheme"

	templatev1 "github.com/openshift/api/template/v1"
	"github.com/openshift/origin/pkg/oc/lib/newapp/appjson"
	appcmd "github.com/openshift/origin/pkg/oc/lib/newapp/cmd"
	"github.com/openshift/origin/pkg/oc/util/ocscheme"
	templateapi "github.com/openshift/origin/pkg/template/apis/template"
	templateapiv1 "github.com/openshift/origin/pkg/template/apis/template/v1"
	templatev1client "github.com/openshift/origin/pkg/template/client/v1"
)

const AppJSONV1GeneratorName = "app-json/v1"

var (
	appJSONLong = templates.LongDesc(`
		Import app.json files as OpenShift objects

		app.json defines the pattern of a simple, stateless web application that can be horizontally scaled.
		This command will transform a provided app.json object into its OpenShift equivalent.
		During transformation fields in the app.json syntax that are not relevant when running on top of
		a containerized platform will be ignored and a warning printed.

		The command will create objects unless you pass the -o yaml or --as-template flags to generate a
		configuration file for later use.

		Experimental: This command is under active development and may change without notice.`)

	appJSONExample = templates.Examples(`
		# Import a directory containing an app.json file
	  $ %[1]s app.json -f .

	  # Turn an app.json file into a template
	  $ %[1]s app.json -f ./app.json -o yaml --as-template`)
)

type AppJSONOptions struct {
	PrintFlags *genericclioptions.PrintFlags

	Printer printers.ResourcePrinter

	BaseImage        string
	Generator        string
	AsTemplate       string
	OutputVersionStr string

	OutputVersions []schema.GroupVersion

	Namespace     string
	RESTMapper    meta.RESTMapper
	DynamicClient dynamic.Interface
	Client        rest.Interface

	genericclioptions.IOStreams
	resource.FilenameOptions
}

func NewAppJSONOptions(streams genericclioptions.IOStreams) *AppJSONOptions {
	return &AppJSONOptions{
		IOStreams:  streams,
		PrintFlags: genericclioptions.NewPrintFlags("created").WithTypeSetter(ocscheme.PrintingInternalScheme),
		Generator:  AppJSONV1GeneratorName,
	}
}

// NewCmdAppJSON imports an app.json file (schema described here: https://devcenter.heroku.com/articles/app-json-schema)
// as a template.
func NewCmdAppJSON(fullName string, f kcmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewAppJSONOptions(streams)
	cmd := &cobra.Command{
		Use:     "app.json -f APPJSON",
		Short:   "Import an app.json definition into OpenShift (experimental)",
		Long:    appJSONLong,
		Example: fmt.Sprintf(appJSONExample, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(o.Complete(f, cmd, args))
			kcmdutil.CheckErr(o.Validate())
			kcmdutil.CheckErr(o.Run())
		},
	}
	usage := "Filename, directory, or URL to app.json file to use"
	kcmdutil.AddJsonFilenameFlag(cmd.Flags(), &o.Filenames, usage)
	cmd.MarkFlagRequired("filename")
	cmd.Flags().StringVar(&o.BaseImage, "image", o.BaseImage, "An optional image to use as your base Docker build (must have ONBUILD directives)")
	cmd.Flags().StringVar(&o.Generator, "generator", o.Generator, "The name of the generator strategy to use - specify this value to for backwards compatibility.")
	cmd.Flags().StringVar(&o.AsTemplate, "as-template", o.AsTemplate, "If set, generate a template with the provided name")
	cmd.Flags().StringVar(&o.OutputVersionStr, "output-version", o.OutputVersionStr, "The preferred API versions of the output objects")

	o.PrintFlags.AddFlags(cmd)
	return cmd
}

func (o *AppJSONOptions) createResources(list *corev1.List) (*corev1.List, []error) {
	errors := []error{}
	created := &corev1.List{}

	for i, item := range list.Items {
		var err error
		unstructuredObj := &unstructured.Unstructured{}
		unstructuredObj.Object, err = runtime.DefaultUnstructuredConverter.ToUnstructured(item)
		if err != nil {
			errors = append(errors, err)
			continue
		}

		mapping, err := o.RESTMapper.RESTMapping(unstructuredObj.GroupVersionKind().GroupKind(), unstructuredObj.GroupVersionKind().Version)
		if err != nil {
			errors = append(errors, err)
			continue
		}

		_, err = o.DynamicClient.Resource(mapping.Resource).Namespace(o.Namespace).Create(unstructuredObj)
		if err != nil {
			errors = append(errors, err)
			continue
		}

		created.Items = append(created.Items, list.Items[i])
	}

	return created, errors
}

func (o *AppJSONOptions) Complete(f kcmdutil.Factory, cmd *cobra.Command, args []string) error {
	for _, v := range strings.Split(o.OutputVersionStr, ",") {
		gv, err := schema.ParseGroupVersion(v)
		if err != nil {
			return fmt.Errorf("provided output-version %q is not valid: %v", v, err)
		}
		o.OutputVersions = append(o.OutputVersions, gv)
	}
	o.OutputVersions = append(o.OutputVersions, scheme.Scheme.PrioritizedVersionsAllGroups()...)

	clientConfig, err := f.ToRESTConfig()
	if err != nil {
		return err
	}

	o.RESTMapper, err = f.ToRESTMapper()
	if err != nil {
		return err
	}

	o.DynamicClient, err = dynamic.NewForConfig(clientConfig)

	o.Printer, err = o.PrintFlags.ToPrinter()
	if err != nil {
		return err
	}

	o.Namespace, _, err = f.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return err
	}

	clientset, err := kubernetes.NewForConfig(clientConfig)
	if err != nil {
		return err
	}

	o.Client = clientset.CoreV1().RESTClient()
	if err != nil {
		return err
	}

	return nil
}

func (o *AppJSONOptions) Validate() error {
	if len(o.Filenames) != 1 {
		return fmt.Errorf("you must provide the path to an app.json file or directory containing app.json")
	}
	switch o.Generator {
	case AppJSONV1GeneratorName:
	default:
		return fmt.Errorf("the generator %q is not supported, use: %s", o.Generator, AppJSONV1GeneratorName)
	}
	return nil
}

func (o *AppJSONOptions) Run() error {
	localPath, contents, err := contentsForPathOrURL(o.Filenames[0], o.In, "app.json")
	if err != nil {
		return err
	}

	g := &appjson.Generator{
		LocalPath: localPath,
		BaseImage: o.BaseImage,
	}
	switch {
	case len(o.AsTemplate) > 0:
		g.Name = o.AsTemplate
	case len(localPath) > 0:
		g.Name = filepath.Base(localPath)
	default:
		g.Name = path.Base(path.Dir(o.Filenames[0]))
	}
	if len(g.Name) == 0 {
		g.Name = "app"
	}

	template, err := g.Generate(contents)
	if err != nil {
		return err
	}

	externalTemplate := &templatev1.Template{}
	if err := templateapiv1.Convert_template_Template_To_v1_Template(template, externalTemplate, nil); err != nil {
		return err
	}

	externalTemplate.ObjectLabels = map[string]string{"app.json": externalTemplate.Name}

	// TODO: stop implying --dry-run behavior when an --output value is provided
	if o.PrintFlags.OutputFormat != nil && len(*o.PrintFlags.OutputFormat) > 0 || len(o.AsTemplate) > 0 {
		var obj runtime.Object
		if len(o.AsTemplate) > 0 {
			externalTemplate.Name = o.AsTemplate
			obj = externalTemplate
		} else {
			obj = &corev1.List{Items: externalTemplate.Objects}
		}
		return o.Printer.PrintObj(obj, o.Out)
	}

	templateProcessor := templatev1client.NewTemplateProcessorClient(o.Client, o.Namespace)
	result, err := appcmd.TransformTemplate(externalTemplate, templateProcessor, o.Namespace, nil, false)
	if err != nil {
		return err
	}

	// TODO(juanvallejo): remove once we have external version describers
	describableResult := &templateapi.Template{}
	if err := templateapiv1.Convert_v1_Template_To_template_Template(result, describableResult, nil); err != nil {
		return err
	}

	appcmd.DescribeGeneratedTemplate(o.Out, "", describableResult, o.Namespace)

	objs := &corev1.List{Items: result.Objects}

	// actually create the objects
	created, errs := o.createResources(objs)

	// print what we have created first, then return a potential set of errors
	if err := o.Printer.PrintObj(created, o.Out); err != nil {
		errs = append(errs, err)
	}

	return kerrors.NewAggregate(errs)
}

func contentsForPathOrURL(s string, in io.Reader, subpaths ...string) (string, []byte, error) {
	switch {
	case s == "-":
		contents, err := ioutil.ReadAll(in)
		return "", contents, err
	case strings.Index(s, "http://") == 0 || strings.Index(s, "https://") == 0:
		_, err := url.Parse(s)
		if err != nil {
			return "", nil, fmt.Errorf("the URL passed to filename %q is not valid: %v", s, err)
		}
		res, err := http.Get(s)
		if err != nil {
			return "", nil, err
		}
		defer res.Body.Close()
		contents, err := ioutil.ReadAll(res.Body)
		return "", contents, err
	default:
		stat, err := os.Stat(s)
		if err != nil {
			return s, nil, err
		}
		if !stat.IsDir() {
			contents, err := ioutil.ReadFile(s)
			return s, contents, err
		}
		for _, sub := range subpaths {
			path := filepath.Join(s, sub)
			stat, err := os.Stat(path)
			if err != nil {
				continue
			}
			if stat.IsDir() {
				continue
			}
			contents, err := ioutil.ReadFile(s)
			return path, contents, err
		}
		return s, nil, os.ErrNotExist
	}
}
