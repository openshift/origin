package admin

import (
	"errors"
	"io"

	"github.com/spf13/cobra"

	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	"github.com/openshift/origin/pkg/oauthserver/server/login"
	"github.com/openshift/origin/pkg/oc/cli/util/clientcmd"
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

type CreateLoginTemplateOptions struct{}

func NewCommandCreateLoginTemplate(f *clientcmd.Factory, commandName string, fullName string, out io.Writer) *cobra.Command {
	options := &CreateLoginTemplateOptions{}

	cmd := &cobra.Command{
		Use:   commandName,
		Short: "Create a login template",
		Long:  longDescription,
		Run: func(cmd *cobra.Command, args []string) {
			if err := options.Validate(args); err != nil {
				cmdutil.CheckErr(cmdutil.UsageErrorf(cmd, err.Error()))
			}

			_, err := io.WriteString(out, login.LoginTemplateExample)
			if err != nil {
				cmdutil.CheckErr(err)
			}
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
