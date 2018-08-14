package createlogintemplate

import (
	"errors"
	"io"

	"github.com/spf13/cobra"

	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"

	"github.com/openshift/origin/pkg/oauthserver/server/login"
)

const CreateLoginTemplateCommand = "create-login-template"

var longDescription = templates.LongDesc(`
	Create a template for customizing the login page

	This command creates a basic template to use as a starting point for
	customizing the login page. Save the output to a file and edit the template to
	change the look and feel or add content. Be careful not to remove any parameter
	values inside curly braces.

	To use the template, set oauthConfig.templates.login in the master
	configuration to point to the template file. For example,

	    oauthConfig:
	      templates:
	        login: templates/login.html
	`)

type CreateLoginTemplateOptions struct {
	genericclioptions.IOStreams
}

func NewCreateLoginTemplateOptions(streams genericclioptions.IOStreams) *CreateLoginTemplateOptions {
	return &CreateLoginTemplateOptions{
		IOStreams: streams,
	}
}

func NewCommandCreateLoginTemplate(f kcmdutil.Factory, commandName string, fullName string, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewCreateLoginTemplateOptions(streams)
	cmd := &cobra.Command{
		Use:   commandName,
		Short: "Create a login template",
		Long:  longDescription,
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(o.Validate(args))
			kcmdutil.CheckErr(o.Run())
		},
	}

	return cmd
}

func (o CreateLoginTemplateOptions) Validate(args []string) error {
	if len(args) != 0 {
		return errors.New("no arguments are supported")
	}

	return nil
}

func (o *CreateLoginTemplateOptions) Run() error {
	_, err := io.WriteString(o.Out, login.LoginTemplateExample)
	return err
}
