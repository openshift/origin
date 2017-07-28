package cmd

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"k8s.io/apimachinery/pkg/runtime"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/resource"

	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	templateapi "github.com/openshift/origin/pkg/template/apis/template"
	templateclient "github.com/openshift/origin/pkg/template/generated/internalclientset"
)

var (
	parameterizeLong = templates.LongDesc(`
		Create a template with parameters from a list of resources or an existing template.`)

	parameterizeExample = templates.Examples(`
	  # Convert list.json resource list into a parameterized template
	  %[1]s parameterize -f list.json --aspects=image-refs --name mytemplate > mytemplate.yaml`)
)

// ParameterizeOptions is the set of options for running the parameterize command
type ParameterizeOptions struct {
	Out io.Writer
	Err io.Writer
	In  io.Reader

	Builder        *resource.Builder
	Infos          []*resource.Info
	TemplateClient templateclient.Interface
	PrintObject    func(runtime.Object) error

	Namespace    string
	Filename     string
	TemplateName string
	Local        bool
	Aspects      []string
	Name         string
}

// NewCmdParameterize implements the OpenShift cli parameterize command
func NewCmdParameterize(fullName string, f *clientcmd.Factory, in io.Reader, out, errout io.Writer) *cobra.Command {
	opt := &ParameterizeOptions{
		Out: out,
		Err: errout,
		In:  in,
	}
	cmd := &cobra.Command{
		Use:     "parameterize (TEMPLATE | -f FILENAME)",
		Short:   "Parameterize a list of resources into a template",
		Long:    parameterizeLong,
		Example: fmt.Sprintf(parameterizeExample, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(opt.Complete(f, cmd, args))
			kcmdutil.CheckErr(opt.Validate())
			if err := opt.Run(); err != nil {
				// TODO: move me to kcmdutil
				if err == cmdutil.ErrExit {
					os.Exit(1)
				}
				kcmdutil.CheckErr(err)
			}
		},
	}
	cmd.Flags().StringVarP(&opt.Filename, "filename", "f", "", "Filename or URL to file that contains template or resource list")
	cmd.MarkFlagFilename("filename", "yaml", "yml", "json")
	cmd.Flags().BoolVar(&opt.Local, "local", false, "If true parameterize the template locally instead of contacting the server.")
	cmd.Flags().StringArrayVar(&opt.Aspects, "aspects", []string{"image-refs"}, "Aspects to parameterize. Supported: image-refs")
	cmd.Flags().StringVar(&opt.Name, "name", "", "Name of the template to create")

	kcmdutil.AddPrinterFlags(cmd)

	return cmd
}

// Complete updates options to make the command runnable
func (p *ParameterizeOptions) Complete(f *clientcmd.Factory, cmd *cobra.Command, args []string) error {
	if len(args) > 1 {
		return kcmdutil.UsageError(cmd, "template name must be specified only once")
	}
	if len(args) == 1 {
		p.TemplateName = args[0]
	}

	if len(p.TemplateName) > 0 && len(p.Filename) > 0 {
		return kcmdutil.UsageError(cmd, "specify a template name or file name, but not both")
	}

	ns, explicit, err := f.DefaultNamespace()
	if err != nil {
		return err
	}

	p.Namespace = ns

	p.Builder = f.NewBuilder(!p.Local).
		ContinueOnError().
		NamespaceParam(p.Namespace).DefaultNamespace().
		FilenameParam(explicit, &resource.FilenameOptions{Recursive: false, Filenames: []string{p.Filename}}).
		Flatten()
	if !p.Local && len(p.TemplateName) > 0 {
		p.Builder = p.Builder.
			ResourceNames("templates", p.TemplateName).
			Latest()
	}

	openshiftClient, _, err := f.Clients()
	if err != nil {
		return err
	}
	p.TemplateClient = templateclient.New(openshiftClient.RESTClient)

	mapper, _ := f.Object()
	p.PrintObject = func(object runtime.Object) error {
		return f.PrintObject(cmd, true, mapper, object, p.Out)
	}
	return nil
}

// Validate ensures that the options to run the paarameterize command are valid
func (p *ParameterizeOptions) Validate() error {
	return nil
}

// Run runs the parameterize command
func (p *ParameterizeOptions) Run() error {
	inputObject, err := p.Builder.Do().Object()
	if err != nil {
		return err
	}

	var inputTemplate *templateapi.Template
	if list, isList := inputObject.(*kapi.List); isList {
		if len(p.Name) == 0 {
			return fmt.Errorf("specify a name with --name when providing a list of objects")
		}
		inputTemplate = &templateapi.Template{}
		inputTemplate.Name = p.TemplateName
		inputTemplate.Objects = list.Items
	}

	if tpl, isTemplate := inputObject.(*templateapi.Template); isTemplate {
		inputTemplate = tpl
	}

	if inputTemplate == nil {
		return fmt.Errorf("input must be a template or a list of objects")
	}

	aspects := []templateapi.ParameterizableAspect{}
	for _, aspect := range p.Aspects {
		aspects = append(aspects, templateapi.ParameterizableAspect(aspect))
	}

	request := &templateapi.ParameterizeTemplateRequest{
		Aspects:  aspects,
		Template: *inputTemplate,
	}

	result, err := p.TemplateClient.Template().Templates(p.Namespace).Parameterize(request)
	if err != nil {
		return err
	}

	return p.PrintObject(result)
}
