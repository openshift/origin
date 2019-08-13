package createerrortemplate

import (
	"errors"
	"io"

	"github.com/spf13/cobra"

	"k8s.io/cli-runtime/pkg/genericclioptions"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/util/templates"
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
	_, err := io.WriteString(o.Out, ErrorPageTemplateExample)
	return err
}

// ErrorPageTemplateExample is a basic template for customizing the error page.
const ErrorPageTemplateExample = `<!DOCTYPE html>
<!--

This template can be modified and used to customize the error page. To replace
the error page, set master configuration option oauthConfig.templates.error to
the path of the template file.

oauthConfig:
  templates:
    error: templates/error-template.html

The Error field contains an error message, which is human readable, and subject to change.
Default error messages are intentionally generic to avoid leaking information about authentication errors.

The ErrorCode field contains a programmatic error code, which may be (but is not limited to):
- mapping_claim_error
- mapping_lookup_error
- authentication_error
- grant_error
-->
<html>
  <head>
    <title>Error</title>
    <style type="text/css">
      body {
        font-family: "Open Sans", Helvetica, Arial, sans-serif;
        font-size: 14px;
        margin: 15px;
      }
    </style>
  </head>
  <body>

    <div>
		<!-- example of handling a particular error code in a special way -->
		{{ if eq .ErrorCode "mapping_claim_error" }}
			Could not create your user. Contact your administrator to resolve this issue.
		{{ else }}
			{{ .Error }}
		{{ end }}
		</div>

  </body>
</html>
`
