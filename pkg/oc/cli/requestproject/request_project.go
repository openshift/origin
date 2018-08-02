package requestproject

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"

	projectv1 "github.com/openshift/api/project/v1"
	projectv1client "github.com/openshift/client-go/project/clientset/versioned/typed/project/v1"
	ocproject "github.com/openshift/origin/pkg/oc/cli/project"
	cliconfig "github.com/openshift/origin/pkg/oc/lib/kubeconfig"
)

// RequestProjectOptions contains all the options for running the RequestProject cli command.
type RequestProjectOptions struct {
	ProjectName string
	DisplayName string
	Description string

	Name   string
	Server string

	SkipConfigWrite bool

	Client         projectv1client.ProjectV1Interface
	ProjectOptions *ocproject.ProjectOptions

	genericclioptions.IOStreams
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
		%[1]s new-project web-team-dev

		# Create a new project with a display name and description
		%[1]s new-project web-team-dev --display-name="Web Team Development" --description="Development project for the web team."`)
)

// RequestProject next steps.
const (
	requestProjectNewAppOutput = `
You can add applications to this project with the 'new-app' command. For example, try:

    %[1]s new-app centos/ruby-25-centos7~https://github.com/sclorg/ruby-ex.git

to build a new example application in Ruby.
`
	requestProjectSwitchProjectOutput = `Project %[2]q created on server %[3]q.

To switch to this project and start adding applications, use:

    %[1]s project %[2]s
`
)

func NewRequestProjectOptions(baseName string, streams genericclioptions.IOStreams) *RequestProjectOptions {
	return &RequestProjectOptions{
		IOStreams: streams,
		Name:      baseName,
	}
}

// NewCmdRequestProject implement the OpenShift cli RequestProject command.
func NewCmdRequestProject(baseName string, f kcmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewRequestProjectOptions(baseName, streams)
	cmd := &cobra.Command{
		Use:     "new-project NAME [--display-name=DISPLAYNAME] [--description=DESCRIPTION]",
		Short:   "Request a new project",
		Long:    requestProjectLong,
		Example: fmt.Sprintf(requestProjectExample, baseName),
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(o.Complete(f, cmd, args))
			kcmdutil.CheckErr(o.Run())
		},
	}
	cmd.Flags().StringVar(&o.DisplayName, "display-name", o.DisplayName, "Project display name")
	cmd.Flags().StringVar(&o.Description, "description", o.Description, "Project description")
	cmd.Flags().BoolVar(&o.SkipConfigWrite, "skip-config-write", o.SkipConfigWrite, "If true, the project will not be set as a cluster entry in kubeconfig after being created")

	return cmd
}

// Complete completes all the required options.
func (o *RequestProjectOptions) Complete(f kcmdutil.Factory, cmd *cobra.Command, args []string) error {
	if len(args) != 1 {
		return errors.New("must have exactly one argument")
	}

	o.ProjectName = args[0]

	if !o.SkipConfigWrite {
		o.ProjectOptions = ocproject.NewProjectOptions(o.IOStreams)
		o.ProjectOptions.PathOptions = cliconfig.NewPathOptions(cmd)
		if err := o.ProjectOptions.Complete(f, []string{""}); err != nil {
			return err
		}
	} else {
		clientConfig, err := f.ToRESTConfig()
		if err != nil {
			return err
		}
		o.Server = clientConfig.Host
	}

	clientConfig, err := f.ToRESTConfig()
	if err != nil {
		return err
	}
	o.Client, err = projectv1client.NewForConfig(clientConfig)
	if err != nil {
		return err
	}

	return nil
}

// Run implements all the necessary functionality for RequestProject.
func (o *RequestProjectOptions) Run() error {
	if err := o.Client.RESTClient().Get().Resource("projectrequests").Do().Into(&metav1.Status{}); err != nil {
		return err
	}

	projectRequest := &projectv1.ProjectRequest{}
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
		o.ProjectOptions.IOStreams = o.IOStreams
		if err := o.ProjectOptions.Run(); err != nil {
			return err
		}

		fmt.Fprintf(o.Out, requestProjectNewAppOutput, o.Name)
	} else {
		fmt.Fprintf(o.Out, requestProjectSwitchProjectOutput, o.Name, o.ProjectName, o.Server)
	}

	return nil
}
