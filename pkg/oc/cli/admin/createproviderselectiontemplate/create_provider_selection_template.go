package createproviderselectiontemplate

import (
	"errors"
	"io"

	"github.com/spf13/cobra"

	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"

	"github.com/openshift/origin/pkg/oauthserver/server/selectprovider"
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

type CreateProviderSelectionTemplateOptions struct {
	genericclioptions.IOStreams
}

func NewCreateProviderSelectionTemplateOptions(streams genericclioptions.IOStreams) *CreateProviderSelectionTemplateOptions {
	return &CreateProviderSelectionTemplateOptions{
		IOStreams: streams,
	}
}

func NewCommandCreateProviderSelectionTemplate(f kcmdutil.Factory, commandName string, fullName string, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewCreateProviderSelectionTemplateOptions(streams)
	cmd := &cobra.Command{
		Use:   commandName,
		Short: "Create a provider selection template",
		Long:  providerSelectionLongDescription,
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(o.Validate(args))
			kcmdutil.CheckErr(o.Run())
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

func (o *CreateProviderSelectionTemplateOptions) Run() error {
	_, err := io.WriteString(o.Out, selectprovider.SelectProviderTemplateExample)
	return err
}
