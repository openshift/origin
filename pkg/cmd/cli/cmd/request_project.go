package cmd

import (
	"errors"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"k8s.io/kubernetes/pkg/fields"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/labels"

	"github.com/openshift/origin/pkg/client"
	cliconfig "github.com/openshift/origin/pkg/cmd/cli/config"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	projectapi "github.com/openshift/origin/pkg/project/api"
)

type NewProjectOptions struct {
	ProjectName string
	DisplayName string
	Description string

	Client client.Interface

	ProjectOptions *ProjectOptions
	Out            io.Writer
}

const (
	requestProjectLong = `
Create a new project for yourself

Assuming your cluster admin has granted you permission, this command will create a new project
for you and assign you as the project admin. If your administrator has not given you permission to
create your own projects, contact your system administrator.

After your project is created it will be made your default project in your config.`

	requestProjectExample = `  // Create a new project with minimal information
  $ %[1]s web-team-dev

  // Create a new project with a display name and description
  $ %[1]s web-team-dev --display-name="Web Team Development" --description="Development project for the web team."`
)

func NewCmdRequestProject(name, fullName, ocLoginName, ocProjectName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	options := &NewProjectOptions{}
	options.Out = out

	cmd := &cobra.Command{
		Use:     fmt.Sprintf("%s NAME [--display-name=DISPLAYNAME] [--description=DESCRIPTION]", name),
		Short:   "Request a new project",
		Long:    fmt.Sprintf(requestProjectLong, ocLoginName, ocProjectName),
		Example: fmt.Sprintf(requestProjectExample, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			if err := options.complete(cmd, f); err != nil {
				kcmdutil.CheckErr(err)
			}

			var err error
			if options.Client, _, err = f.Clients(); err != nil {
				kcmdutil.CheckErr(err)
			}
			if err := options.Run(); err != nil {
				kcmdutil.CheckErr(err)
			}
		},
	}

	cmd.Flags().StringVar(&options.DisplayName, "display-name", "", "Project display name")
	cmd.Flags().StringVar(&options.Description, "description", "", "Project description")

	return cmd
}

func (o *NewProjectOptions) complete(cmd *cobra.Command, f *clientcmd.Factory) error {
	args := cmd.Flags().Args()
	if len(args) != 1 {
		cmd.Help()
		return errors.New("must have exactly one argument")
	}

	o.ProjectName = args[0]

	o.ProjectOptions = &ProjectOptions{}
	o.ProjectOptions.PathOptions = cliconfig.NewPathOptions(cmd)
	if err := o.ProjectOptions.Complete(f, []string{""}, o.Out); err != nil {
		return err
	}

	return nil
}

func (o *NewProjectOptions) Run() error {
	// TODO eliminate this when we get better forbidden messages
	_, err := o.Client.ProjectRequests().List(labels.Everything(), fields.Everything())
	if err != nil {
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

		if err := o.ProjectOptions.RunProject(); err != nil {
			return err
		}
	}

	return nil
}
