package createlogintemplate

import (
	"errors"
	"io"

	"github.com/spf13/cobra"

	"k8s.io/cli-runtime/pkg/genericclioptions"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/util/templates"
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
	_, err := io.WriteString(o.Out, LoginTemplateExample)
	return err
}

// LoginTemplateExample is a basic template for customizing the login page.
const LoginTemplateExample = `<!DOCTYPE html>
<!--

This template can be modified and used to customize the login page. To replace
the login page, set master configuration option oauthConfig.templates.login to
the path of the template file. Don't remove parameters in curly braces below.

oauthConfig:
  templates:
    login: templates/login-template.html

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

      input {
        margin-bottom: 10px;
        width: 300px;
      }

      .error {
        color: red;
        margin-bottom: 10px;
      }
    </style>
  </head>
  <body>

    {{ if .Error }}
      <div class="error">{{ .Error }}</div>
      <!-- Error code: {{ .ErrorCode }} -->
    {{ end }}

    <!-- Identity provider name: {{ .ProviderName }} -->
    <form action="{{ .Action }}" method="POST">
      <input type="hidden" name="{{ .Names.Then }}" value="{{ .Values.Then }}">
      <input type="hidden" name="{{ .Names.CSRF }}" value="{{ .Values.CSRF }}">

      <div>
        <label for="inputUsername">Username</label>
      </div>
      <div>
        <input type="text" id="inputUsername" autofocus="autofocus" type="text" name="{{ .Names.Username }}" value="{{ .Values.Username }}">
      </div>

      <div>
        <label for="inputPassword">Password</label>
      </div>
      <div>
        <input type="password" id="inputPassword" type="password" name="{{ .Names.Password }}" value="">
      </div>

      <button type="submit">Log In</button>

    </form>

  </body>
</html>
`
