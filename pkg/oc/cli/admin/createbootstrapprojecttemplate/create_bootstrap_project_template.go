package createbootstrapprojecttemplate

import (
	"errors"

	"github.com/spf13/cobra"

	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"
	"k8s.io/kubernetes/pkg/kubectl/scheme"
	"k8s.io/kubernetes/pkg/printers"

	"github.com/openshift/origin/pkg/project/apiserver/registry/projectrequest/delegated"
)

const CreateBootstrapProjectTemplateCommand = "create-bootstrap-project-template"

type CreateBootstrapProjectTemplateOptions struct {
	PrintFlags *genericclioptions.PrintFlags

	Name string
	Args []string

	Printer printers.ResourcePrinter

	genericclioptions.IOStreams
}

func NewCreateBootstrapProjectTemplateOptions(streams genericclioptions.IOStreams) *CreateBootstrapProjectTemplateOptions {
	return &CreateBootstrapProjectTemplateOptions{
		PrintFlags: genericclioptions.NewPrintFlags("created").WithTypeSetter(scheme.Scheme).WithDefaultOutput("json"),
		Name:       delegated.DefaultTemplateName,
		IOStreams:  streams,
	}
}

func NewCommandCreateBootstrapProjectTemplate(f kcmdutil.Factory, commandName string, fullName string, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewCreateBootstrapProjectTemplateOptions(streams)
	cmd := &cobra.Command{
		Use:   commandName,
		Short: "Create a bootstrap project template",
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(o.Complete(args))
			kcmdutil.CheckErr(o.Validate())
			kcmdutil.CheckErr(o.Run())
		},
	}

	cmd.Flags().StringVar(&o.Name, "name", o.Name, "The name of the template to output.")
	o.PrintFlags.AddFlags(cmd)

	return cmd
}

func (o *CreateBootstrapProjectTemplateOptions) Complete(args []string) error {
	o.Args = args
	var err error
	o.Printer, err = o.PrintFlags.ToPrinter()
	if err != nil {
		return err
	}
	return nil
}

func (o *CreateBootstrapProjectTemplateOptions) Validate() error {
	if len(o.Args) != 0 {
		return errors.New("no arguments are supported")
	}
	if len(o.Name) == 0 {
		return errors.New("--name must be provided")
	}

	return nil
}

func (o *CreateBootstrapProjectTemplateOptions) Run() error {
	template := delegated.DefaultTemplate()
	template.Name = o.Name

	return o.Printer.PrintObj(template, o.Out)
}
