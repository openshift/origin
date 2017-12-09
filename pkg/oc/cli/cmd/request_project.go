package cmd

import (
	"errors"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	cliconfig "github.com/openshift/origin/pkg/oc/cli/config"
	"github.com/openshift/origin/pkg/oc/cli/util/clientcmd"
	projectapi "github.com/openshift/origin/pkg/project/apis/project"
	projectclient "github.com/openshift/origin/pkg/project/generated/internalclientset/typed/project/internalversion"
)

// RequestProjectRecommendedCommandName is the recommended command name.
const RequestProjectRecommendedCommandName = "new-project"

// NewProjectOptions contains all the options for running the RequestProject cli command.
type NewProjectOptions struct {
	ProjectName string
	DisplayName string
	Description string

	Name   string
	Server string

	SkipConfigWrite bool

	Client projectclient.ProjectInterface

	ProjectOptions *ProjectOptions
	Out            io.Writer
}

// RequestProject command description.
var (
	requestProjectLong = templates.LongDesc(`
		Create a new project for yourself

		If your administrator allows self-service, this command will create a new project for you and assign you
		as the project admin.

		After your project is created it will become the default project in your config.`)

	requestProjectExample = templates.Examples(`
		# Create a new project with minimal information
	  %[1]s %[2]s web-team-dev

	  # Create a new project with a display name and description
	  %[1]s %[2]s web-team-dev --display-name="Web Team Development" --description="Development project for the web team."`)
)

// RequestProject next steps.
const (
	requestProjectNewAppOutput = `
You can add applications to this project with the 'new-app' command. For example, try:

    %[1]s new-app centos/ruby-22-centos7~https://github.com/openshift/ruby-ex.git

to build a new example application in Ruby.
`
	requestProjectSwitchProjectOutput = `Project %[2]q created on server %[3]q.

To switch to this project and start adding applications, use:

    %[1]s project %[2]s
`
)

// NewCmdRequestProject implement the OpenShift cli RequestProject command.
func NewCmdRequestProject(name, baseName string, f *clientcmd.Factory, out, errout io.Writer) *cobra.Command {
	o := &NewProjectOptions{}
	o.Out = out
	o.Name = baseName

	cmd := &cobra.Command{
		Use:     fmt.Sprintf("%s NAME [--display-name=DISPLAYNAME] [--description=DESCRIPTION]", name),
		Short:   "Request a new project",
		Long:    requestProjectLong,
		Example: fmt.Sprintf(requestProjectExample, baseName, name),
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(o.Complete(f, cmd, args))
			projectClient, err := f.OpenshiftInternalProjectClient()
			kcmdutil.CheckErr(err)
			o.Client = projectClient.Project()
			kcmdutil.CheckErr(o.Run())
		},
	}

	cmd.Flags().StringVar(&o.DisplayName, "display-name", "", "Project display name")
	cmd.Flags().StringVar(&o.Description, "description", "", "Project description")
	cmd.Flags().BoolVar(&o.SkipConfigWrite, "skip-config-write", false, "If true, the project will not be set as a cluster entry in kubeconfig after being created")

	return cmd
}

// Complete completes all the required options.
func (o *NewProjectOptions) Complete(f *clientcmd.Factory, cmd *cobra.Command, args []string) error {
	if len(args) != 1 {
		cmd.Help()
		return errors.New("must have exactly one argument")
	}

	o.ProjectName = args[0]

	if !o.SkipConfigWrite {
		o.ProjectOptions = &ProjectOptions{}
		o.ProjectOptions.PathOptions = cliconfig.NewPathOptions(cmd)
		if err := o.ProjectOptions.Complete(f, []string{""}, o.Out); err != nil {
			return err
		}
	} else {
		clientConfig, err := f.OpenShiftClientConfig().ClientConfig()
		if err != nil {
			return err
		}
		o.Server = clientConfig.Host
	}

	return nil
}

// Run implements all the necessary functionality for RequestProject.
func (o *NewProjectOptions) Run() error {
	if err := o.Client.RESTClient().Get().Resource("projectrequests").Do().Into(&metav1.Status{}); err != nil {
		return err
	}

	projectRequest := &projectapi.ProjectRequest{}
	projectRequest.Name = o.ProjectName
	projectRequest.DisplayName = o.DisplayName
	projectRequest.Description = o.Description
	projectRequest.Annotations = make(map[string]string)

	project, err := o.Client.ProjectRequests().Create(projectRequest)
	if err != nil {
		return err
	}

	if o.ProjectOptions != nil {
		o.ProjectOptions.ProjectName = project.Name
		o.ProjectOptions.ProjectOnly = true
		o.ProjectOptions.SkipAccessValidation = true

		if err := o.ProjectOptions.RunProject(); err != nil {
			return err
		}

		fmt.Fprintf(o.Out, requestProjectNewAppOutput, o.Name)
	} else {
		fmt.Fprintf(o.Out, requestProjectSwitchProjectOutput, o.Name, o.ProjectName, o.Server)
	}

	return nil
}
