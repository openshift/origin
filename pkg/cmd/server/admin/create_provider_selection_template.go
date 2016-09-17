package admin

import (
	"errors"
	"io"

	"github.com/spf13/cobra"

	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	"github.com/openshift/origin/pkg/auth/server/selectprovider"
	"github.com/openshift/origin/pkg/cmd/templates"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

const CreateProviderSelectionTemplateCommand = "create-provider-selection-template"

var providerSelectionLongDescription = templates.LongDesc(`
	Create a template for customizing the provider selection page

	This command creates a basic template to use as a starting point for
	customizing the login provider selection page. Save the output to a file and edit
	the template to change the look and feel or add content. Be careful not to remove
	any parameter values inside curly braces.

	To use the template, set oauthConfig.templates.providerSelection in the master
	configuration to point to the template file. For example,

	    oauthConfig:
	      templates:
	        providerSelection: templates/provider-selection.html
	`)

type CreateProviderSelectionTemplateOptions struct{}

func NewCommandCreateProviderSelectionTemplate(f *clientcmd.Factory, commandName string, fullName string, out io.Writer) *cobra.Command {
	options := &CreateProviderSelectionTemplateOptions{}

	cmd := &cobra.Command{
		Use:   commandName,
		Short: "Create a provider selection template",
		Long:  providerSelectionLongDescription,
		Run: func(cmd *cobra.Command, args []string) {
			if err := options.Validate(args); err != nil {
				cmdutil.CheckErr(cmdutil.UsageError(cmd, err.Error()))
			}

			_, err := io.WriteString(out, selectprovider.SelectProviderTemplateExample)
			if err != nil {
				cmdutil.CheckErr(err)
			}
		},
	}

	return cmd
}

func (o CreateProviderSelectionTemplateOptions) Validate(args []string) error {
	if len(args) != 0 {
		return errors.New("no arguments are supported")
	}

	return nil
}
