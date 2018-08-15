package createerrortemplate

import (
	"errors"
	"io"

	"github.com/spf13/cobra"

	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"

	"github.com/openshift/origin/pkg/oauthserver/server/errorpage"
)

const CreateErrorTemplateCommand = "create-error-template"

var errorLongDescription = templates.LongDesc(`
		Create a template for customizing the error page

		This command creates a basic template to use as a starting point for
		customizing the authentication error page. Save the output to a file and edit
		the template to change the look and feel or add content.

		To use the template, set oauthConfig.templates.error in the master
		configuration to point to the template file. For example,

		    oauthConfig:
		      templates:
		        error: templates/error.html
		`)

type CreateErrorTemplateOptions struct {
	genericclioptions.IOStreams
}

func NewCreateErrorTemplateOptions(streams genericclioptions.IOStreams) *CreateErrorTemplateOptions {
	return &CreateErrorTemplateOptions{
		IOStreams: streams,
	}
}

func NewCommandCreateErrorTemplate(f kcmdutil.Factory, commandName string, fullName string, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewCreateErrorTemplateOptions(streams)
	cmd := &cobra.Command{
		Use:   commandName,
		Short: "Create an error page template",
		Long:  errorLongDescription,
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(o.Validate(args))
			kcmdutil.CheckErr(o.Run())
		},
	}

	return cmd
}

func (o *CreateErrorTemplateOptions) Validate(args []string) error {
	if len(args) != 0 {
		return errors.New("no arguments are supported")
	}

	return nil
}

func (o *CreateErrorTemplateOptions) Run() error {
	_, err := io.WriteString(o.Out, errorpage.ErrorPageTemplateExample)
	return err
}
