package createproviderselectiontemplate

import (
	"errors"
	"io"

	"github.com/spf13/cobra"

	"k8s.io/cli-runtime/pkg/genericclioptions"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/util/templates"
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
	_, err := io.WriteString(o.Out, SelectProviderTemplateExample)
	return err
}

// SelectProviderTemplateExample is a basic template for customizing the provider selection page.
const SelectProviderTemplateExample = `<!DOCTYPE html>
<!--

This template can be modified and used to customize the provider selection page. To replace
the provider selection page, set master configuration option oauthConfig.templates.providerSelection to
the path of the template file. Don't remove parameters in curly braces below.

oauthConfig:
  templates:
    providerSelection: templates/select-provider-template.html

The Name is unique for each provider and can be used for provider specific customizations like
the example below.  The Name matches the name of an identity provider in the master configuration.
-->
<html>
  <head>
    <title>Login</title>
    <style type="text/css">
      body {
        font-family: "Open Sans", Helvetica, Arial, sans-serif;
        font-size: 14px;
        margin: 15px;
      }
    </style>
  </head>
  <body>

    {{ range $provider := .Providers }}
      <div>
        <!-- This is an example of customizing display for a particular provider based on its Name -->
        {{ if eq $provider.Name "anypassword" }}
          <a href="{{$provider.URL}}">Log in</a> with any username and password
        {{ else }}
          <a href="{{$provider.URL}}">{{$provider.Name}}</a>
        {{ end }}
      </div>
    {{ end }}

  </body>
</html>
`
